using System.Globalization;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class NavService(
    AppDbContext db,
    INavHistoryRepository navRepo,
    IFundRepository fundRepo,
    IAmfiService amfiService,
    ICorporateActionService corporateActionService,
    INavHistoryProvider navHistoryProvider,
    ILogger<NavService> logger) : INavService
{
    private const int LikelyDiscontinuedGraceDays = 30;

    public async Task<Dictionary<int, List<NavHistory>>> EnsureNavHistoriesAsync(
        IEnumerable<int> schemeCodes,
        DateOnly? from = null,
        DateOnly? to = null,
        CancellationToken ct = default)
    {
        var snapshots = await navHistoryProvider.GetHistoriesAsync(
            schemeCodes,
            new NavHistoryScope(from, to, AllHistory: !from.HasValue && !to.HasValue),
            ct);

        return snapshots.ToDictionary(kvp => kvp.Key, kvp => kvp.Value.Rows);
    }

    public async Task<NavDataResponse?> GetNavHistoryAsync(int schemeCode, DateOnly? from = null, DateOnly? to = null, string? interval = null, bool adjusted = false, CancellationToken ct = default)
    {
        var snapshot = await navHistoryProvider.GetHistoryAsync(
            schemeCode,
            new NavHistoryScope(from, to, AllHistory: !from.HasValue && !to.HasValue),
            ct);

        return snapshot is null
            ? null
            : await BuildResponseFromDb(snapshot.Fund, snapshot.WasFetchedFromApi, from, to, interval, adjusted);

    }

    public async Task<List<NavGapDto>?> DetectNavGapsAsync(int schemeCode, int minGapDays = 30, CancellationToken ct = default)
    {
        var snapshot = await navHistoryProvider.GetHistoryAsync(
            schemeCode,
            new NavHistoryScope(AllHistory: true),
            ct);
        if (snapshot is null)
            return null;

        return DetectGaps(snapshot.Fund, snapshot.Rows, minGapDays);
    }

    public async Task<List<NavGapDto>> DetectAllNavGapsAsync(int minGapDays = 30, CancellationToken ct = default)
    {
        logger.LogInformation("Detecting NAV gaps across all funds with minimum gap {MinGapDays} days", minGapDays);

        // Use SQL LAG() window function to detect gaps without loading full history into memory
        var sql = @"
            SELECT scheme_code AS scheme_code, date AS current_date, nav AS nav,
                   LAG(date) OVER (PARTITION BY scheme_code ORDER BY date) AS previous_date
            FROM ""NavHistories""
            ORDER BY scheme_code, date";

        var rawGaps = await db.Database.SqlQueryRaw<NavGapRawRow>(sql).ToListAsync(ct);

        if (rawGaps.Count == 0)
            return [];

        // Filter to only rows with gaps exceeding threshold
        var gapRows = rawGaps
            .Where(r => r.PreviousDate.HasValue
                        && (r.CurrentDate.DayNumber - r.PreviousDate.Value.DayNumber) > minGapDays)
            .ToList();

        if (gapRows.Count == 0)
            return [];

        var schemeCodes = gapRows.Select(r => r.SchemeCode).Distinct().ToList();
        var funds = await db.Funds
            .AsNoTracking()
            .Where(f => schemeCodes.Contains(f.SchemeCode))
            .ToDictionaryAsync(f => f.SchemeCode, ct);

        var gaps = new List<NavGapDto>();
        foreach (var row in gapRows)
        {
            if (!funds.TryGetValue(row.SchemeCode, out var fund))
                continue;

            var gapDays = row.CurrentDate.DayNumber - row.PreviousDate!.Value.DayNumber;
            var missingFrom = row.PreviousDate.Value.AddDays(1);
            var missingTo = row.CurrentDate.AddDays(-1);

            gaps.Add(new NavGapDto
            {
                SchemeCode = fund.SchemeCode,
                SchemeName = fund.SchemeName,
                FundHouse = fund.FundHouse,
                PreviousDate = row.PreviousDate.Value.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                NextDate = row.CurrentDate.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                MissingFrom = missingFrom.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                MissingTo = missingTo.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                GapDays = gapDays,
                MissingCalendarDays = missingTo.DayNumber - missingFrom.DayNumber + 1
            });
        }

        logger.LogInformation("Found {Count} NAV gaps across all funds", gaps.Count);

        return gaps
            .OrderByDescending(g => g.GapDays)
            .ThenBy(g => g.SchemeName)
            .ToList();
    }

    public async Task<NavGapRepairResultDto?> RepairNavGapsAsync(int schemeCode, int minGapDays = 30, CancellationToken ct = default)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
            return null;

        var histories = await navRepo.GetBySchemeCodeAsync(schemeCode);
        var gaps = DetectGaps(fund, histories, minGapDays);
        var result = new NavGapRepairResultDto
        {
            SchemeCode = fund.SchemeCode,
            SchemeName = fund.SchemeName,
            DetectedGaps = gaps.Count
        };

        foreach (var gap in gaps)
        {
            if (!DateOnly.TryParseExact(gap.MissingFrom, "dd-MM-yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None, out var missingFrom)
                || !DateOnly.TryParseExact(gap.MissingTo, "dd-MM-yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None, out var missingTo))
                continue;

            var apiResponse = await amfiService.FetchNavHistoryAsync(
                schemeCode,
                missingFrom,
                missingTo,
                ct,
                stopAtFirstOlderEmptyChunk: false);
            if (apiResponse is null)
            {
                result.Gaps.Add(ToRepairItem(gap, "FetchFailed", 0));
                continue;
            }

            var entries = new List<NavHistory>();
            foreach (var entry in apiResponse.Data)
            {
                if (!DateOnly.TryParseExact(entry.Date, "yyyy-MM-dd", CultureInfo.InvariantCulture, DateTimeStyles.None, out var date))
                    continue;

                if (date < missingFrom || date > missingTo || entry.Nav <= 0)
                    continue;

                entries.Add(new NavHistory
                {
                    SchemeCode = schemeCode,
                    Date = date,
                    Nav = entry.Nav
                });
            }

            if (entries.Count == 0)
            {
                result.VerifiedEmptyGaps++;
                result.Gaps.Add(ToRepairItem(gap, "NoRecordsReturned", 0));
                continue;
            }

            await navRepo.AddRangeAsync(entries);
            var stats = await navRepo.GetStatsAsync(schemeCode);
            await navRepo.UpdateTrackerHistoryStatsAsync(
                schemeCode,
                stats.StartDate,
                stats.EndDate,
                stats.Count,
                IndiaTime.Now(),
                hadNewData: true,
                ct);
            result.RepairedGaps++;
            result.InsertedRecords += entries.Count;
            result.Gaps.Add(ToRepairItem(gap, "Repaired", entries.Count));
        }

        return result;
    }

    private static List<NavGapDto> DetectGaps(Fund fund, List<NavHistory> histories, int minGapDays)
    {
        var sorted = histories.OrderBy(h => h.Date).ToList();
        if (sorted.Count < 2)
            return [];

        var gaps = new List<NavGapDto>();
        for (var i = 1; i < sorted.Count; i++)
        {
            var previous = sorted[i - 1];
            var next = sorted[i];
            var gapDays = next.Date.DayNumber - previous.Date.DayNumber;
            if (gapDays <= minGapDays)
                continue;

            var missingFrom = previous.Date.AddDays(1);
            var missingTo = next.Date.AddDays(-1);
            gaps.Add(new NavGapDto
            {
                SchemeCode = fund.SchemeCode,
                SchemeName = fund.SchemeName,
                FundHouse = fund.FundHouse,
                PreviousDate = previous.Date.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                NextDate = next.Date.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                MissingFrom = missingFrom.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                MissingTo = missingTo.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture),
                GapDays = gapDays,
                MissingCalendarDays = missingTo.DayNumber - missingFrom.DayNumber + 1
            });
        }

        return gaps;
    }

    private static NavGapRepairItemDto ToRepairItem(NavGapDto gap, string status, int insertedRecords)
    {
        return new NavGapRepairItemDto
        {
            SchemeCode = gap.SchemeCode,
            SchemeName = gap.SchemeName,
            FundHouse = gap.FundHouse,
            PreviousDate = gap.PreviousDate,
            NextDate = gap.NextDate,
            MissingFrom = gap.MissingFrom,
            MissingTo = gap.MissingTo,
            GapDays = gap.GapDays,
            MissingCalendarDays = gap.MissingCalendarDays,
            InsertedRecords = insertedRecords,
            Status = status
        };
    }

    private async Task<NavDataResponse> BuildResponseFromDb(Fund fund, bool wasFetchedFromApi, DateOnly? from = null, DateOnly? to = null, string? interval = null, bool adjusted = false)
    {
        var tracker = fund.NavSyncTracker;
        var latestNavDate = FormatDate(tracker?.LatestNavDate);
        var navStartDate = tracker?.NavStartDate;
        var navEndDate = tracker?.NavEndDate;

        // Load corporate actions for this fund (always needed for the response)
        var actions = await corporateActionService.GetActionsAsync(fund.SchemeCode);

        // When adjusted, determine a floor date from the latest series break
        DateOnly? adjustedFloor = null;
        if (adjusted && actions.Count > 0)
        {
            adjustedFloor = actions
                .Where(a => CorporateActionRules.IsSeriesBreakAction(a.ActionType))
                .Select(a => DateOnly.TryParseExact(a.ActionDate, "dd-MM-yyyy",
                    System.Globalization.CultureInfo.InvariantCulture,
                    System.Globalization.DateTimeStyles.None, out var d) ? d : (DateOnly?)null)
                .Where(d => d.HasValue)
                .Max();
        }

        // Compute effective date bounds for the DB query
        var effectiveFrom = from;
        if (adjustedFloor.HasValue)
            effectiveFrom = effectiveFrom.HasValue && effectiveFrom.Value > adjustedFloor.Value
                ? effectiveFrom
                : adjustedFloor;

        // Fetch only the needed date range from the database
        var histories = await navRepo.GetBySchemeCodeAsync(fund.SchemeCode, effectiveFrom, to);

        // If we had no cached metadata, compute it from what we loaded
        if (!navStartDate.HasValue && histories.Count > 0)
            navStartDate = histories.Min(h => h.Date);
        if (!navEndDate.HasValue && histories.Count > 0)
            navEndDate = histories.Max(h => h.Date);

        DateOnly? overallLatestDate = histories.Count > 0 ? histories[0].Date : null;

        // Apply interval sampling (data is newest-first, sampling needs ascending order)
        if (!string.IsNullOrEmpty(interval) && !interval.Equals("daily", StringComparison.OrdinalIgnoreCase))
        {
            var ascending = histories.AsEnumerable().Reverse().ToList();
            ascending = NavCalculationRules.SampleByInterval(ascending, interval, h => h.Date);
            histories = ascending.AsEnumerable().Reverse().ToList();
        }

        return new NavDataResponse
        {
            SchemeCode = fund.SchemeCode,
            SchemeName = fund.SchemeName,
            FundHouse = fund.FundHouse,
            LatestNavDate = latestNavDate,
            NavStartDate = FormatDate(navStartDate),
            NavEndDate = FormatDate(navEndDate),
            LastSuccessfulNavCheckUtc = FormatIst(tracker?.LastHistoryCheckAtIst ?? tracker?.LastLatestNavCheckAtIst),
            NavStatus = GetNavStatus(tracker?.LatestNavDate, tracker?.LastLatestNavCheckAtIst),
            IsLikelyDiscontinued = IsLikelyDiscontinued(tracker?.LatestNavDate, tracker?.LastLatestNavCheckAtIst),
            WasFetchedFromApi = wasFetchedFromApi,
            IsAdjusted = adjusted,
            Data = histories.Select(h =>
            {
                var nav = h.Nav;
                if (adjusted && overallLatestDate.HasValue && actions.Count > 0)
                {
                    var factor = CorporateActionRules.ComputeAdjustmentFactor(actions, h.Date, overallLatestDate.Value);
                    nav /= factor;
                }
                return new NavHistoryDto
                {
                    Date = h.Date.ToString("dd-MM-yyyy"),
                    Nav = Math.Round(nav, 6)
                };
            }).ToList(),
            CorporateActions = actions
        };
    }

    public async Task<ReturnsResponse?> GetReturnsAsync(int schemeCode, DateOnly startDate, DateOnly endDate, CancellationToken ct = default)
    {
        // Ensure NAV data is up-to-date
        var navResponse = await GetNavHistoryAsync(schemeCode, ct: ct);
        if (navResponse is null)
            return null;

        var fund = (await fundRepo.GetBySchemeCodeAsync(schemeCode))!;
        var histories = await navRepo.GetBySchemeCodeAsync(schemeCode);

        if (histories.Count == 0)
            return new ReturnsResponse
            {
                SchemeCode = schemeCode,
                SchemeName = fund.SchemeName,
                FundHouse = fund.FundHouse
            };

        var navByDate = histories.OrderBy(h => h.Date).ToList();

        // Clamp to max available range when requested dates exceed available data
        var startEntry = NavCalculationRules.FindClosestNav(navByDate, startDate) ?? navByDate[0];
        var endEntry = NavCalculationRules.FindClosestNav(navByDate, endDate) ?? navByDate[^1];

        // Check for data discontinuity between start and end dates
        var discontinuityDate = await corporateActionService.GetDiscontinuityDateAsync(
            schemeCode, startEntry.Date, endEntry.Date);
        bool hasDiscontinuity = discontinuityDate.HasValue;

        if (hasDiscontinuity)
        {
            // Clamp start to the first NAV on or after the discontinuity date
            var postDiscontinuity = navByDate.FirstOrDefault(n => n.Date >= discontinuityDate.Value);
            if (postDiscontinuity is not null && postDiscontinuity.Date < endEntry.Date)
                startEntry = postDiscontinuity;
        }

        if (startEntry.Date >= endEntry.Date || startEntry.Nav <= 0)
            return new ReturnsResponse
            {
                SchemeCode = schemeCode,
                SchemeName = fund.SchemeName,
                FundHouse = fund.FundHouse
            };

        // Adjust start NAV for any corporate actions (splits/mergers) between start and end dates
        var adjustmentFactor = await corporateActionService.GetCumulativeAdjustmentFactorAsync(
            schemeCode, startEntry.Date, endEntry.Date);
        var adjustedStartNav = startEntry.Nav / adjustmentFactor;

        var absoluteReturn = (endEntry.Nav - adjustedStartNav) / adjustedStartNav * 100m;
        var days = endEntry.Date.DayNumber - startEntry.Date.DayNumber;
        var cagr = NavCalculationRules.CalculateCagr(adjustedStartNav, endEntry.Nav, startEntry.Date, endEntry.Date);

        return new ReturnsResponse
        {
            SchemeCode = schemeCode,
            SchemeName = fund.SchemeName,
            FundHouse = fund.FundHouse,
            AbsoluteReturn = Math.Round(absoluteReturn, 2),
            Cagr = cagr.HasValue ? Math.Round(cagr.Value, 2) : null,
            StartDate = startEntry.Date.ToString("dd-MM-yyyy"),
            StartNav = Math.Round(adjustedStartNav, 6),
            EndDate = endEntry.Date.ToString("dd-MM-yyyy"),
            EndNav = endEntry.Nav,
            HasDiscontinuity = hasDiscontinuity
        };
    }

    public async Task<RollingReturnResponse?> GetRollingReturnsAsync(int schemeCode, int periodInYears, DateOnly? from = null, DateOnly? to = null, string? interval = null, CancellationToken ct = default)
    {
        // Ensure NAV data is up-to-date
        var navResponse = await GetNavHistoryAsync(schemeCode, ct: ct);
        if (navResponse is null)
            return null;

        var fund = (await fundRepo.GetBySchemeCodeAsync(schemeCode))!;
        var histories = await navRepo.GetBySchemeCodeAsync(schemeCode);
        var tracker = fund.NavSyncTracker;

        if (histories.Count == 0)
            return new RollingReturnResponse
            {
                SchemeCode = schemeCode,
                SchemeName = fund.SchemeName,
                FundHouse = fund.FundHouse,
                NavStartDate = FormatDate(tracker?.NavStartDate),
                NavEndDate = FormatDate(tracker?.NavEndDate),
                LastSuccessfulNavCheckUtc = FormatIst(tracker?.LastHistoryCheckAtIst ?? tracker?.LastLatestNavCheckAtIst),
                NavStatus = GetNavStatus(tracker?.LatestNavDate, tracker?.LastLatestNavCheckAtIst),
                IsLikelyDiscontinued = IsLikelyDiscontinued(tracker?.LatestNavDate, tracker?.LastLatestNavCheckAtIst),
                PeriodInYears = periodInYears
            };

        // Sort ascending by date
        var navByDate = histories.OrderBy(h => h.Date).ToList();

        // Pre-load corporate actions for this fund to avoid repeated DB calls
        var actions = await corporateActionService.GetActionsAsync(schemeCode);

        // Pre-compute series-break dates so we can skip windows that span them.
        var discontinuityDates = actions
            .Where(a => CorporateActionRules.IsSeriesBreakAction(a.ActionType))
            .Select(a => DateOnly.TryParseExact(a.ActionDate, "dd-MM-yyyy",
                System.Globalization.CultureInfo.InvariantCulture,
                System.Globalization.DateTimeStyles.None, out var d) ? d : (DateOnly?)null)
            .Where(d => d.HasValue)
            .Select(d => d!.Value)
            .ToList();

        var rollingReturns = new List<(DateOnly Date, decimal ReturnPercentage)>();

        foreach (var entry in navByDate)
        {
            var startDate = entry.Date.AddYears(-periodInYears);

            // Find the closest available NAV date to the target start date (within 7 days tolerance)
            var startNav = NavCalculationRules.FindClosestNavValue(navByDate, startDate);
            if (startNav is null)
                continue;

            // Skip windows that span a data discontinuity — returns across it are unreliable
            if (discontinuityDates.Any(d => d > startNav.Value.Date && d <= entry.Date))
                continue;

            if (startNav.Value.Nav <= 0)
                continue;

            // Adjust start NAV for any corporate actions between start and end dates
            var adjustmentFactor = CorporateActionRules.ComputeAdjustmentFactor(actions, startNav.Value.Date, entry.Date);
            var adjustedStartNav = startNav.Value.Nav / adjustmentFactor;

            var cagr = NavCalculationRules.CalculateCagr(adjustedStartNav, entry.Nav, startNav.Value.Date, entry.Date);
            if (!cagr.HasValue)
                continue;

            rollingReturns.Add((entry.Date, Math.Round(cagr.Value, 2)));
        }

        // Filter by date range (clamp to available range — no data is discarded if dates exceed bounds)
        if (from.HasValue || to.HasValue)
        {
            rollingReturns = rollingReturns
                .Where(r => (!from.HasValue || r.Date >= from.Value) && (!to.HasValue || r.Date <= to.Value))
                .ToList();
        }

        // Compute summary stats on filtered data points before interval sampling
        decimal? min = null, max = null, avg = null, median = null;
        if (rollingReturns.Count > 0)
        {
            var values = rollingReturns.Select(r => r.ReturnPercentage).OrderBy(v => v).ToList();
            min = values[0];
            max = values[^1];
            avg = Math.Round(values.Average(), 2);
            median = NavCalculationRules.GetMedian(values);
        }

        // Apply interval sampling to reduce chart data points
        var sampledReturns = NavCalculationRules.SampleByInterval(rollingReturns, interval, r => r.Date);

        return new RollingReturnResponse
        {
            SchemeCode = schemeCode,
            SchemeName = fund.SchemeName,
            FundHouse = fund.FundHouse,
            NavStartDate = FormatDate(tracker?.NavStartDate ?? navByDate[0].Date),
            NavEndDate = FormatDate(tracker?.NavEndDate ?? navByDate[^1].Date),
            LastSuccessfulNavCheckUtc = FormatIst(tracker?.LastHistoryCheckAtIst ?? tracker?.LastLatestNavCheckAtIst),
            NavStatus = GetNavStatus(tracker?.LatestNavDate, tracker?.LastLatestNavCheckAtIst),
            IsLikelyDiscontinued = IsLikelyDiscontinued(tracker?.LatestNavDate, tracker?.LastLatestNavCheckAtIst),
            PeriodInYears = periodInYears,
            DataPoints = rollingReturns.Count,
            MinReturn = min,
            MaxReturn = max,
            AverageReturn = avg,
            MedianReturn = median,
            Data = sampledReturns.Select(r => new RollingReturnDto
            {
                Date = r.Date.ToString("dd-MM-yyyy"),
                ReturnPercentage = r.ReturnPercentage
            }).ToList()
        };
    }

    private static string? FormatDate(DateOnly? date)
        => date?.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture);

    private static string? FormatIst(DateTime? dateTime)
        => dateTime?.ToString("O", CultureInfo.InvariantCulture);

    private static string GetNavStatus(DateOnly? latestNavDate, DateTime? lastSuccessfulCheckAtIst)
    {
        if (!latestNavDate.HasValue)
            return "Unknown";

        var today = IndiaTime.Today();
        if (today.DayNumber - latestNavDate.Value.DayNumber <= LikelyDiscontinuedGraceDays)
            return "Active";

        if (!lastSuccessfulCheckAtIst.HasValue)
            return "NeedsCheck";

        return IsLikelyDiscontinued(latestNavDate, lastSuccessfulCheckAtIst)
            ? "LikelyDiscontinued"
            : "NeedsCheck";
    }

    private static bool IsLikelyDiscontinued(DateOnly? latestNavDate, DateTime? lastSuccessfulCheckAtIst)
    {
        if (!latestNavDate.HasValue || !lastSuccessfulCheckAtIst.HasValue)
            return false;

        var lastCheckDate = DateOnly.FromDateTime(lastSuccessfulCheckAtIst.Value);
        return lastCheckDate.DayNumber - latestNavDate.Value.DayNumber > LikelyDiscontinuedGraceDays;
    }
}
