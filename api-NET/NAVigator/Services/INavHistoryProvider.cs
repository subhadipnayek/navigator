using NAVigator.Models.Database;

namespace NAVigator.Services;

public interface INavHistoryProvider
{
    Task<NavHistorySnapshot?> GetHistoryAsync(
        int schemeCode,
        NavHistoryScope scope,
        CancellationToken ct = default);

    Task<Dictionary<int, NavHistorySnapshot>> GetHistoriesAsync(
        IEnumerable<int> schemeCodes,
        NavHistoryScope scope,
        CancellationToken ct = default);

    Task<NavSyncTracker?> RefreshLatestNavTrackerIfStaleAsync(
        int schemeCode,
        CancellationToken ct = default);
}

public sealed record NavHistoryScope(
    DateOnly? From = null,
    DateOnly? To = null,
    bool AllHistory = false);

public sealed record NavHistorySnapshot(
    Fund Fund,
    NavSyncTracker? Tracker,
    List<NavHistory> Rows,
    bool WasFetchedFromApi);
