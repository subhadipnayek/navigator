using NAVigator.Models.Dto;

namespace NAVigator.Services;

public interface ICorporateActionService
{
    /// <summary>
    /// Detects NAV anomalies (sudden large drops/spikes) across all funds that have NAV history.
    /// Flags whether each anomaly already has a corporate action recorded.
    /// </summary>
    Task<List<NavAnomalyDto>> DetectAnomaliesAsync(decimal thresholdPercent = 30m, CancellationToken ct = default);

    /// <summary>Gets all corporate actions for a fund.</summary>
    Task<List<CorporateActionDto>> GetActionsAsync(int schemeCode);

    /// <summary>Adds a new corporate action for a fund.</summary>
    Task<CorporateActionDto?> AddActionAsync(int schemeCode, CreateCorporateActionRequest request);

    /// <summary>Deletes a corporate action by id.</summary>
    Task<bool> DeleteActionAsync(int schemeCode, int actionId);

    /// <summary>
    /// Computes the cumulative adjustment factor for a fund between two dates.
    /// Used to normalize NAVs across splits/mergers when calculating returns.
    /// Discontinuity actions are excluded from the factor calculation.
    /// </summary>
    Task<decimal> GetCumulativeAdjustmentFactorAsync(int schemeCode, DateOnly fromDate, DateOnly toDate);

    /// <summary>
    /// Returns the latest Discontinuity action date between two dates for a fund, or null if none exists.
    /// Used to detect data gaps where returns across the range are unreliable.
    /// </summary>
    Task<DateOnly?> GetDiscontinuityDateAsync(int schemeCode, DateOnly fromDate, DateOnly toDate);
}
