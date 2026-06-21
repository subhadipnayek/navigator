using NAVigator.Models.Dto;
using NAVigator.Models.Database;

namespace NAVigator.Services;

public interface INavService
{
    /// <summary>
    /// Returns NAV history for the given scheme through the shared history provider:
    /// 1. Refresh latest NAV tracker state when stale.
    /// 2. Backfill requested history ranges and large internal gaps.
    /// 3. Serve consistent stored history with optional sampling and adjustment.
    /// </summary>
    Task<NavDataResponse?> GetNavHistoryAsync(int schemeCode, DateOnly? from = null, DateOnly? to = null, string? interval = null, bool adjusted = false, CancellationToken ct = default);

    /// <summary>
    /// Central lazy NAV history access point for calculation callers. Ensures each
    /// requested fund goes through the shared history provider before returning rows.
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
