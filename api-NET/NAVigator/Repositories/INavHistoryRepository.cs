using NAVigator.Models.Database;

namespace NAVigator.Repositories;

public interface INavHistoryRepository
{
    Task<List<NavHistory>> GetBySchemeCodeAsync(int schemeCode);
    Task<DateOnly?> GetLatestNavDateAsync(int schemeCode);
    Task<NavHistory?> GetLatestNavEntryAsync(int schemeCode);
    Task AddRangeAsync(List<NavHistory> entries);
    Task<NavSyncTracker?> GetSyncTrackerAsync(int schemeCode);
    Task UpsertSyncTrackerAsync(NavSyncTracker tracker);
}
