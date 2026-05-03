using System.Globalization;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Models.Dto.Portfolio;
using NAVigator.Repositories;

namespace NAVigator.Services;

public partial class PortfolioSimulationService(
    INavHistoryRepository navRepo,
    IFundRepository fundRepo,
    INavService navService,
    ICorporateActionService corporateActionService,
    ILogger<PortfolioSimulationService> logger) : IPortfolioSimulationService
{
    private static readonly CultureInfo Inv = CultureInfo.InvariantCulture;
    private const string DateFmt = "dd-MM-yyyy";

    public async Task<SimulationResponse> SimulateAsync(SimulationRequest request, CancellationToken ct = default)
    {
        var startDate = ParseDate(request.StartDate);
        var endDate = ParseDate(request.EndDate);
        var basket = request.Basket;

        var simResult = await RunSimulationCoreAsync(
            basket, request.SipAmount, request.SipDay, request.LumpSumAmount,
            startDate, endDate, ct);

        return simResult;
    }

    internal async Task<SimulationResponse> RunSimulationCoreAsync(
        BasketDefinition basket, decimal sipAmount, int sipDay, decimal lumpSum,
        DateOnly startDate, DateOnly endDate, CancellationToken ct)
    {
        // 1. Ensure NAV data is fresh and load histories
        logger.LogInformation("Running simulation for basket '{Basket}' with {FundCount} funds, SIP ₹{Sip}, Lump ₹{Lump}, {Start} to {End}",
            basket.Name, basket.Funds.Count, sipAmount, lumpSum, startDate, endDate);
        var fundData = new Dictionary<int, FundNavData>();
        foreach (var alloc in basket.Funds)
        {
            await navService.GetNavHistoryAsync(alloc.SchemeCode, ct: ct);
            var fund = await fundRepo.GetBySchemeCodeAsync(alloc.SchemeCode)
                ?? throw new ArgumentException($"Fund {alloc.SchemeCode} not found.");
            var navs = await navRepo.GetBySchemeCodeAsync(alloc.SchemeCode, startDate, endDate);
            var sorted = navs.OrderBy(n => n.Date).ToList();
            var actions = await corporateActionService.GetActionsAsync(alloc.SchemeCode);
            fundData[alloc.SchemeCode] = new FundNavData(fund, sorted, actions, alloc.AllocationPercent);
        }

        // 2. Build per-fund state trackers
        var state = new Dictionary<int, FundState>();
        foreach (var kvp in fundData)
            state[kvp.Key] = new FundState();

        // 3. Process lump sum on start date
        decimal totalInvested = 0m;
        var cashFlows = new List<(DateTime Date, double Amount)>();

        if (lumpSum > 0)
        {
            foreach (var kvp in fundData)
            {
                var amount = lumpSum * kvp.Value.AllocationPct / 100m;
                var nav = FindNavOnOrAfter(kvp.Value.Navs, startDate);
                if (nav is null) continue;
                state[kvp.Key].Units += amount / nav.Nav;
                state[kvp.Key].Invested += amount;
            }
            totalInvested += lumpSum;
            cashFlows.Add((startDate.ToDateTime(TimeOnly.MinValue), -(double)lumpSum));
        }

        // 4. Generate SIP dates and process installments
        int sipInstallments = 0;
        if (sipAmount > 0)
        {
            var sipDates = GenerateSipDates(startDate, endDate, sipDay);
            foreach (var sipDate in sipDates)
            {
                bool anyProcessed = false;
                foreach (var kvp in fundData)
                {
                    var amount = sipAmount * kvp.Value.AllocationPct / 100m;
                    var nav = FindNavOnOrAfter(kvp.Value.Navs, sipDate);
                    if (nav is null) continue;

                    // Apply corporate action adjustments for events between last SIP and this one
                    ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions, state[kvp.Key].LastProcessedDate, nav.Date);

                    state[kvp.Key].Units += amount / nav.Nav;
                    state[kvp.Key].Invested += amount;
                    state[kvp.Key].LastProcessedDate = nav.Date;
                    anyProcessed = true;
                }
                if (anyProcessed)
                {
                    totalInvested += sipAmount;
                    sipInstallments++;
                    cashFlows.Add((sipDate.ToDateTime(TimeOnly.MinValue), -(double)sipAmount));
                }
            }
        }

        // 5. Apply rebalancing
        ApplyRebalancing(basket.RebalanceFrequency, fundData, state, startDate, endDate);

        // 6. Compute final values
        decimal finalCorpus = 0m;
        var fundDetails = new List<FundSimulationDetail>();
        foreach (var kvp in fundData)
        {
            var lastNav = kvp.Value.Navs.LastOrDefault();
            if (lastNav is null) continue;
            ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions, state[kvp.Key].LastProcessedDate, lastNav.Date);
            var value = state[kvp.Key].Units * lastNav.Nav;
            finalCorpus += value;
            var invested = state[kvp.Key].Invested;
            fundDetails.Add(new FundSimulationDetail
            {
                SchemeCode = kvp.Key,
                SchemeName = kvp.Value.Fund.SchemeName,
                AllocationPercent = kvp.Value.AllocationPct,
                TotalUnits = Math.Round(state[kvp.Key].Units, 4),
                TotalInvested = Math.Round(invested, 2),
                FinalValue = Math.Round(value, 2),
                AbsoluteReturnPercent = invested > 0 ? Math.Round((value - invested) / invested * 100m, 2) : 0
            });
        }

        // 7. Build time-series
        var timeSeries = BuildTimeSeries(fundData, state, basket, sipAmount, sipDay, lumpSum, startDate, endDate);

        // 8. Compute aggregate metrics
        decimal absReturn = totalInvested > 0 ? (finalCorpus - totalInvested) / totalInvested * 100m : 0;
        int days = endDate.DayNumber - startDate.DayNumber;
        decimal? cagr = days > 0 && totalInvested > 0
            ? (decimal)(Math.Pow((double)(finalCorpus / totalInvested), 365.25 / days) - 1) * 100m
            : null;

        // XIRR
        cashFlows.Add((endDate.ToDateTime(TimeOnly.MinValue), (double)finalCorpus));
        var xirr = XirrCalculator.Calculate(cashFlows);

        // Drawdown & volatility from time-series
        var (maxDrawdown, volatility) = ComputeRiskMetrics(timeSeries);

        return new SimulationResponse
        {
            BasketName = basket.Name,
            TotalInvested = Math.Round(totalInvested, 2),
            FinalCorpusValue = Math.Round(finalCorpus, 2),
            AbsoluteReturnPercent = Math.Round(absReturn, 2),
            Cagr = cagr.HasValue ? Math.Round(cagr.Value, 2) : null,
            Xirr = xirr,
            MaxDrawdownPercent = maxDrawdown,
            Volatility = volatility,
            StartDate = startDate.ToString(DateFmt, Inv),
            EndDate = endDate.ToString(DateFmt, Inv),
            SipInstallments = sipInstallments,
            SipAmount = sipAmount,
            LumpSumAmount = lumpSum,
            FundDetails = fundDetails,
            TimeSeries = timeSeries
        };
    }

    // ─── Helpers ───

    private static DateOnly ParseDate(string s) =>
        DateOnly.ParseExact(s, DateFmt, Inv);

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
        // Also accept up to 7 days before if nothing on/after target
        if (best is null && sorted.Count > 0)
        {
            var last = sorted[^1];
            if (target.DayNumber - last.Date.DayNumber <= 7) return last;
        }
        // Verify the found date isn't too far away (max 7 days forward)
        if (best is not null && best.Date.DayNumber - target.DayNumber > 7) return null;
        return best;
    }

    private static List<DateOnly> GenerateSipDates(DateOnly start, DateOnly end, int sipDay)
    {
        sipDay = Math.Clamp(sipDay, 1, 28);
        var dates = new List<DateOnly>();
        var current = new DateOnly(start.Year, start.Month, Math.Min(sipDay, DateTime.DaysInMonth(start.Year, start.Month)));
        if (current < start)
            current = current.AddMonths(1);
        while (current <= end)
        {
            dates.Add(current);
            current = current.AddMonths(1);
            current = new DateOnly(current.Year, current.Month, Math.Min(sipDay, DateTime.DaysInMonth(current.Year, current.Month)));
        }
        return dates;
    }

    private static void ApplyCorporateActions(FundState fundState, List<CorporateActionDto> actions, DateOnly? from, DateOnly to)
    {
        foreach (var action in actions)
        {
            if (action.ActionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase) ||
                action.ActionType.Equals("MissingData", StringComparison.OrdinalIgnoreCase))
                continue;

            if (!DateOnly.TryParseExact(action.ActionDate, DateFmt, Inv, DateTimeStyles.None, out var aDate))
                continue;

            if (from.HasValue && aDate <= from.Value) continue;
            if (aDate > to) continue;

            // Split/bonus: units multiply by adjustment factor
            fundState.Units *= action.AdjustmentFactor;
        }
    }

    private void ApplyRebalancing(string frequency, Dictionary<int, FundNavData> fundData,
        Dictionary<int, FundState> state, DateOnly start, DateOnly end)
    {
        if (frequency.Equals("None", StringComparison.OrdinalIgnoreCase)) return;

        var dates = GenerateRebalanceDates(frequency, start, end);
        foreach (var rebalDate in dates)
        {
            decimal totalValue = 0;
            var currentValues = new Dictionary<int, decimal>();
            foreach (var kvp in fundData)
            {
                var nav = FindNavOnOrAfter(kvp.Value.Navs, rebalDate);
                if (nav is null) continue;
                var val = state[kvp.Key].Units * nav.Nav;
                currentValues[kvp.Key] = val;
                totalValue += val;
            }
            if (totalValue <= 0) continue;

            foreach (var kvp in fundData)
            {
                if (!currentValues.ContainsKey(kvp.Key)) continue;
                var nav = FindNavOnOrAfter(kvp.Value.Navs, rebalDate);
                if (nav is null || nav.Nav <= 0) continue;
                var targetValue = totalValue * kvp.Value.AllocationPct / 100m;
                state[kvp.Key].Units = targetValue / nav.Nav;
            }
        }
    }

    private static List<DateOnly> GenerateRebalanceDates(string frequency, DateOnly start, DateOnly end)
    {
        int monthsInterval = frequency.ToLowerInvariant() switch
        {
            "monthly" => 1, "quarterly" => 3, "yearly" => 12, _ => 0
        };
        if (monthsInterval == 0) return [];

        var dates = new List<DateOnly>();
        var current = start.AddMonths(monthsInterval);
        while (current <= end)
        {
            dates.Add(current);
            current = current.AddMonths(monthsInterval);
        }
        return dates;
    }

    private List<PortfolioSnapshot> BuildTimeSeries(
        Dictionary<int, FundNavData> fundData, Dictionary<int, FundState> stateAtEnd,
        BasketDefinition basket, decimal sipAmount, int sipDay, decimal lumpSum,
        DateOnly start, DateOnly end)
    {
        // Re-simulate to capture state at each month-end
        var snapshots = new List<PortfolioSnapshot>();
        var simState = new Dictionary<int, FundState>();
        foreach (var kvp in fundData)
            simState[kvp.Key] = new FundState();

        decimal invested = 0;
        int sipIdx = 0;
        var sipDates = sipAmount > 0 ? GenerateSipDates(start, end, sipDay) : [];

        // Process lump sum
        if (lumpSum > 0)
        {
            foreach (var kvp in fundData)
            {
                var amount = lumpSum * kvp.Value.AllocationPct / 100m;
                var nav = FindNavOnOrAfter(kvp.Value.Navs, start);
                if (nav is null) continue;
                simState[kvp.Key].Units += amount / nav.Nav;
                simState[kvp.Key].Invested += amount;
            }
            invested = lumpSum;
        }

        // Walk month by month
        var month = new DateOnly(start.Year, start.Month, 1).AddMonths(1);
        while (month <= end.AddMonths(1))
        {
            var snapDate = month <= end ? new DateOnly(month.Year, month.Month,
                Math.Min(DateTime.DaysInMonth(month.Year, month.Month),
                month == end.AddMonths(1) ? end.Day : DateTime.DaysInMonth(month.Year, month.Month)))
                : end;
            if (snapDate > end) snapDate = end;

            // Process SIPs up to this snapshot
            while (sipIdx < sipDates.Count && sipDates[sipIdx] <= snapDate)
            {
                foreach (var kvp in fundData)
                {
                    var amount = sipAmount * kvp.Value.AllocationPct / 100m;
                    var nav = FindNavOnOrAfter(kvp.Value.Navs, sipDates[sipIdx]);
                    if (nav is null) continue;
                    ApplyCorporateActions(simState[kvp.Key], kvp.Value.Actions, simState[kvp.Key].LastProcessedDate, nav.Date);
                    simState[kvp.Key].Units += amount / nav.Nav;
                    simState[kvp.Key].Invested += amount;
                    simState[kvp.Key].LastProcessedDate = nav.Date;
                }
                invested += sipAmount;
                sipIdx++;
            }

            // Value the portfolio at snapDate
            decimal portfolioValue = 0;
            var fundSnapshots = new List<FundSnapshot>();
            foreach (var kvp in fundData)
            {
                var nav = FindNavOnOrBefore(kvp.Value.Navs, snapDate);
                if (nav is null) continue;
                ApplyCorporateActions(simState[kvp.Key], kvp.Value.Actions, simState[kvp.Key].LastProcessedDate, nav.Date);
                simState[kvp.Key].LastProcessedDate = nav.Date;
                var val = simState[kvp.Key].Units * nav.Nav;
                portfolioValue += val;
                fundSnapshots.Add(new FundSnapshot
                {
                    SchemeCode = kvp.Key,
                    Value = Math.Round(val, 2),
                    Units = Math.Round(simState[kvp.Key].Units, 4)
                });
            }

            snapshots.Add(new PortfolioSnapshot
            {
                Date = snapDate.ToString(DateFmt, Inv),
                PortfolioValue = Math.Round(portfolioValue, 2),
                TotalInvested = Math.Round(invested, 2),
                GainPercent = invested > 0 ? Math.Round((portfolioValue - invested) / invested * 100m, 2) : 0,
                Funds = fundSnapshots
            });

            if (snapDate >= end) break;
            month = month.AddMonths(1);
        }
        return snapshots;
    }

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

    private static (decimal? MaxDrawdown, decimal? Volatility) ComputeRiskMetrics(List<PortfolioSnapshot> timeSeries)
    {
        if (timeSeries.Count < 2) return (null, null);

        // Max drawdown
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

        // Volatility: annualized std dev of monthly returns
        var returns = new List<double>();
        for (int i = 1; i < timeSeries.Count; i++)
        {
            if (timeSeries[i - 1].PortfolioValue > 0)
                returns.Add((double)(timeSeries[i].PortfolioValue / timeSeries[i - 1].PortfolioValue - 1m));
        }

        decimal? vol = null;
        if (returns.Count >= 2)
        {
            double avg = returns.Average();
            double variance = returns.Sum(r => (r - avg) * (r - avg)) / (returns.Count - 1);
            vol = Math.Round((decimal)(Math.Sqrt(variance) * Math.Sqrt(12)) * 100m, 2);
        }

        return (maxDd > 0 ? Math.Round(maxDd, 2) : 0, vol);
    }

    // ─── Internal data structures ───

    internal record FundNavData(Fund Fund, List<NavHistory> Navs, List<CorporateActionDto> Actions, decimal AllocationPct);

    internal class FundState
    {
        public decimal Units { get; set; }
        public decimal Invested { get; set; }
        public DateOnly? LastProcessedDate { get; set; }
    }
}
