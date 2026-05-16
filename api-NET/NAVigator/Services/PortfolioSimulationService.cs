using System.Globalization;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Models.Dto.Portfolio;
using NAVigator.Repositories;

namespace NAVigator.Services;

public partial class PortfolioSimulationService(
    IFundRepository fundRepo,
    INavService navService,
    ICorporateActionService corporateActionService,
    ILogger<PortfolioSimulationService> logger) : IPortfolioSimulationService
{
    private static readonly CultureInfo Inv = CultureInfo.InvariantCulture;
    private const string DateFmt = "dd-MM-yyyy";

    public async Task<SimulationResponse> SimulateAsync(SimulationRequest request, CancellationToken ct = default)
    {
        var requestedStart = ParseDate(request.StartDate);
        var requestedEnd   = ParseDate(request.EndDate);

        // Clamp to the youngest fund's inception date so no fund is silently
        // skipped and allocation percentages remain valid for the whole period.
        var (effectiveStart, effectiveEnd, wasClamped, dateRangeNote) =
            await ComputeGlobalEffectiveDateRangeAsync(
                [request.Basket], requestedStart, requestedEnd, ct);

        var result = await RunSimulationCoreAsync(
            request.Basket, request.SipAmount, request.SipDay, request.LumpSumAmount,
            effectiveStart, effectiveEnd, ct);

        // Overwrite effective dates with the clamped values and surface the note.
        result.StartDate           = requestedStart.ToString(DateFmt, Inv);
        result.EndDate             = requestedEnd.ToString(DateFmt, Inv);
        result.EffectiveStartDate  = effectiveStart.ToString(DateFmt, Inv);
        result.EffectiveEndDate    = effectiveEnd.ToString(DateFmt, Inv);
        result.WasDateRangeClamped = wasClamped;
        result.DateRangeNote       = dateRangeNote;

        return result;
    }

    internal async Task<SimulationResponse> RunSimulationCoreAsync(
        BasketDefinition basket, decimal sipAmount, int sipDay, decimal lumpSum,
        DateOnly startDate, DateOnly endDate, CancellationToken ct)
    {
        // ── 1. Load NAV histories and fund metadata ──────────────────────────────
        logger.LogInformation(
            "Running simulation for basket '{Basket}' with {FundCount} funds, SIP ₹{Sip}, Lump ₹{Lump}, {Start} to {End}",
            basket.Name, basket.Funds.Count, sipAmount, lumpSum, startDate, endDate);

        var fundData = new Dictionary<int, FundNavData>();
        var navHistories = await navService.EnsureNavHistoriesAsync(
            basket.Funds.Select(f => f.SchemeCode), startDate, endDate, ct);
        foreach (var alloc in basket.Funds)
        {
            var fund = await fundRepo.GetBySchemeCodeAsync(alloc.SchemeCode)
                       ?? throw new ArgumentException($"Fund {alloc.SchemeCode} not found.");
            var sorted = navHistories.GetValueOrDefault(alloc.SchemeCode) ?? [];
            var actions = await corporateActionService.GetActionsAsync(alloc.SchemeCode);
            fundData[alloc.SchemeCode] = new FundNavData(fund, sorted, actions, alloc.AllocationPercent);
        }

        // ── 2. Initialise per-fund state ─────────────────────────────────────────
        // LastCaAppliedUpTo = startDate - 1 prevents any CA that predates the
        // simulation from being applied to units bought on the first event.
        var state = new Dictionary<int, FundState>();
        foreach (var kvp in fundData)
            state[kvp.Key] = new FundState { LastCaAppliedUpTo = startDate.AddDays(-1) };

        decimal totalInvested = 0m;
        int     sipInstallments = 0;
        var     cashFlows  = new List<(DateTime Date, double Amount)>();
        var     timeSeries = new List<PortfolioSnapshot>();

        // ── 3. Build one unified, chronologically sorted event list ──────────────
        // Same-date ordering: investments (0) → rebalances (1) → snapshots (2).
        // This guarantees that a rebalance sees only money actually invested so
        // far, and that snapshots reflect the state after all transactions that day.
        var sipDates = sipAmount > 0
            ? GenerateSipDates(startDate, endDate, sipDay)
            : new List<DateOnly>();

        var rebalDates = basket.RebalanceFrequency.Equals("None", StringComparison.OrdinalIgnoreCase)
            ? new List<DateOnly>()
            : GenerateRebalanceDates(basket.RebalanceFrequency, startDate, endDate);

        var snapDates = BuildSnapDates(startDate, endDate);

        var allEvents = new List<(DateOnly Date, int Priority, string Type)>();
        if (lumpSum > 0)
            allEvents.Add((startDate, 0, "LUMP"));
        foreach (var d in sipDates)
            allEvents.Add((d, 0, "SIP"));
        foreach (var d in rebalDates)
            allEvents.Add((d, 1, "REBAL"));
        foreach (var d in snapDates)
            allEvents.Add((d, 2, "SNAP"));

        allEvents.Sort((a, b) => a.Date != b.Date
            ? a.Date.CompareTo(b.Date)
            : a.Priority.CompareTo(b.Priority));

        // ── 4. Walk events in order ──────────────────────────────────────────────
        foreach (var (date, _, type) in allEvents)
        {
            switch (type)
            {
                // ── Investment (lump sum or SIP) ─────────────────────────────────
                case "LUMP":
                case "SIP":
                {
                    var amount      = type == "LUMP" ? lumpSum : sipAmount;
                    bool processed  = false;

                    foreach (var kvp in fundData)
                    {
                        // Use FindNavOnOrAfter so the purchase executes at the next
                        // available NAV (mimics real-world T+1 cut-off behaviour).
                        var nav = FindNavOnOrAfter(kvp.Value.Navs, date);
                        if (nav is null) continue;

                        // Apply any splits / bonuses since the last event BEFORE buying.
                        // AMFI NAVs are raw and unadjusted, so this keeps unit counts
                        // in sync with NAV drops caused by corporate actions.
                        ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions,
                            state[kvp.Key].LastCaAppliedUpTo, nav.Date);
                        state[kvp.Key].LastCaAppliedUpTo = nav.Date;

                        var alloc = amount * kvp.Value.AllocationPct / 100m;
                        state[kvp.Key].Units    += alloc / nav.Nav;
                        state[kvp.Key].Invested += alloc;
                        processed = true;
                    }

                    if (processed)
                    {
                        totalInvested += amount;
                        cashFlows.Add((date.ToDateTime(TimeOnly.MinValue), -(double)amount));
                        if (type == "SIP") sipInstallments++;
                    }
                    break;
                }

                // ── Rebalance ────────────────────────────────────────────────────
                case "REBAL":
                {
                    // Step A: apply CAs and compute current value at the rebal date.
                    decimal totalValue = 0;
                    var navMap = new Dictionary<int, NavHistory>();

                    foreach (var kvp in fundData)
                    {
                        var nav = FindNavOnOrAfter(kvp.Value.Navs, date);
                        if (nav is null) continue;

                        ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions,
                            state[kvp.Key].LastCaAppliedUpTo, nav.Date);
                        state[kvp.Key].LastCaAppliedUpTo = nav.Date;

                        navMap[kvp.Key] = nav;
                        totalValue += state[kvp.Key].Units * nav.Nav;
                    }

                    // Step B: redistribute units to hit target allocation percentages.
                    // Total portfolio value is preserved; only the per-fund split changes.
                    if (totalValue > 0)
                    {
                        foreach (var kvp in fundData)
                        {
                            if (!navMap.TryGetValue(kvp.Key, out var nav) || nav.Nav <= 0) continue;
                            var targetValue = totalValue * kvp.Value.AllocationPct / 100m;
                            state[kvp.Key].Units    = targetValue / nav.Nav;
                            state[kvp.Key].Invested = totalInvested * kvp.Value.AllocationPct / 100m;
                        }
                    }
                    break;
                }

                // ── Snapshot (time-series data point) ───────────────────────────
                case "SNAP":
                {
                    decimal portfolioValue = 0;
                    var fundSnapshots = new List<FundSnapshot>();

                    foreach (var kvp in fundData)
                    {
                        // FindNavOnOrBefore for valuation — we never look ahead.
                        var nav = FindNavOnOrBefore(kvp.Value.Navs, date);
                        if (nav is null) continue;

                        // CAs must still be applied here. A split causes the raw NAV
                        // to drop sharply; without updating units the portfolio would
                        // show a fake loss. Advancing LastCaAppliedUpTo means the next
                        // event's CA window starts here and the split is not re-applied.
                        ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions,
                            state[kvp.Key].LastCaAppliedUpTo, nav.Date);
                        state[kvp.Key].LastCaAppliedUpTo = nav.Date;

                        var val = state[kvp.Key].Units * nav.Nav;
                        portfolioValue += val;
                        fundSnapshots.Add(new FundSnapshot
                        {
                            SchemeCode = kvp.Key,
                            Value      = Math.Round(val, 2),
                            Units      = Math.Round(state[kvp.Key].Units, 4)
                        });
                    }

                    timeSeries.Add(new PortfolioSnapshot
                    {
                        Date           = date.ToString(DateFmt, Inv),
                        PortfolioValue = Math.Round(portfolioValue, 2),
                        TotalInvested  = Math.Round(totalInvested, 2),
                        GainPercent    = totalInvested > 0
                            ? Math.Round((portfolioValue - totalInvested) / totalInvested * 100m, 2)
                            : 0,
                        Funds = fundSnapshots
                    });
                    break;
                }
            }
        }

        // ── 5. Final corpus & per-fund detail ────────────────────────────────────
        // Flush any remaining CAs up to endDate before computing final values.
        foreach (var kvp in fundData)
        {
            var lastNav = FindNavOnOrBefore(kvp.Value.Navs, endDate);
            if (lastNav is null) continue;
            ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions,
                state[kvp.Key].LastCaAppliedUpTo, lastNav.Date);
            state[kvp.Key].LastCaAppliedUpTo = lastNav.Date;
        }

        decimal finalCorpus = 0;
        var fundDetails = new List<FundSimulationDetail>();
        foreach (var kvp in fundData)
        {
            var lastNav = FindNavOnOrBefore(kvp.Value.Navs, endDate);
            if (lastNav is null) continue;
            var val      = state[kvp.Key].Units * lastNav.Nav;
            var invested = state[kvp.Key].Invested;
            finalCorpus += val;
            fundDetails.Add(new FundSimulationDetail
            {
                SchemeCode            = kvp.Key,
                SchemeName            = kvp.Value.Fund.SchemeName,
                AllocationPercent     = kvp.Value.AllocationPct,
                TotalUnits            = Math.Round(state[kvp.Key].Units, 4),
                TotalInvested         = Math.Round(invested, 2),
                FinalValue            = Math.Round(val, 2),
                AbsoluteReturnPercent = invested > 0
                    ? Math.Round((val - invested) / invested * 100m, 2) : 0
            });
        }

        // ── 6. Aggregate metrics ─────────────────────────────────────────────────
        decimal absReturn = totalInvested > 0
            ? (finalCorpus - totalInvested) / totalInvested * 100m : 0;

        int days = endDate.DayNumber - startDate.DayNumber;
        decimal? cagr = days > 0 && totalInvested > 0
            ? (decimal)(Math.Pow((double)(finalCorpus / totalInvested), 365.25 / days) - 1) * 100m
            : null;

        cashFlows.Add((endDate.ToDateTime(TimeOnly.MinValue), (double)finalCorpus));
        var xirr = XirrCalculator.Calculate(cashFlows);
        var (maxDrawdown, volatility) = ComputeRiskMetrics(timeSeries);

        return new SimulationResponse
        {
            BasketName            = basket.Name,
            TotalInvested         = Math.Round(totalInvested, 2),
            FinalCorpusValue      = Math.Round(finalCorpus, 2),
            AbsoluteReturnPercent = Math.Round(absReturn, 2),
            Cagr                  = cagr.HasValue ? Math.Round(cagr.Value, 2) : null,
            Xirr                  = xirr,
            MaxDrawdownPercent    = maxDrawdown,
            Volatility            = volatility,
            StartDate             = startDate.ToString(DateFmt, Inv),
            EndDate               = endDate.ToString(DateFmt, Inv),
            EffectiveStartDate    = startDate.ToString(DateFmt, Inv),
            EffectiveEndDate      = endDate.ToString(DateFmt, Inv),
            SipInstallments       = sipInstallments,
            SipAmount             = sipAmount,
            LumpSumAmount         = lumpSum,
            FundDetails           = fundDetails,
            TimeSeries            = timeSeries
        };
    }

    // ─── Helpers ─────────────────────────────────────────────────────────────────

    private static DateOnly ParseDate(string s) =>
        DateOnly.ParseExact(s, DateFmt, Inv);

    /// <summary>
    /// Finds the earliest NAV on or after <paramref name="target"/>.
    /// Falls back to the last available NAV if it is within 7 days before target
    /// (handles end-of-data edge case). Returns null if the nearest NAV is more
    /// than 7 days away in either direction.
    /// </summary>
    private static NavHistory? FindNavOnOrAfter(List<NavHistory> sorted, DateOnly target)
    {
        int lo = 0, hi = sorted.Count - 1;
        NavHistory? best = null;
        while (lo <= hi)
        {
            int mid = (lo + hi) / 2;
            if (sorted[mid].Date >= target) { best = sorted[mid]; hi = mid - 1; }
            else lo = mid + 1;
        }
        // Last-resort: accept a NAV up to 7 days before target when there is no
        // future data (e.g. fund is brand-new or end-of-range).
        if (best is null && sorted.Count > 0)
        {
            var last = sorted[^1];
            if (target.DayNumber - last.Date.DayNumber <= 7) return last;
        }
        // Reject if the found NAV is more than 7 trading days ahead (stale data).
        if (best is not null && best.Date.DayNumber - target.DayNumber > 7) return null;
        return best;
    }

    /// <summary>
    /// Finds the latest NAV on or before <paramref name="target"/>.
    /// </summary>
    private static NavHistory? FindNavOnOrBefore(List<NavHistory> sorted, DateOnly target)
    {
        int lo = 0, hi = sorted.Count - 1;
        NavHistory? best = null;
        while (lo <= hi)
        {
            int mid = (lo + hi) / 2;
            if (sorted[mid].Date <= target) { best = sorted[mid]; lo = mid + 1; }
            else hi = mid - 1;
        }
        return best;
    }

    /// <summary>
    /// Applies split / bonus corporate actions in the half-open window
    /// (from, to].  The <paramref name="from"/> value is non-nullable;
    /// initialise FundState.LastCaAppliedUpTo to startDate - 1 so the first
    /// event's window naturally covers the simulation start date.
    /// </summary>
    private static void ApplyCorporateActions(
        FundState fundState, List<CorporateActionDto> actions,
        DateOnly from, DateOnly to)
    {
        foreach (var action in actions)
        {
            if (action.ActionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase) ||
                action.ActionType.Equals("MissingData",   StringComparison.OrdinalIgnoreCase))
                continue;

            if (!DateOnly.TryParseExact(action.ActionDate, DateFmt, Inv,
                    DateTimeStyles.None, out var aDate))
                continue;

            // Strict half-open window: (from, to]
            if (aDate <= from || aDate > to) continue;

            fundState.Units *= action.AdjustmentFactor;
        }
    }

    private static List<DateOnly> GenerateSipDates(DateOnly start, DateOnly end, int sipDay)
    {
        sipDay = Math.Clamp(sipDay, 1, 28);
        var dates   = new List<DateOnly>();
        var current = new DateOnly(start.Year, start.Month,
            Math.Min(sipDay, DateTime.DaysInMonth(start.Year, start.Month)));
        if (current < start)
            current = current.AddMonths(1);
        // Re-clamp day after advancing (e.g. February when sipDay = 30)
        current = new DateOnly(current.Year, current.Month,
            Math.Min(sipDay, DateTime.DaysInMonth(current.Year, current.Month)));

        while (current <= end)
        {
            dates.Add(current);
            current = current.AddMonths(1);
            current = new DateOnly(current.Year, current.Month,
                Math.Min(sipDay, DateTime.DaysInMonth(current.Year, current.Month)));
        }
        return dates;
    }

    /// <summary>
    /// Generates month-end snapshot dates between <paramref name="start"/> and
    /// <paramref name="end"/>, always including <paramref name="end"/> itself.
    /// </summary>
    private static List<DateOnly> BuildSnapDates(DateOnly start, DateOnly end)
    {
        var dates = new List<DateOnly>();
        var month = new DateOnly(start.Year, start.Month, 1).AddMonths(1);

        while (month <= end)
        {
            var lastDay = new DateOnly(month.Year, month.Month,
                DateTime.DaysInMonth(month.Year, month.Month));
            var snap = lastDay <= end ? lastDay : end;
            dates.Add(snap);
            if (snap >= end) break;
            month = month.AddMonths(1);
        }

        // Guarantee the final date is always the end date.
        if (dates.Count == 0 || dates[^1] != end)
            dates.Add(end);

        return dates;
    }

    private static List<DateOnly> GenerateRebalanceDates(string frequency, DateOnly start, DateOnly end)
    {
        int monthsInterval = frequency.ToLowerInvariant() switch
        {
            "monthly"   => 1,
            "quarterly" => 3,
            "yearly"    => 12,
            _           => 0
        };
        if (monthsInterval == 0) return [];

        var dates   = new List<DateOnly>();
        var current = start.AddMonths(monthsInterval);
        while (current <= end)
        {
            dates.Add(current);
            current = current.AddMonths(monthsInterval);
        }
        return dates;
    }

    private static (decimal? MaxDrawdown, decimal? Volatility) ComputeRiskMetrics(
        List<PortfolioSnapshot> timeSeries)
    {
        if (timeSeries.Count < 2) return (null, null);

        decimal peak = 0, maxDd = 0;
        foreach (var snap in timeSeries)
        {
            if (snap.PortfolioValue > peak) peak = snap.PortfolioValue;
            if (peak > 0)
            {
                var dd = (peak - snap.PortfolioValue) / peak * 100m;
                if (dd > maxDd) maxDd = dd;
            }
        }

        var returns = new List<double>();
        for (int i = 1; i < timeSeries.Count; i++)
        {
            if (timeSeries[i - 1].PortfolioValue > 0)
                returns.Add((double)(timeSeries[i].PortfolioValue / timeSeries[i - 1].PortfolioValue - 1m));
        }

        decimal? vol = null;
        if (returns.Count >= 2)
        {
            double avg      = returns.Average();
            double variance = returns.Sum(r => (r - avg) * (r - avg)) / (returns.Count - 1);
            vol = Math.Round((decimal)(Math.Sqrt(variance) * Math.Sqrt(12)) * 100m, 2);
        }

        return (maxDd > 0 ? Math.Round(maxDd, 2) : 0, vol);
    }

    // ─── Internal data structures ─────────────────────────────────────────────

    internal record FundNavData(
        Fund Fund,
        List<NavHistory> Navs,
        List<CorporateActionDto> Actions,
        decimal AllocationPct);

    internal class FundState
    {
        public decimal Units   { get; set; }
        public decimal Invested { get; set; }

        /// <summary>
        /// The upper bound of the corporate-action window that has already been
        /// applied to this fund's unit count.  Initialise to startDate - 1 day
        /// so that the very first event's window is (startDate-1, navDate], which
        /// covers CAs on startDate without including any pre-simulation history.
        /// </summary>
        public DateOnly LastCaAppliedUpTo { get; set; }
    }
}
