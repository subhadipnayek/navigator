using System.Globalization;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class NavHistoryProvider(
    AppDbContext db,
    INavHistoryRepository navRepo,
    IFundRepository fundRepo,
    IAmfiService amfiService,
    ICorporateActionService corporateActionService,
    ILogger<NavHistoryProvider> logger) : INavHistoryProvider
{
    private static readonly TimeSpan ApiCooldown = TimeSpan.FromHours(1);
    private static readonly DateOnly HistoricalFetchStartDate = new(1990, 1, 1);
    private const int RepairableGapDays = 30;

    public async Task<Dictionary<int, NavHistorySnapshot>> GetHistoriesAsync(
        IEnumerable<int> schemeCodes,
        NavHistoryScope scope,
        CancellationToken ct = default)
    {
        var result = new Dictionary<int, NavHistorySnapshot>();
        foreach (var schemeCode in schemeCodes.Distinct())
        {
            var snapshot = await GetHistoryAsync(schemeCode, scope, ct);
            if (snapshot is not null)
                result[schemeCode] = snapshot;
        }

        return result;
    }

    public async Task<NavHistorySnapshot?> GetHistoryAsync(
        int schemeCode,
        NavHistoryScope scope,
        CancellationToken ct = default)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
        {
            logger.LogWarning("Fund {SchemeCode} not found in database", schemeCode);
            return null;
        }

        await RefreshLatestNavTrackerIfStaleAsync(schemeCode, ct);

        var checkedHistoryApi = false;
        var fetchedFromApi = false;
        await using var advisoryTx = await db.Database.BeginTransactionAsync(ct);
        try
        {
            await db.Database.ExecuteSqlRawAsync(
                "SELECT pg_advisory_xact_lock({0})",
                schemeCode);
        }
        catch (Exception ex)
        {
            logger.LogWarning(ex, "Failed to acquire advisory lock for {SchemeCode}", schemeCode);
        }

        try
        {
            var tracker = await navRepo.GetSyncTrackerAsync(schemeCode);
            var stats = await navRepo.GetStatsAsync(schemeCode);
            var syncTargetDate = GetSyncTargetDate(tracker);
            var ensureFrom = GetRequestedFetchStart(scope, fund);
            var ensureTo = GetRequestedFetchEnd(scope, syncTargetDate);

            if (!stats.StartDate.HasValue)
            {
                var initialFrom = ensureFrom ?? fund.LaunchDate ?? HistoricalFetchStartDate;
                var fetchResult = await FetchAndStoreRangeAsync(
                    fund,
                    initialFrom,
                    ensureTo,
                    stopAtFirstOlderEmptyChunk: !scope.AllHistory,
                    ct);
                checkedHistoryApi |= fetchResult.WasChecked;
                fetchedFromApi |= fetchResult.HadNewData;
                stats = await navRepo.GetStatsAsync(schemeCode);
                tracker = await navRepo.GetSyncTrackerAsync(schemeCode);
            }

            if (ensureFrom.HasValue && stats.StartDate.HasValue && stats.StartDate.Value > ensureFrom.Value)
            {
                var fetchResult = await FetchAndStoreRangeAsync(
                    fund,
                    ensureFrom.Value,
                    stats.StartDate.Value.AddDays(-1),
                    stopAtFirstOlderEmptyChunk: false,
                    ct);
                checkedHistoryApi |= fetchResult.WasChecked;
                fetchedFromApi |= fetchResult.HadNewData;
                stats = await navRepo.GetStatsAsync(schemeCode);
            }

            if (stats.EndDate.HasValue && stats.EndDate.Value < ensureTo)
            {
                var shouldCallApi = ShouldCallApi(tracker, ensureTo);
                if (shouldCallApi)
                {
                    var fetchResult = await FetchAndStoreRangeAsync(
                        fund,
                        stats.EndDate.Value.AddDays(1),
                        ensureTo,
                        stopAtFirstOlderEmptyChunk: true,
                        ct);
                    checkedHistoryApi |= fetchResult.WasChecked;
                    fetchedFromApi |= fetchResult.HadNewData;
                    stats = await navRepo.GetStatsAsync(schemeCode);
                }
                else
                {
                    logger.LogDebug(
                        "Fund {SchemeCode}: history check on cooldown, serving stored rows through {StoredEndDate}",
                        schemeCode,
                        stats.EndDate);
                }
            }

            var gapRepairResult = await RepairRequestedGapsAsync(fund, scope, syncTargetDate, ct);
            checkedHistoryApi |= gapRepairResult.WasChecked;
            fetchedFromApi |= gapRepairResult.HadNewData;

            if (checkedHistoryApi)
            {
                stats = await navRepo.GetStatsAsync(schemeCode);
                await navRepo.UpdateTrackerHistoryStatsAsync(
                    schemeCode,
                    stats.StartDate,
                    stats.EndDate,
                    stats.Count,
                    IndiaTime.Now(),
                    fetchedFromApi,
                    ct);
            }

            await advisoryTx.CommitAsync(ct);
        }
        catch
        {
            await advisoryTx.RollbackAsync(ct);
            throw;
        }

        fund = await fundRepo.GetBySchemeCodeAsync(schemeCode) ?? fund;
        var rows = await navRepo.GetBySchemeCodeAsync(schemeCode, scope.AllHistory ? null : scope.From, scope.To);
        rows = rows.OrderBy(n => n.Date).ToList();
        fund.NavSyncTracker = await navRepo.GetSyncTrackerAsync(schemeCode);

        return new NavHistorySnapshot(fund, fund.NavSyncTracker, rows, fetchedFromApi);
    }

    public async Task<NavSyncTracker?> RefreshLatestNavTrackerIfStaleAsync(
        int schemeCode,
        CancellationToken ct = default)
    {
        var tracker = await navRepo.GetSyncTrackerAsync(schemeCode);
        var nowIst = IndiaTime.Now();
        if (tracker?.LastLatestNavCheckAtIst is { } lastCheck && nowIst - lastCheck < ApiCooldown)
            return tracker;

        try
        {
            var snapshots = await amfiService.FetchLatestNavSnapshotsAsync(ct);
            await navRepo.UpsertLatestNavSnapshotsAsync(snapshots, nowIst, ct);
            return await navRepo.GetSyncTrackerAsync(schemeCode);
        }
        catch (Exception ex) when (ex is not OperationCanceledException)
        {
            logger.LogWarning(
                ex,
                "Failed to refresh AMFI latest NAV tracker snapshots before serving scheme {SchemeCode}",
                schemeCode);
            return tracker;
        }
    }

    private async Task<NavFetchResult> RepairRequestedGapsAsync(
        Fund fund,
        NavHistoryScope scope,
        DateOnly syncTargetDate,
        CancellationToken ct)
    {
        var rows = await navRepo.GetBySchemeCodeAsync(fund.SchemeCode, scope.AllHistory ? null : scope.From, scope.To);
        var sorted = rows.OrderBy(n => n.Date).ToList();
        if (sorted.Count < 2)
            return NavFetchResult.NotChecked;

        var checkedApi = false;
        var fetched = false;
        var requestedFrom = scope.AllHistory ? null : scope.From;
        var requestedTo = scope.To ?? syncTargetDate;

        for (var i = 1; i < sorted.Count; i++)
        {
            var previous = sorted[i - 1];
            var next = sorted[i];
            if (next.Date.DayNumber - previous.Date.DayNumber <= RepairableGapDays)
                continue;

            var missingFrom = previous.Date.AddDays(1);
            var missingTo = next.Date.AddDays(-1);
            if (requestedFrom.HasValue && missingTo < requestedFrom.Value)
                continue;
            if (missingFrom > requestedTo)
                continue;

            if (requestedFrom.HasValue && missingFrom < requestedFrom.Value)
                missingFrom = requestedFrom.Value;
            if (missingTo > requestedTo)
                missingTo = requestedTo;

            var fetchResult = await FetchAndStoreRangeAsync(
                fund,
                missingFrom,
                missingTo,
                stopAtFirstOlderEmptyChunk: false,
                ct);
            checkedApi |= fetchResult.WasChecked;
            fetched |= fetchResult.HadNewData;
        }

        return new NavFetchResult(checkedApi, fetched);
    }

    private async Task<NavFetchResult> FetchAndStoreRangeAsync(
        Fund fund,
        DateOnly from,
        DateOnly to,
        bool stopAtFirstOlderEmptyChunk,
        CancellationToken ct)
    {
        if (from > to)
            return NavFetchResult.NotChecked;

        logger.LogInformation(
            "Fund {SchemeCode}: fetching NAV history from AMFI from {From} to {To}",
            fund.SchemeCode,
            from,
            to);

        var apiResponse = await amfiService.FetchNavHistoryAsync(
            fund.SchemeCode,
            from,
            to,
            ct,
            stopAtFirstOlderEmptyChunk);
        if (apiResponse is null)
            return NavFetchResult.NotChecked;

        if (apiResponse.Data.Count == 0)
            return new NavFetchResult(WasChecked: true, HadNewData: false);

        var entries = new List<NavHistory>();
        foreach (var entry in apiResponse.Data)
        {
            if (!DateOnly.TryParseExact(entry.Date, "yyyy-MM-dd", CultureInfo.InvariantCulture,
                    DateTimeStyles.None, out var date))
                continue;

            if (date < from || date > to || entry.Nav <= 0)
                continue;

            entries.Add(new NavHistory
            {
                SchemeCode = fund.SchemeCode,
                Date = date,
                Nav = entry.Nav
            });
        }

        if (entries.Count == 0)
            return new NavFetchResult(WasChecked: true, HadNewData: false);

        await navRepo.AddRangeAsync(entries);
        await corporateActionService.AutoRecordAnomaliesAsync(specificSchemeCode: fund.SchemeCode, ct: ct);
        logger.LogInformation(
            "Fund {SchemeCode}: stored up to {Count} NAV entries from AMFI",
            fund.SchemeCode,
            entries.Count);
        return new NavFetchResult(WasChecked: true, HadNewData: true);
    }

    private static DateOnly? GetRequestedFetchStart(NavHistoryScope scope, Fund fund)
    {
        if (scope.AllHistory)
            return fund.LaunchDate ?? HistoricalFetchStartDate;

        return scope.From;
    }

    private static DateOnly GetRequestedFetchEnd(NavHistoryScope scope, DateOnly syncTargetDate)
    {
        if (!scope.To.HasValue)
            return syncTargetDate;

        return scope.To.Value <= syncTargetDate ? scope.To.Value : syncTargetDate;
    }

    private static DateOnly GetSyncTargetDate(NavSyncTracker? tracker)
    {
        var today = IndiaTime.Today();
        return tracker?.LatestNavDate is { } latestNavDate && latestNavDate < today
            ? latestNavDate
            : today;
    }

    private static bool ShouldCallApi(NavSyncTracker? tracker, DateOnly targetDate)
    {
        if (tracker is null)
            return true;

        if (tracker.LastCheckHadNewData && tracker.NavEndDate.HasValue
            && tracker.NavEndDate.Value >= targetDate)
            return false;

        if (!tracker.LastCheckHadNewData && tracker.LastHistoryCheckAtIst.HasValue)
        {
            var elapsed = IndiaTime.Now() - tracker.LastHistoryCheckAtIst.Value;
            if (elapsed < ApiCooldown)
                return false;
        }

        return true;
    }

    private sealed record NavFetchResult(bool WasChecked, bool HadNewData)
    {
        public static readonly NavFetchResult NotChecked = new(WasChecked: false, HadNewData: false);
    }
}


