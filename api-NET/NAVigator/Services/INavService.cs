using NAVigator.Models.Dto;
using NAVigator.Models.Database;

namespace NAVigator.Services;

public interface INavService
{
    /// <summary>
    /// Returns NAV history for the given scheme. Uses smart caching:
    /// 1. Serve from DB if latest NAV is today.
    /// 2. If stale, check cooldown — skip API if checked recently with no new data.
    /// 3. Otherwise fetch from external API, store new entries, and update tracker.
    /// Supports optional date range filtering, interval-based sampling,
    /// and split-adjusted NAVs when <paramref name="adjusted"/> is true.
    /// </summary>
    Task<NavDataResponse?> GetNavHistoryAsync(int schemeCode, DateOnly? from = null, DateOnly? to = null, string? interval = null, bool adjusted = false, CancellationToken ct = default);

    /// <summary>
    /// Central lazy NAV history access point. Ensures each requested fund has
    /// history loaded/up-to-date, then returns stored DB history for callers that
    /// need raw rows for calculations.
    /// </summary>
    Task<Dictionary<int, List<NavHistory>>> EnsureNavHistoriesAsync(IEnumerable<int> schemeCodes, DateOnly? from = null, DateOnly? to = null, CancellationToken ct = default);

    Task<List<NavGapDto>?> DetectNavGapsAsync(int schemeCode, int minGapDays = 30, CancellationToken ct = default);

    Task<List<NavGapDto>> DetectAllNavGapsAsync(int minGapDays = 30, CancellationToken ct = default);

    Task<NavGapRepairResultDto?> RepairNavGapsAsync(int schemeCode, int minGapDays = 30, CancellationToken ct = default);

    /// <summary>
    /// Computes point-to-point returns (absolute + CAGR) between the specified dates.
    /// Ensures NAV data is up-to-date before calculating.
    /// </summary>
    Task<ReturnsResponse?> GetReturnsAsync(int schemeCode, DateOnly startDate, DateOnly endDate, CancellationToken ct = default);

    /// <summary>
    /// Computes rolling returns (annualized CAGR) for every possible window of the
    /// given period length across the fund's NAV history.
    /// Also returns summary stats: min, max, average, median.
    /// Supports optional date range filtering and interval-based sampling.
    /// </summary>
    Task<RollingReturnResponse?> GetRollingReturnsAsync(int schemeCode, int periodInYears, DateOnly? from = null, DateOnly? to = null, string? interval = null, CancellationToken ct = default);
}
