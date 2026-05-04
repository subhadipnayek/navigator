import sys
import re

file_path = r"d:\Projects\NAVigator\api-NET\NAVigator\Services\PortfolioSimulationService.cs"
with open(file_path, "r", encoding="utf-8") as f:
    content = f.read()

# 1. Update SimulateAsync
simulate_async_pattern = r"    public async Task<SimulationResponse> SimulateAsync.*?return simResult;\s*\n    }"
simulate_async_replacement = """    public async Task<SimulationResponse> SimulateAsync(SimulationRequest request, CancellationToken ct = default)
    {
        var startDate = ParseDate(request.StartDate);
        var endDate = ParseDate(request.EndDate);
        var basket = request.Basket;

        var (effectiveStart, effectiveEnd, wasClamped, dateRangeNote) =
            await ComputeGlobalEffectiveDateRangeAsync([basket], startDate, endDate, ct);

        var simResult = await RunSimulationCoreAsync(
            basket, request.SipAmount, request.SipDay, request.LumpSumAmount,
            effectiveStart, effectiveEnd, ct);

        simResult.StartDate = request.StartDate;
        simResult.EndDate = request.EndDate;
        simResult.EffectiveStartDate = effectiveStart.ToString(DateFmt, Inv);
        simResult.EffectiveEndDate = effectiveEnd.ToString(DateFmt, Inv);
        simResult.WasDateRangeClamped = wasClamped;
        simResult.DateRangeNote = dateRangeNote;

        return simResult;
    }"""
content = re.sub(simulate_async_pattern, simulate_async_replacement, content, flags=re.DOTALL)

# 2. Replace RunSimulationCoreAsync up to BuildTimeSeries
core_async_pattern = r"    internal async Task<SimulationResponse> RunSimulationCoreAsync\(.*?return new SimulationResponse.*?};.*?    }"
core_async_replacement = """    internal async Task<SimulationResponse> RunSimulationCoreAsync(
        BasketDefinition basket, decimal sipAmount, int sipDay, decimal lumpSum,
        DateOnly startDate, DateOnly endDate, CancellationToken ct)
    {
        // 1. Ensure NAV data is fresh and load fund metadata
        logger.LogInformation("Running simulation for basket '{Basket}' with {FundCount} funds, SIP ₹{Sip}, Lump ₹{Lump}, {Start} to {End}",
            basket.Name, basket.Funds.Count, sipAmount, lumpSum, startDate, endDate);

        var fundMeta = new Dictionary<int, (Fund Fund, List<CorporateActionDto> Actions, decimal AllocationPct)>();
        foreach (var alloc in basket.Funds)
        {
            await navService.GetNavHistoryAsync(alloc.SchemeCode, ct: ct);
            var fund = await fundRepo.GetBySchemeCodeAsync(alloc.SchemeCode)
                ?? throw new ArgumentException($"Fund {alloc.SchemeCode} not found.");
            var actions = await corporateActionService.GetActionsAsync(alloc.SchemeCode);
            fundMeta[alloc.SchemeCode] = (fund, actions, alloc.AllocationPercent);
        }

        // 2. Load NAV histories with effective date range
        var fundData = new Dictionary<int, FundNavData>();
        foreach (var alloc in basket.Funds)
        {
            var (fund, actions, allocPct) = fundMeta[alloc.SchemeCode];
            var navs = await navRepo.GetBySchemeCodeAsync(alloc.SchemeCode, startDate, endDate);
            var sorted = navs.OrderBy(n => n.Date).ToList();
            fundData[alloc.SchemeCode] = new FundNavData(fund, sorted, actions, allocPct);
        }

        // 3. Setup sequential simulation state
        var state = new Dictionary<int, FundState>();
        foreach (var kvp in fundData)
            state[kvp.Key] = new FundState();

        decimal totalInvested = 0m;
        int sipInstallments = 0;
        var cashFlows = new List<(DateTime Date, double Amount)>();
        var timeSeries = new List<PortfolioSnapshot>();

        var sipDates = sipAmount > 0 ? GenerateSipDates(startDate, endDate, sipDay) : [];
        int sipIdx = 0;

        var rebalDates = basket.RebalanceFrequency.Equals("None", StringComparison.OrdinalIgnoreCase)
            ? [] : GenerateRebalanceDates(basket.RebalanceFrequency, startDate, endDate);

        // Lump sum
        if (lumpSum > 0)
        {
            foreach (var kvp in fundData)
            {
                var amount = lumpSum * kvp.Value.AllocationPct / 100m;
                var nav = FindNavOnOrAfter(kvp.Value.Navs, startDate);
                if (nav is null) continue;
                state[kvp.Key].Units += amount / nav.Nav;
                state[kvp.Key].Invested += amount;
                state[kvp.Key].LastProcessedDate = nav.Date;
            }
            totalInvested += lumpSum;
            cashFlows.Add((startDate.ToDateTime(TimeOnly.MinValue), -(double)lumpSum));
        }

        // Walk month by month
        var month = new DateOnly(startDate.Year, startDate.Month, 1).AddMonths(1);
        while (month <= endDate.AddMonths(1))
        {
            var snapDate = month <= endDate ? new DateOnly(month.Year, month.Month,
                Math.Min(DateTime.DaysInMonth(month.Year, month.Month),
                month == endDate.AddMonths(1) ? endDate.Day : DateTime.DaysInMonth(month.Year, month.Month)))
                : endDate;
            if (snapDate > endDate) snapDate = endDate;

            // Process SIPs up to this snapshot
            while (sipIdx < sipDates.Count && sipDates[sipIdx] <= snapDate)
            {
                bool anyProcessed = false;
                foreach (var kvp in fundData)
                {
                    var amount = sipAmount * kvp.Value.AllocationPct / 100m;
                    var nav = FindNavOnOrAfter(kvp.Value.Navs, sipDates[sipIdx]);
                    if (nav is null) continue;
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
                    cashFlows.Add((sipDates[sipIdx].ToDateTime(TimeOnly.MinValue), -(double)sipAmount));
                }
                sipIdx++;
            }

            // Apply rebalancing
            if (rebalDates.Count > 0)
            {
                var lastMonth = month.AddMonths(-1);
                var rebalDateInWindow = rebalDates.FirstOrDefault(d => d > lastMonth && d <= snapDate);
                
                if (rebalDateInWindow != default)
                {
                    decimal totalValueForRebal = 0;
                    var currentValues = new Dictionary<int, decimal>();
                    foreach (var kvp in fundData)
                    {
                        var nav = FindNavOnOrAfter(kvp.Value.Navs, rebalDateInWindow);
                        if (nav is null) continue;
                        ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions, state[kvp.Key].LastProcessedDate, nav.Date);
                        state[kvp.Key].LastProcessedDate = nav.Date;
                        var val = state[kvp.Key].Units * nav.Nav;
                        currentValues[kvp.Key] = val;
                        totalValueForRebal += val;
                    }

                    if (totalValueForRebal > 0)
                    {
                        foreach (var kvp in fundData)
                        {
                            if (!currentValues.ContainsKey(kvp.Key)) continue;
                            var nav = FindNavOnOrAfter(kvp.Value.Navs, rebalDateInWindow);
                            if (nav is null || nav.Nav <= 0) continue;
                            var targetValue = totalValueForRebal * kvp.Value.AllocationPct / 100m;
                            state[kvp.Key].Units = targetValue / nav.Nav;
                            // Rebalance the "Invested" amount proportionally to keep return calculations per fund sane
                            state[kvp.Key].Invested = totalInvested * kvp.Value.AllocationPct / 100m;
                        }
                    }
                }
            }

            // Value the portfolio at snapDate
            decimal portfolioValue = 0;
            var fundSnapshots = new List<FundSnapshot>();
            foreach (var kvp in fundData)
            {
                var nav = FindNavOnOrBefore(kvp.Value.Navs, snapDate);
                if (nav is null) continue;
                ApplyCorporateActions(state[kvp.Key], kvp.Value.Actions, state[kvp.Key].LastProcessedDate, nav.Date);
                state[kvp.Key].LastProcessedDate = nav.Date;
                var val = state[kvp.Key].Units * nav.Nav;
                portfolioValue += val;
                fundSnapshots.Add(new FundSnapshot
                {
                    SchemeCode = kvp.Key,
                    Value = Math.Round(val, 2),
                    Units = Math.Round(state[kvp.Key].Units, 4)
                });
            }

            timeSeries.Add(new PortfolioSnapshot
            {
                Date = snapDate.ToString(DateFmt, Inv),
                PortfolioValue = Math.Round(portfolioValue, 2),
                TotalInvested = Math.Round(totalInvested, 2),
                GainPercent = totalInvested > 0 ? Math.Round((portfolioValue - totalInvested) / totalInvested * 100m, 2) : 0,
                Funds = fundSnapshots
            });

            if (snapDate >= endDate) break;
            month = month.AddMonths(1);
        }

        // 4. Final values from the last snapshot
        decimal finalCorpus = timeSeries.Count > 0 ? timeSeries.Last().PortfolioValue : 0;
        var fundDetails = new List<FundSimulationDetail>();
        foreach (var kvp in fundData)
        {
            var val = timeSeries.Count > 0 ? timeSeries.Last().Funds.FirstOrDefault(f => f.SchemeCode == kvp.Key)?.Value ?? 0 : 0;
            var units = timeSeries.Count > 0 ? timeSeries.Last().Funds.FirstOrDefault(f => f.SchemeCode == kvp.Key)?.Units ?? 0 : 0;
            var invested = state[kvp.Key].Invested;
            
            fundDetails.Add(new FundSimulationDetail
            {
                SchemeCode = kvp.Key,
                SchemeName = kvp.Value.Fund.SchemeName,
                AllocationPercent = kvp.Value.AllocationPct,
                TotalUnits = units,
                TotalInvested = Math.Round(invested, 2),
                FinalValue = val,
                AbsoluteReturnPercent = invested > 0 ? Math.Round((val - invested) / invested * 100m, 2) : 0
            });
        }

        // 5. Compute aggregate metrics
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
            SipInstallments = sipInstallments,
            SipAmount = sipAmount,
            LumpSumAmount = lumpSum,
            FundDetails = fundDetails,
            TimeSeries = timeSeries
            // Dates and Note will be populated by the caller (SimulateAsync, CompareAsync, etc.)
        };
    }"""
content = re.sub(core_async_pattern, core_async_replacement, content, flags=re.DOTALL)

# 3. Remove ApplyRebalancing and BuildTimeSeries entirely
remove_pattern = r"    private void ApplyRebalancing\(.*?return snapshots;\s*\n    }"
content = re.sub(remove_pattern, "", content, flags=re.DOTALL)

with open(file_path, "w", encoding="utf-8") as f:
    f.write(content)
