using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Repositories;

public interface INavHistoryRepository
{
    Task<List<NavHistory>> GetBySchemeCodeAsync(int schemeCode, DateOnly? from = null, DateOnly? to = null);
    Task<(DateOnly? StartDate, DateOnly? EndDate)> GetDateRangeAsync(int schemeCode);
    Task<(DateOnly? StartDate, DateOnly? EndDate, int Count)> GetStatsAsync(int schemeCode);
    Task<DateOnly?> GetLatestNavDateAsync(int schemeCode);
    Task<NavHistory?> GetLatestNavEntryAsync(int schemeCode);
    Task AddRangeAsync(List<NavHistory> entries);
    Task<NavSyncTracker?> GetSyncTrackerAsync(int schemeCode);
    Task UpsertLatestNavSnapshotsAsync(List<AmfiLatestNavSnapshot> snapshots, DateTime checkedAtIst, CancellationToken ct = default);
    Task UpdateTrackerHistoryStatsAsync(int schemeCode, DateOnly? startDate, DateOnly? endDate, int count, DateTime checkedAtIst, bool hadNewData, CancellationToken ct = default);
    Task UpdateTrackerCorporateActionCountAsync(int schemeCode, int count, DateTime updatedAtIst, CancellationToken ct = default);
}
