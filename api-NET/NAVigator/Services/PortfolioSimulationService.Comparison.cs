using System.Globalization;
using NAVigator.Models.Database;
using NAVigator.Models.Dto.Portfolio;

namespace NAVigator.Services;

public partial class PortfolioSimulationService
{
    public async Task<ComparisonResponse> CompareAsync(ComparisonRequest request, CancellationToken ct = default)
    {
        var requestedStart = ParseDate(request.StartDate);
        var requestedEnd = ParseDate(request.EndDate);

        // Pre-scan ALL funds across ALL entries to find the global effective date range
        var allBaskets = request.Entries.Select(e => ResolveBasket(e)).ToList();
        var (effectiveStart, effectiveEnd, wasClamped, dateRangeNote) =
            await ComputeGlobalEffectiveDateRangeAsync(allBaskets, requestedStart, requestedEnd, ct);

        var results = new List<ComparisonEntryResult>();
        var allTimeSeries = new Dictionary<string, List<PortfolioSnapshot>>();

        foreach (var entry in request.Entries)
        {
            var basket = ResolveBasket(entry);
            var sim = await RunSimulationCoreAsync(basket, entry.SipAmount, entry.SipDay,
                entry.LumpSumAmount, effectiveStart, effectiveEnd, ct);

            results.Add(new ComparisonEntryResult
            {
                Name = entry.Name,
                TotalInvested = sim.TotalInvested,
                FinalCorpusValue = sim.FinalCorpusValue,
                AbsoluteReturnPercent = sim.AbsoluteReturnPercent,
                Cagr = sim.Cagr,
                Xirr = sim.Xirr,
                MaxDrawdownPercent = sim.MaxDrawdownPercent,
                Volatility = sim.Volatility
            });
            allTimeSeries[entry.Name] = sim.TimeSeries;
        }

        // Compute alpha vs first entry (benchmark)
        if (results.Count > 1 && results[0].Cagr.HasValue)
        {
            for (int i = 1; i < results.Count; i++)
            {
                if (results[i].Cagr.HasValue)
                    results[i].AlphaVsFirst = Math.Round(results[i].Cagr!.Value - results[0].Cagr!.Value, 2);
            }
        }

        // Build aligned time-series
        var alignedTs = BuildAlignedTimeSeries(allTimeSeries, request.NormalizeSipAmounts);

        return new ComparisonResponse
        {
            StartDate = requestedStart.ToString(DateFmt, Inv),
            EndDate = requestedEnd.ToString(DateFmt, Inv),
            EffectiveStartDate = effectiveStart.ToString(DateFmt, Inv),
            EffectiveEndDate = effectiveEnd.ToString(DateFmt, Inv),
            WasDateRangeClamped = wasClamped,
            DateRangeNote = dateRangeNote,
            IsNormalized = request.NormalizeSipAmounts,
            Results = results,
            TimeSeries = alignedTs
        };
    }

    public async Task<WhatIfResponse> WhatIfAsync(WhatIfRequest request, CancellationToken ct = default)
    {
        var requestedStart = ParseDate(request.StartDate);
        var requestedEnd = ParseDate(request.EndDate);

        // Pre-scan ALL funds across ALL scenarios to find the global effective date range
        var allBaskets = request.Scenarios.Select(s => s.Basket).ToList();
        var (effectiveStart, effectiveEnd, wasClamped, dateRangeNote) =
            await ComputeGlobalEffectiveDateRangeAsync(allBaskets, requestedStart, requestedEnd, ct);

        var scenarios = new List<WhatIfScenarioResult>();

        foreach (var scenario in request.Scenarios)
        {
            var sim = await RunSimulationCoreAsync(scenario.Basket, scenario.SipAmount,
                scenario.SipDay, scenario.LumpSumAmount, effectiveStart, effectiveEnd, ct);

            scenarios.Add(new WhatIfScenarioResult
            {
                Name = scenario.Name,
                SipAmount = scenario.SipAmount,
                LumpSumAmount = scenario.LumpSumAmount,
                TotalInvested = sim.TotalInvested,
                FinalCorpusValue = sim.FinalCorpusValue,
                AbsoluteReturnPercent = sim.AbsoluteReturnPercent,
                Cagr = sim.Cagr,
                Xirr = sim.Xirr,
                MaxDrawdownPercent = sim.MaxDrawdownPercent,
                Volatility = sim.Volatility,
                CorpusPerRupeeInvested = sim.TotalInvested > 0
                    ? Math.Round(sim.FinalCorpusValue / sim.TotalInvested, 4) : 0
            });
        }

        var insights = GenerateInsights(scenarios);

        return new WhatIfResponse
        {
            StartDate = requestedStart.ToString(DateFmt, Inv),
            EndDate = requestedEnd.ToString(DateFmt, Inv),
            EffectiveStartDate = effectiveStart.ToString(DateFmt, Inv),
            EffectiveEndDate = effectiveEnd.ToString(DateFmt, Inv),
            WasDateRangeClamped = wasClamped,
            DateRangeNote = dateRangeNote,
            Scenarios = scenarios,
            Insights = insights
        };
    }

    public async Task<RollingComparisonResponse> RollingComparisonAsync(
        RollingComparisonRequest request, CancellationToken ct = default)
    {
        var startDate = ParseDate(request.StartDate);
        var endDate = ParseDate(request.EndDate);
        var entryResults = new List<RollingComparisonEntryResult>();
        var allSeries = new Dictionary<string, List<(DateOnly Date, decimal Return)>>();

        foreach (var entry in request.Entries)
        {
            var basket = ResolveBasket(entry);
            // For rolling returns, simulate lump sum of ₹10000 at each window start
            var rollingData = await ComputeRollingReturnsForBasketAsync(
                basket, request.PeriodInYears, startDate, endDate, ct);

            var values = rollingData.Select(r => r.Return).OrderBy(v => v).ToList();
            entryResults.Add(new RollingComparisonEntryResult
            {
                Name = entry.Name,
                DataPoints = values.Count,
                MinReturn = values.Count > 0 ? values[0] : null,
                MaxReturn = values.Count > 0 ? values[^1] : null,
                AverageReturn = values.Count > 0 ? Math.Round(values.Average(), 2) : null,
                MedianReturn = values.Count > 0 ? GetMedian(values) : null
            });
            allSeries[entry.Name] = rollingData;
        }

        // Build aligned rolling time-series
        var allDates = allSeries.Values.SelectMany(s => s.Select(x => x.Date)).Distinct().OrderBy(d => d).ToList();

        // Apply interval sampling
        if (!string.IsNullOrEmpty(request.Interval) && !request.Interval.Equals("daily", StringComparison.OrdinalIgnoreCase))
        {
            allDates = SampleDates(allDates, request.Interval);
        }

        var timeSeries = new List<RollingComparisonTimePoint>();
        foreach (var date in allDates)
        {
            var point = new RollingComparisonTimePoint
            {
                Date = date.ToString(DateFmt, Inv),
                Values = []
            };
            foreach (var kvp in allSeries)
            {
                var match = kvp.Value.FirstOrDefault(x => x.Date == date);
                point.Values.Add(new RollingComparisonSeriesValue
                {
                    Name = kvp.Key,
                    ReturnPercent = match.Date == date ? match.Return : null
                });
            }
            timeSeries.Add(point);
        }

        return new RollingComparisonResponse
        {
            StartDate = startDate.ToString(DateFmt, Inv),
            EndDate = endDate.ToString(DateFmt, Inv),
            PeriodInYears = request.PeriodInYears,
            Results = entryResults,
            TimeSeries = timeSeries
        };
    }

    // ─── Comparison helpers ───

    /// <summary>
    /// Scans ALL funds across ALL baskets to determine the tightest overlapping date range.
    /// The youngest fund (latest NavStartDate) dictates the effective start;
    /// the fund with the least recent data (earliest NavEndDate) dictates the effective end.
    /// </summary>
    private async Task<(DateOnly EffectiveStart, DateOnly EffectiveEnd, bool WasClamped, string? Note)>
        ComputeGlobalEffectiveDateRangeAsync(
            List<BasketDefinition> baskets, DateOnly requestedStart, DateOnly requestedEnd, CancellationToken ct)
    {
        var effectiveStart = requestedStart;
        var effectiveEnd = requestedEnd;
        string? youngestFundName = null;
        string? constrainingEndFundName = null;

        // Collect all unique scheme codes across all baskets
        var uniqueSchemeCodes = baskets
            .SelectMany(b => b.Funds.Select(f => f.SchemeCode))
            .Distinct()
            .ToList();

        await navService.EnsureNavHistoriesAsync(uniqueSchemeCodes, ct: ct);
        foreach (var schemeCode in uniqueSchemeCodes)
        {
            var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
            if (fund is null) continue;

            var navStartDate = fund.NavSyncTracker?.NavStartDate;
            var navEndDate = fund.NavSyncTracker?.NavEndDate;

            if (navStartDate.HasValue && navStartDate.Value > effectiveStart)
            {
                effectiveStart = navStartDate.Value;
                youngestFundName = fund.SchemeName;
            }
            if (navEndDate.HasValue && navEndDate.Value < effectiveEnd)
            {
                effectiveEnd = navEndDate.Value;
                constrainingEndFundName = fund.SchemeName;
            }
        }

        if (effectiveStart > effectiveEnd)
            throw new ArgumentException(
                $"No overlapping date range across all baskets: youngest fund '{youngestFundName}' " +
                $"(starts {effectiveStart.ToString(DateFmt, Inv)}) has no overlap with available end date.");

        bool wasClamped = effectiveStart != requestedStart || effectiveEnd != requestedEnd;
        string? note = null;
        if (wasClamped)
        {
            var parts = new List<string>();
            if (effectiveStart != requestedStart)
                parts.Add($"Start date adjusted to {effectiveStart.ToString(DateFmt, Inv)} — youngest fund is '{youngestFundName}'");
            if (effectiveEnd != requestedEnd)
                parts.Add($"End date adjusted to {effectiveEnd.ToString(DateFmt, Inv)} — '{constrainingEndFundName}' has data only up to this date");
            note = string.Join(". ", parts) + ".";
            logger.LogInformation("Global date range clamped: {Note}", note);
        }

        return (effectiveStart, effectiveEnd, wasClamped, note);
    }

    private static BasketDefinition ResolveBasket(ComparisonEntry entry)
    {
        if (entry.Basket is not null) return entry.Basket;
        if (entry.SingleFundSchemeCode.HasValue)
        {
            return new BasketDefinition
            {
                Name = entry.Name,
                Funds = [new BasketFundAllocation
                {
                    SchemeCode = entry.SingleFundSchemeCode.Value,
                    AllocationPercent = 100
                }]
            };
        }
        throw new ArgumentException($"Entry '{entry.Name}' must have either Basket or SingleFundSchemeCode.");
    }

    private static List<ComparisonTimePoint> BuildAlignedTimeSeries(
        Dictionary<string, List<PortfolioSnapshot>> allSeries, bool normalize)
    {
        var allDates = allSeries.Values
            .SelectMany(ts => ts.Select(s => s.Date))
            .Distinct()
            .OrderBy(d => d)
            .ToList();

        var result = new List<ComparisonTimePoint>();
        foreach (var date in allDates)
        {
            var point = new ComparisonTimePoint { Date = date, Values = [] };
            foreach (var kvp in allSeries)
            {
                var snap = kvp.Value.FirstOrDefault(s => s.Date == date);
                point.Values.Add(new ComparisonSeriesValue
                {
                    Name = kvp.Key,
                    PortfolioValue = snap?.PortfolioValue ?? 0,
                    NormalizedValue = snap is not null && snap.TotalInvested > 0
                        ? Math.Round(snap.PortfolioValue / snap.TotalInvested, 4) : 0
                });
            }
            result.Add(point);
        }
        return result;
    }

    private static WhatIfInsights GenerateInsights(List<WhatIfScenarioResult> scenarios)
    {
        if (scenarios.Count == 0) return new WhatIfInsights();

        var best = scenarios.OrderByDescending(s => s.FinalCorpusValue).First();
        var mostEfficient = scenarios.OrderByDescending(s => s.CorpusPerRupeeInvested).First();
        var leastDrawdown = scenarios.Where(s => s.MaxDrawdownPercent.HasValue)
            .OrderBy(s => s.MaxDrawdownPercent!.Value).FirstOrDefault();

        // Check if any lower-capital scenario outperformed a higher-capital one
        bool allocationBeat = false;
        string? details = null;
        for (int i = 0; i < scenarios.Count; i++)
        {
            for (int j = 0; j < scenarios.Count; j++)
            {
                if (i == j) continue;
                if (scenarios[i].TotalInvested < scenarios[j].TotalInvested &&
                    scenarios[i].FinalCorpusValue > scenarios[j].FinalCorpusValue)
                {
                    allocationBeat = true;
                    details = $"'{scenarios[i].Name}' (invested ₹{scenarios[i].TotalInvested:N0}) outperformed " +
                              $"'{scenarios[j].Name}' (invested ₹{scenarios[j].TotalInvested:N0}) by " +
                              $"₹{scenarios[i].FinalCorpusValue - scenarios[j].FinalCorpusValue:N0} in final corpus.";
                    break;
                }
            }
            if (allocationBeat) break;
        }

        return new WhatIfInsights
        {
            BestAbsoluteReturner = best.Name,
            BestCapitalEfficiency = mostEfficient.Name,
            LowestDrawdown = leastDrawdown?.Name,
            AllocationBeatCapital = allocationBeat,
            AllocationBeatCapitalDetails = details
        };
    }

    private async Task<List<(DateOnly Date, decimal Return)>> ComputeRollingReturnsForBasketAsync(
        BasketDefinition basket, int periodYears, DateOnly start, DateOnly end, CancellationToken ct)
    {
        // Load NAV data for all funds
        var fundData = new Dictionary<int, List<NavHistory>>();
        var navHistories = await navService.EnsureNavHistoriesAsync(
            basket.Funds.Select(f => f.SchemeCode), start.AddYears(-periodYears), end, ct);
        foreach (var alloc in basket.Funds)
        {
            fundData[alloc.Key()] = navHistories.GetValueOrDefault(alloc.SchemeCode) ?? [];
        }

        // Get union of all dates in the evaluation window
        var evalDates = fundData.Values
            .SelectMany(n => n.Where(x => x.Date >= start && x.Date <= end).Select(x => x.Date))
            .Distinct().OrderBy(d => d).ToList();

        var results = new List<(DateOnly Date, decimal Return)>();
        foreach (var evalDate in evalDates)
        {
            var lookbackDate = evalDate.AddYears(-periodYears);

            // Compute weighted basket NAV at both dates
            decimal startVal = 0, endVal = 0;
            bool valid = true;
            foreach (var alloc in basket.Funds)
            {
                var navs = fundData[alloc.SchemeCode];
                var startNav = FindClosestNavValue(navs, lookbackDate, 7);
                var endNav = FindClosestNavValue(navs, evalDate, 7);
                if (startNav is null || endNav is null || startNav.Value.Nav <= 0)
                { valid = false; break; }

                var weight = alloc.AllocationPercent / 100m;
                startVal += startNav.Value.Nav * weight;
                endVal += endNav.Value.Nav * weight;
            }
            if (!valid || startVal <= 0) continue;

            int days = evalDate.DayNumber - lookbackDate.DayNumber;
            if (days <= 0) continue;
            var cagr = (decimal)(Math.Pow((double)(endVal / startVal), 365.25 / days) - 1) * 100m;
            results.Add((evalDate, Math.Round(cagr, 2)));
        }
        return results;
    }

    private static (DateOnly Date, decimal Nav)? FindClosestNavValue(List<NavHistory> sorted, DateOnly target, int tolerance)
    {
        var nav = FindNavOnOrBefore(sorted, target);
        if (nav is not null && target.DayNumber - nav.Date.DayNumber <= tolerance)
            return (nav.Date, nav.Nav);
        var navAfter = FindNavOnOrAfter(sorted, target);
        if (navAfter is not null && navAfter.Date.DayNumber - target.DayNumber <= tolerance)
            return (navAfter.Date, navAfter.Nav);
        return null;
    }

    private static decimal GetMedian(List<decimal> sorted) =>
        sorted.Count % 2 == 0
            ? Math.Round((sorted[sorted.Count / 2 - 1] + sorted[sorted.Count / 2]) / 2m, 2)
            : sorted[sorted.Count / 2];

    private static List<DateOnly> SampleDates(List<DateOnly> dates, string interval) =>
        interval.ToLowerInvariant() switch
        {
            "weekly" => dates.GroupBy(d => (ISOWeek.GetYear(d.ToDateTime(TimeOnly.MinValue)),
                ISOWeek.GetWeekOfYear(d.ToDateTime(TimeOnly.MinValue)))).Select(g => g.Last()).ToList(),
            "monthly" => dates.GroupBy(d => (d.Year, d.Month)).Select(g => g.Last()).ToList(),
            _ => dates
        };
}

// Extension to support keying by SchemeCode
internal static class BasketFundAllocationExtensions
{
    public static int Key(this BasketFundAllocation alloc) => alloc.SchemeCode;
}
