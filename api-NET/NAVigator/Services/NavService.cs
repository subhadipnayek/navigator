using System.Collections.Concurrent;
using System.Globalization;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class NavService(
    INavHistoryRepository navRepo,
    IFundRepository fundRepo,
    IAmfiService amfiService,
    ICorporateActionService corporateActionService,
    ILogger<NavService> logger) : INavService
{
    /// <summary>
    /// Cooldown period: if we checked the API and found no new data,
    /// don't hit the API again for this fund until this duration elapses.
    /// </summary>
    private static readonly TimeSpan ApiCooldown = TimeSpan.FromHours(1);

    /// <summary>
    /// Starting point for first-time historical syncs. AMFI returns empty slices for
    /// periods before a scheme exists, so this is safe for older and newer funds.
    /// </summary>
    private static readonly DateOnly HistoricalFetchStartDate = new(1990, 1, 1);

    /// <summary>
    /// Per-scheme lock to prevent concurrent API fetches and DB inserts for the same fund.
    /// </summary>
    private static readonly ConcurrentDictionary<int, SemaphoreSlim> SchemeLocks = new();

    public async Task<NavDataResponse?> GetNavHistoryAsync(int schemeCode, DateOnly? from = null, DateOnly? to = null, string? interval = null, bool adjusted = false, CancellationToken ct = default)
    {
        // 1. Verify the fund exists in our master list
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
        {
            logger.LogWarning("Fund {SchemeCode} not found in database", schemeCode);
            return null;
        }

        // Acquire a per-scheme lock so concurrent requests for the same fund
        // don't race through the check-then-insert sequence simultaneously.
        var schemeLock = SchemeLocks.GetOrAdd(schemeCode, _ => new SemaphoreSlim(1, 1));
        await schemeLock.WaitAsync(ct);
        try
        {
            // 2. Get latest NAV date from DB
            var latestDbDate = await navRepo.GetLatestNavDateAsync(schemeCode);
            var today = DateOnly.FromDateTime(TimeZoneInfo.ConvertTimeFromUtc(
                DateTime.UtcNow, TimeZoneInfo.FindSystemTimeZoneById("India Standard Time")));

            // 2b. Reconcile: if NavHistory has newer data than the Fund record, sync it
            if (latestDbDate.HasValue && (!fund.LatestNavDate.HasValue || latestDbDate.Value > fund.LatestNavDate.Value))
            {
                var latestEntry = await navRepo.GetLatestNavEntryAsync(schemeCode);
                if (latestEntry is not null)
                {
                    await fundRepo.UpdateLatestNavAsync(schemeCode, latestEntry.Nav, latestEntry.Date);
                    fund.LatestNav = latestEntry.Nav;
                    fund.LatestNavDate = latestEntry.Date;
                }
            }

            // 3. If latest NAV in DB has reached the known master-list date, serve from DB.
            var syncTargetDate = fund.LatestNavDate.HasValue && fund.LatestNavDate.Value < today
                ? fund.LatestNavDate.Value
                : today;

            if (latestDbDate.HasValue && latestDbDate.Value >= syncTargetDate)
            {
                logger.LogDebug(
                    "Fund {SchemeCode}: NAV is current ({DbDate}) for sync target {TargetDate}, serving from DB",
                    schemeCode,
                    latestDbDate,
                    syncTargetDate);
                return await BuildResponseFromDb(fund, false, from, to, interval, adjusted);
            }

            // 4. Check cooldown tracker — should we call the API?
            //    Always call the API if the DB has no data at all (bypass cooldown).
            var tracker = await navRepo.GetSyncTrackerAsync(schemeCode);
            bool shouldCallApi = !latestDbDate.HasValue || ShouldCallApi(tracker, syncTargetDate);

            if (!shouldCallApi)
            {
                logger.LogDebug(
                    "Fund {SchemeCode}: API on cooldown (last check: {LastCheck}), serving from DB",
                    schemeCode, tracker?.LastApiCheckUtc);
                return await BuildResponseFromDb(fund, false, from, to, interval, adjusted);
            }

            // 5. Call the external API for the missing date range only.
            var fetchFrom = latestDbDate.HasValue ? latestDbDate.Value.AddDays(1) : HistoricalFetchStartDate;
            logger.LogInformation(
                "Fund {SchemeCode}: Fetching NAV history from AMFI from {From} to {To}",
                schemeCode,
                fetchFrom,
                syncTargetDate);
            var apiResponse = await amfiService.FetchNavHistoryAsync(schemeCode, fetchFrom, syncTargetDate, ct);

            if (apiResponse is null || apiResponse.Data.Count == 0)
            {
                // API failed or returned empty — mark tracker so we don't retry immediately
                await navRepo.UpsertSyncTrackerAsync(new NavSyncTracker
                {
                    SchemeCode = schemeCode,
                    LastApiCheckUtc = DateTime.UtcNow,
                    LatestNavDateFromApi = latestDbDate,
                    LastCheckHadNewData = false
                });

                return await BuildResponseFromDb(fund, false, from, to, interval, adjusted);
            }

            // 6. Parse and store new NAV entries
            var newEntries = new List<NavHistory>();
            DateOnly? apiLatestDate = null;

            foreach (var entry in apiResponse.Data)
            {
                if (!DateOnly.TryParseExact(entry.Date, "yyyy-MM-dd", CultureInfo.InvariantCulture,
                        DateTimeStyles.None, out var date))
                    continue;

                var nav = entry.Nav;
                if (nav <= 0)
                    continue;

                if (apiLatestDate is null || date > apiLatestDate)
                    apiLatestDate = date;

                // Only add entries newer than what we already have
                if (!latestDbDate.HasValue || date > latestDbDate.Value)
                {
                    newEntries.Add(new NavHistory
                    {
                        SchemeCode = schemeCode,
                        Date = date,
                        Nav = nav
                    });
                }
            }

            bool hadNewData = newEntries.Count > 0;

            if (hadNewData)
            {
                await navRepo.AddRangeAsync(newEntries);
                logger.LogInformation("Fund {SchemeCode}: Stored {Count} new NAV entries", schemeCode, newEntries.Count);

                // Update the fund's latest NAV so the funds endpoint stays in sync
                var latestEntry = newEntries.OrderByDescending(e => e.Date).First();
                if (!fund.LatestNavDate.HasValue || latestEntry.Date > fund.LatestNavDate.Value)
                {
                    await fundRepo.UpdateLatestNavAsync(schemeCode, latestEntry.Nav, latestEntry.Date);
                }
            }
            else
            {
                logger.LogDebug("Fund {SchemeCode}: API had no newer data than DB ({DbDate})", schemeCode, latestDbDate);
            }

            // 7. Update the sync tracker
            await navRepo.UpsertSyncTrackerAsync(new NavSyncTracker
            {
                SchemeCode = schemeCode,
                LastApiCheckUtc = DateTime.UtcNow,
                LatestNavDateFromApi = apiLatestDate ?? latestDbDate,
                LastCheckHadNewData = hadNewData
            });

            return await BuildResponseFromDb(fund, true, from, to, interval, adjusted);
        }
        finally
        {
            schemeLock.Release();
        }
    }

    public async Task<List<NavGapDto>?> DetectNavGapsAsync(int schemeCode, int minGapDays = 30, CancellationToken ct = default)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
            return null;

        var histories = await navRepo.GetBySchemeCodeAsync(schemeCode);
        return DetectGaps(fund, histories, minGapDays);
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
            result.RepairedGaps++;
            result.InsertedRecords += entries.Count;
            result.Gaps.Add(ToRepairItem(gap, "Repaired", entries.Count));
        }

        return result;
    }

    /// <summary>
    /// Determines whether we should call the external API based on the sync tracker state.
    /// </summary>
    private static bool ShouldCallApi(NavSyncTracker? tracker, DateOnly today)
    {
        // Never checked before — definitely call
        if (tracker is null)
            return true;

        // If the last check DID return new data and the latest date from API is today,
        // no need to check again today
        if (tracker.LastCheckHadNewData && tracker.LatestNavDateFromApi.HasValue
            && tracker.LatestNavDateFromApi.Value >= today)
            return false;

        // If the last check had NO new data, enforce cooldown
        if (!tracker.LastCheckHadNewData)
        {
            var elapsed = DateTime.UtcNow - tracker.LastApiCheckUtc;
            if (elapsed < ApiCooldown)
                return false;
        }

        return true;
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
        var histories = await navRepo.GetBySchemeCodeAsync(fund.SchemeCode);

        // Capture the latest date before any filtering (for freshness indicator)
        var latestNavDate = histories.Count > 0
            ? histories[0].Date.ToString("dd-MM-yyyy")
            : null;

        // Load corporate actions for this fund (always needed for the response)
        var actions = await corporateActionService.GetActionsAsync(fund.SchemeCode);

        // Build adjustment lookup: for each NAV entry, compute the cumulative factor
        // from that date to the latest date so pre-action NAVs become comparable.
        DateOnly? overallLatestDate = histories.Count > 0 ? histories[0].Date : null;

        // When adjusted, truncate data before the latest discontinuity
        // (pre-discontinuity NAVs are not meaningfully connected to post-discontinuity)
        if (adjusted && actions.Count > 0)
        {
            var latestDiscontinuity = actions
                .Where(a => a.ActionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase))
                .Select(a => DateOnly.TryParseExact(a.ActionDate, "dd-MM-yyyy",
                    System.Globalization.CultureInfo.InvariantCulture,
                    System.Globalization.DateTimeStyles.None, out var d) ? d : (DateOnly?)null)
                .Where(d => d.HasValue)
                .Max();

            if (latestDiscontinuity.HasValue)
            {
                histories = histories
                    .Where(h => h.Date >= latestDiscontinuity.Value)
                    .ToList();
            }
        }

        // Apply date range filter
        if (from.HasValue || to.HasValue)
        {
            histories = histories
                .Where(h => (!from.HasValue || h.Date >= from.Value) && (!to.HasValue || h.Date <= to.Value))
                .ToList();
        }

        // Apply interval sampling (data is newest-first, sampling needs ascending order)
        if (!string.IsNullOrEmpty(interval) && !interval.Equals("daily", StringComparison.OrdinalIgnoreCase))
        {
            var ascending = histories.AsEnumerable().Reverse().ToList();
            ascending = SampleByInterval(ascending, interval, h => h.Date);
            histories = ascending.AsEnumerable().Reverse().ToList();
        }

        return new NavDataResponse
        {
            SchemeCode = fund.SchemeCode,
            SchemeName = fund.SchemeName,
            FundHouse = fund.FundHouse,
            LatestNavDate = latestNavDate,
            WasFetchedFromApi = wasFetchedFromApi,
            IsAdjusted = adjusted,
            Data = histories.Select(h =>
            {
                var nav = h.Nav;
                if (adjusted && overallLatestDate.HasValue && actions.Count > 0)
                {
                    var factor = ComputeAdjustmentFactor(actions, h.Date, overallLatestDate.Value);
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
        var startEntry = FindClosestNav(navByDate, startDate) ?? navByDate[0];
        var endEntry = FindClosestNav(navByDate, endDate) ?? navByDate[^1];

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
        decimal? cagr = days > 0
            ? (decimal)(Math.Pow((double)(endEntry.Nav / adjustedStartNav), 365.25 / days) - 1) * 100m
            : null;

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

        if (histories.Count == 0)
            return new RollingReturnResponse
            {
                SchemeCode = schemeCode,
                SchemeName = fund.SchemeName,
                FundHouse = fund.FundHouse,
                PeriodInYears = periodInYears
            };

        // Sort ascending by date
        var navByDate = histories.OrderBy(h => h.Date).ToList();

        // Pre-load corporate actions for this fund to avoid repeated DB calls
        var actions = await corporateActionService.GetActionsAsync(schemeCode);

        // Pre-compute discontinuity dates so we can skip windows that span them
        var discontinuityDates = actions
            .Where(a => a.ActionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase))
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
            var startNav = FindClosestNavValue(navByDate, startDate, toleranceDays: 7);
            if (startNav is null)
                continue;

            // Skip windows that span a data discontinuity — returns across it are unreliable
            if (discontinuityDates.Any(d => d > startNav.Value.Date && d <= entry.Date))
                continue;

            var days = entry.Date.DayNumber - startDate.DayNumber;
            if (days <= 0 || startNav.Value.Nav <= 0)
                continue;

            // Adjust start NAV for any corporate actions between start and end dates
            var adjustmentFactor = ComputeAdjustmentFactor(actions, startNav.Value.Date, entry.Date);
            var adjustedStartNav = startNav.Value.Nav / adjustmentFactor;

            var cagr = (decimal)(Math.Pow((double)(entry.Nav / adjustedStartNav), 365.25 / days) - 1) * 100m;

            rollingReturns.Add((entry.Date, Math.Round(cagr, 2)));
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
            median = values.Count % 2 == 0
                ? Math.Round((values[values.Count / 2 - 1] + values[values.Count / 2]) / 2m, 2)
                : values[values.Count / 2];
        }

        // Apply interval sampling to reduce chart data points
        var sampledReturns = SampleByInterval(rollingReturns, interval, r => r.Date);

        return new RollingReturnResponse
        {
            SchemeCode = schemeCode,
            SchemeName = fund.SchemeName,
            FundHouse = fund.FundHouse,
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

    /// <summary>
    /// Reduces data points by selecting the last entry per week or month.
    /// Input must be sorted ascending by date.
    /// </summary>
    private static List<T> SampleByInterval<T>(List<T> sortedAsc, string? interval, Func<T, DateOnly> dateSelector)
    {
        if (string.IsNullOrEmpty(interval) || interval.Equals("daily", StringComparison.OrdinalIgnoreCase))
            return sortedAsc;

        if (interval.Equals("weekly", StringComparison.OrdinalIgnoreCase))
        {
            return sortedAsc
                .GroupBy(item =>
                {
                    var dt = dateSelector(item).ToDateTime(TimeOnly.MinValue);
                    return (ISOWeek.GetYear(dt), ISOWeek.GetWeekOfYear(dt));
                })
                .Select(g => g.Last())
                .ToList();
        }

        if (interval.Equals("monthly", StringComparison.OrdinalIgnoreCase))
        {
            return sortedAsc
                .GroupBy(item =>
                {
                    var date = dateSelector(item);
                    return (date.Year, date.Month);
                })
                .Select(g => g.Last())
                .ToList();
        }

        return sortedAsc;
    }

    /// <summary>
    /// Finds the NAV entry closest to the target date (on or before, within 7 days).
    /// </summary>
    private static NavHistory? FindClosestNav(List<NavHistory> sortedAsc, DateOnly targetDate)
    {
        // Binary search for the target date
        int idx = sortedAsc.BinarySearch(null!, Comparer<NavHistory?>.Create((a, b) =>
        {
            var dateA = a?.Date ?? targetDate;
            var dateB = b?.Date ?? targetDate;
            return dateA.CompareTo(dateB);
        }));

        if (idx >= 0)
            return sortedAsc[idx];

        // ~idx is the index of the next element greater than target
        int insertionPoint = ~idx;

        // We want the entry on or just before the target date
        if (insertionPoint > 0)
        {
            var candidate = sortedAsc[insertionPoint - 1];
            if (targetDate.DayNumber - candidate.Date.DayNumber <= 7)
                return candidate;
        }

        // Or the entry just after (within 7 days)
        if (insertionPoint < sortedAsc.Count)
        {
            var candidate = sortedAsc[insertionPoint];
            if (candidate.Date.DayNumber - targetDate.DayNumber <= 7)
                return candidate;
        }

        return null;
    }

    /// <summary>
    /// Finds the NAV value closest to the target date within the specified tolerance.
    /// </summary>
    private static (DateOnly Date, decimal Nav)? FindClosestNavValue(List<NavHistory> sortedAsc, DateOnly targetDate, int toleranceDays)
    {
        var entry = FindClosestNav(sortedAsc, targetDate);
        if (entry is null)
            return null;

        if (Math.Abs(entry.Date.DayNumber - targetDate.DayNumber) <= toleranceDays)
            return (entry.Date, entry.Nav);

        return null;
    }

    /// <summary>
    /// Computes cumulative adjustment factor from pre-loaded corporate actions DTOs.
    /// Used in rolling returns to avoid repeated DB calls per data point.
    /// </summary>
    private static decimal ComputeAdjustmentFactor(List<CorporateActionDto> actions, DateOnly fromDate, DateOnly toDate)
    {
        if (actions.Count == 0)
            return 1m;

        decimal factor = 1m;
        foreach (var action in actions)
        {
            if (!DateOnly.TryParseExact(action.ActionDate, "dd-MM-yyyy",
                    System.Globalization.CultureInfo.InvariantCulture,
                    System.Globalization.DateTimeStyles.None, out var actionDate))
                continue;

            // Discontinuity actions have no meaningful adjustment factor — skip them
            if (action.ActionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase))
                continue;

            if (actionDate > fromDate && actionDate <= toDate)
                factor *= action.AdjustmentFactor;
        }

        return factor;
    }
}
