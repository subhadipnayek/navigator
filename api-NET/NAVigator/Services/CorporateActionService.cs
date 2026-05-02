using System.Globalization;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class CorporateActionService(
    AppDbContext db,
    ICorporateActionRepository actionRepo,
    IFundRepository fundRepo,
    ILogger<CorporateActionService> logger) : ICorporateActionService
{
    private static readonly string[] ValidActionTypes = ["Split", "Merger", "Bonus", "Discontinuity", "MissingData", "Other"];
    private static readonly decimal[] CommonAdjustmentFactors = [2m, 3m, 4m, 5m, 10m, 100m];
    private const string AutoRecordedPrefix = "[Auto-recorded]";

    public async Task<List<NavAnomalyDto>> DetectAnomaliesAsync(decimal thresholdPercent = 30m, int minDaysSincePrevious = 1, CancellationToken ct = default)
    {
        logger.LogInformation(
            "Detecting NAV anomalies with threshold {Threshold}% and minimum NAV point spacing {MinDays} day(s)",
            thresholdPercent,
            minDaysSincePrevious);

        // Query all funds that have at least 2 NAV entries, looking for large day-over-day swings.
        // Use a subquery for the filter (EF Core can't translate GroupBy+SelectMany with ordering).
        var schemesWithMultipleNavs = db.NavHistories
            .GroupBy(n => n.SchemeCode)
            .Where(g => g.Count() > 1)
            .Select(g => g.Key);

        var anomalies = await db.NavHistories
            .AsNoTracking()
            .Where(n => schemesWithMultipleNavs.Contains(n.SchemeCode))
            .OrderBy(n => n.SchemeCode)
            .ThenBy(n => n.Date)
            .Select(n => new { n.SchemeCode, n.Date, n.Nav })
            .ToListAsync(ct);

        // Process in memory — group by scheme, detect day-over-day anomalies
        var grouped = anomalies
            .GroupBy(n => n.SchemeCode)
            .ToDictionary(g => g.Key, g => g.OrderBy(n => n.Date).ToList());

        // Get all corporate action dates for quick lookup
        var allActions = await db.CorporateActions
            .AsNoTracking()
            .Select(ca => new { ca.SchemeCode, ca.ActionDate })
            .ToListAsync(ct);
        var actionLookup = allActions
            .ToLookup(a => a.SchemeCode, a => a.ActionDate);

        var result = new List<NavAnomalyDto>();

        // Load fund names in bulk
        var schemeCodes = grouped.Keys.ToList();
        var funds = await db.Set<Fund>()
            .AsNoTracking()
            .Where(f => schemeCodes.Contains(f.SchemeCode))
            .Select(f => new { f.SchemeCode, f.SchemeName, f.FundHouse })
            .ToDictionaryAsync(f => f.SchemeCode, ct);

        foreach (var (schemeCode, navEntries) in grouped)
        {
            var actionDates = actionLookup[schemeCode].ToHashSet();

            for (int i = 1; i < navEntries.Count; i++)
            {
                var prev = navEntries[i - 1];
                var curr = navEntries[i];

                if (prev.Nav <= 0) continue;

                var daysSincePrevious = curr.Date.DayNumber - prev.Date.DayNumber;
                if (daysSincePrevious < minDaysSincePrevious) continue;

                var changePercent = (curr.Nav - prev.Nav) / prev.Nav * 100m;

                if (Math.Abs(changePercent) >= thresholdPercent)
                {
                    funds.TryGetValue(schemeCode, out var fund);
                    var classification = ClassifyAnomaly(prev.Nav, curr.Nav, changePercent, daysSincePrevious);

                    result.Add(new NavAnomalyDto
                    {
                        SchemeCode = schemeCode,
                        SchemeName = fund?.SchemeName ?? "Unknown",
                        FundHouse = fund?.FundHouse,
                        Date = curr.Date.ToString("dd-MM-yyyy"),
                        PreviousDate = prev.Date.ToString("dd-MM-yyyy"),
                        DaysSincePrevious = daysSincePrevious,
                        PreviousNav = prev.Nav,
                        CurrentNav = curr.Nav,
                        ChangePercent = Math.Round(changePercent, 2),
                        Direction = changePercent < 0 ? "Drop" : "Spike",
                        HasCorporateAction = actionDates.Contains(curr.Date),
                        SuggestedActionType = classification.ActionType,
                        SuggestedAdjustmentFactor = classification.AdjustmentFactor,
                        SuggestedNotes = classification.Notes,
                        Confidence = classification.Confidence,
                        ReviewPriority = classification.ReviewPriority,
                        IsAutoRecordable = classification.IsAutoRecordable && !actionDates.Contains(curr.Date),
                        RequiresPreReview = classification.RequiresPreReview && !actionDates.Contains(curr.Date)
                    });
                }
            }
        }

        logger.LogInformation("Found {Count} anomalies across all funds", result.Count);

        return result
            .OrderByDescending(a => Math.Abs(a.ChangePercent))
            .ToList();
    }

    public async Task<AnomalyAutoRecordResultDto> AutoRecordAnomaliesAsync(
        decimal thresholdPercent = 30m,
        decimal minConfidence = 0.9m,
        CancellationToken ct = default)
    {
        var anomalies = await DetectAnomaliesAsync(thresholdPercent, ct: ct);
        var eligible = anomalies
            .Where(a => !a.HasCorporateAction
                        && a.IsAutoRecordable
                        && a.Confidence >= minConfidence
                        && a.SuggestedActionType is not null
                        && a.SuggestedAdjustmentFactor is not null)
            .ToList();

        var recordedActions = new List<CorporateActionDto>();

        foreach (var anomaly in eligible)
        {
            if (!DateOnly.TryParseExact(anomaly.Date, "dd-MM-yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None, out var actionDate))
                continue;

            var created = await AddActionAsync(anomaly.SchemeCode, new CreateCorporateActionRequest
            {
                ActionDate = actionDate,
                ActionType = anomaly.SuggestedActionType!,
                AdjustmentFactor = anomaly.SuggestedAdjustmentFactor!.Value,
                Notes = $"{AutoRecordedPrefix} {anomaly.SuggestedNotes}"
            });

            if (created is not null)
                recordedActions.Add(created);
        }

        logger.LogInformation(
            "Auto-recorded {RecordedCount} corporate actions from {EligibleCount} eligible anomalies",
            recordedActions.Count,
            eligible.Count);

        return new AnomalyAutoRecordResultDto
        {
            DetectedAnomalies = anomalies.Count,
            EligibleAnomalies = eligible.Count,
            RecordedActions = recordedActions.Count,
            Actions = recordedActions
        };
    }

    public async Task<List<CorporateActionDto>> GetActionsAsync(int schemeCode)
    {
        var actions = await actionRepo.GetBySchemeCodeAsync(schemeCode);

        return actions.Select(a => new CorporateActionDto
        {
            Id = a.Id,
            SchemeCode = a.SchemeCode,
            ActionDate = a.ActionDate.ToString("dd-MM-yyyy"),
            ActionType = a.ActionType,
            AdjustmentFactor = a.AdjustmentFactor,
            Notes = a.Notes
        }).ToList();
    }

    public async Task<CorporateActionDto?> AddActionAsync(int schemeCode, CreateCorporateActionRequest request)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
            return null;

        if (!ValidActionTypes.Contains(request.ActionType, StringComparer.OrdinalIgnoreCase))
            return null;

        var actionType = NormalizeActionType(request.ActionType);
        var adjustmentFactor = IsSeriesBreakAction(actionType) ? 1m : request.AdjustmentFactor;

        var action = new CorporateAction
        {
            SchemeCode = schemeCode,
            ActionDate = request.ActionDate,
            ActionType = actionType,
            AdjustmentFactor = adjustmentFactor,
            Notes = request.Notes
        };

        await actionRepo.AddAsync(action);

        logger.LogInformation(
            "Added corporate action for {SchemeCode}: {ActionType} on {Date} with factor {Factor}",
            schemeCode, actionType, request.ActionDate, adjustmentFactor);

        return ToDto(action);
    }

    public async Task<CorporateActionDto?> UpdateActionAsync(int schemeCode, int actionId, UpdateCorporateActionRequest request)
    {
        var action = await db.CorporateActions.FirstOrDefaultAsync(a => a.Id == actionId && a.SchemeCode == schemeCode);
        if (action is null)
            return null;

        if (!ValidActionTypes.Contains(request.ActionType, StringComparer.OrdinalIgnoreCase))
            return null;

        var actionType = NormalizeActionType(request.ActionType);
        var adjustmentFactor = IsSeriesBreakAction(actionType) ? 1m : request.AdjustmentFactor;

        action.ActionDate = request.ActionDate;
        action.ActionType = actionType;
        action.AdjustmentFactor = adjustmentFactor;
        action.OldFaceValue = null;
        action.NewFaceValue = null;
        action.Notes = request.Notes;

        await db.SaveChangesAsync();

        logger.LogInformation(
            "Updated corporate action {ActionId} for {SchemeCode}: {ActionType} on {Date} with factor {Factor}",
            actionId, schemeCode, actionType, request.ActionDate, adjustmentFactor);

        return ToDto(action);
    }

    public async Task<bool> DeleteActionAsync(int schemeCode, int actionId)
    {
        var action = await actionRepo.GetByIdAsync(actionId);
        if (action is null || action.SchemeCode != schemeCode)
            return false;

        await actionRepo.DeleteAsync(action);

        logger.LogInformation("Deleted corporate action {Id} for scheme {SchemeCode}", actionId, schemeCode);
        return true;
    }

    public async Task<decimal> GetCumulativeAdjustmentFactorAsync(int schemeCode, DateOnly fromDate, DateOnly toDate)
    {
        var actions = await actionRepo.GetBySchemeCodeAsync(schemeCode);

        // Multiply adjustment factors for all actions between fromDate (exclusive) and toDate (inclusive).
        // This gives the cumulative factor to adjust the fromDate NAV to be comparable with toDate NAV.
        decimal cumulativeFactor = 1m;

        foreach (var action in actions)
        {
            if (action.ActionDate > fromDate && action.ActionDate <= toDate
                && !IsSeriesBreakAction(action.ActionType))
            {
                cumulativeFactor *= action.AdjustmentFactor;
            }
        }

        return cumulativeFactor;
    }

    /// <summary>
    /// Checks whether any Discontinuity corporate action exists between two dates for a fund.
    /// Used to determine if returns across this range are unreliable.
    /// </summary>
    public async Task<DateOnly?> GetDiscontinuityDateAsync(int schemeCode, DateOnly fromDate, DateOnly toDate)
    {
        var actions = await actionRepo.GetBySchemeCodeAsync(schemeCode);

        // Return the latest discontinuity date in the range (so callers can clamp to post-discontinuity)
        return actions
            .Where(a => IsSeriesBreakAction(a.ActionType)
                        && a.ActionDate > fromDate && a.ActionDate <= toDate)
            .OrderByDescending(a => a.ActionDate)
            .Select(a => (DateOnly?)a.ActionDate)
            .FirstOrDefault();
    }

    private static AnomalyClassification ClassifyAnomaly(decimal previousNav, decimal currentNav, decimal changePercent, int daysSincePrevious)
    {
        var isSuspiciousGap = daysSincePrevious >= 8;
        var ratio = changePercent < 0 ? previousNav / currentNav : currentNav / previousNav;
        var roundedRatio = Math.Round(ratio, 4);
        var nearestFactor = FindNearestCommonFactor(ratio);

        if (nearestFactor.HasValue)
        {
            var relativeDifference = Math.Abs(ratio - nearestFactor.Value) / nearestFactor.Value;
            var isNearExact = relativeDifference <= 0.01m;
            var adjustmentFactor = changePercent < 0
                ? nearestFactor.Value
                : Math.Round(1m / nearestFactor.Value, 8);

            return new AnomalyClassification(
                "Split",
                adjustmentFactor,
                isNearExact
                    ? $"NAV ratio matches a {nearestFactor.Value:0.####}x action over {daysSincePrevious} day(s). Auto-recorded actions still need post-record review."
                    : $"NAV ratio is close to {nearestFactor.Value:0.####}x over {daysSincePrevious} day(s). Review before recording.",
                isNearExact && !isSuspiciousGap ? 0.98m : 0.84m,
                "High",
                isNearExact && !isSuspiciousGap,
                isSuspiciousGap);
        }

        if (isSuspiciousGap)
        {
            return new AnomalyClassification(
                "MissingData",
                1m,
                $"{daysSincePrevious}-day data gap with a non-standard NAV jump. Try repairing the gap, or mark MissingData so returns do not cross it.",
                0.78m,
                "High",
                false,
                true);
        }

        return new AnomalyClassification(
            "Other",
            Math.Max(roundedRatio, 0.0001m),
            $"Unusual {Math.Abs(changePercent):0.##}% {ChangeLabel(changePercent)} over {daysSincePrevious} day(s); manual review needed.",
            0.55m,
            Math.Abs(changePercent) >= 60 ? "High" : "Medium",
            false,
            true);
    }

    private static decimal? FindNearestCommonFactor(decimal ratio)
    {
        foreach (var factor in CommonAdjustmentFactors)
        {
            var relativeDifference = Math.Abs(ratio - factor) / factor;
            if (relativeDifference <= 0.08m)
                return factor;
        }

        return null;
    }

    private static string ChangeLabel(decimal changePercent) => changePercent < 0 ? "drop" : "spike";

    private static bool IsSeriesBreakAction(string actionType)
    {
        return actionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase)
               || actionType.Equals("MissingData", StringComparison.OrdinalIgnoreCase);
    }

    private static string NormalizeActionType(string actionType)
    {
        return ValidActionTypes.FirstOrDefault(t => t.Equals(actionType, StringComparison.OrdinalIgnoreCase))
               ?? actionType;
    }

    private static CorporateActionDto ToDto(CorporateAction action)
    {
        return new CorporateActionDto
        {
            Id = action.Id,
            SchemeCode = action.SchemeCode,
            ActionDate = action.ActionDate.ToString("dd-MM-yyyy"),
            ActionType = action.ActionType,
            AdjustmentFactor = action.AdjustmentFactor,
            Notes = action.Notes
        };
    }

    private sealed record AnomalyClassification(
        string ActionType,
        decimal AdjustmentFactor,
        string Notes,
        decimal Confidence,
        string ReviewPriority,
        bool IsAutoRecordable,
        bool RequiresPreReview);
}
