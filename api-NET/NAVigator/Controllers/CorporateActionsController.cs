using System.Globalization;
using Microsoft.AspNetCore.Mvc;
using Microsoft.AspNetCore.Authorization;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Dto;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
[Authorize(Roles = "Admin")]
public class CorporateActionsController(
    ICorporateActionService actionService,
    INavService navService,
    AppDbContext db) : ControllerBase
{
    private static readonly string[] ValidActionTypes = ["Split", "Merger", "Bonus", "Discontinuity", "MissingData", "Other"];
    private static readonly string[] ValidReviewFilters = ["all", "factor-actions", "gaps", "missing-data", "large-moves", "recorded", "needs-review"];
    private static readonly string[] ValidPriorities = ["all", "high", "medium", "low"];

    /// <summary>
    /// Detect NAV anomalies across all funds. Returns large day-over-day NAV changes
    /// that may indicate splits, mergers, bonuses, or data discontinuities.
    /// </summary>
    [HttpGet("anomalies")]
    public async Task<IActionResult> DetectAnomalies(
        [FromQuery] decimal threshold = 30m,
        [FromQuery] int minDaysSincePrevious = 1,
        CancellationToken ct = default)
    {
        if (threshold is < 5 or > 99)
            return BadRequest(new { message = "threshold must be between 5 and 99." });

        if (minDaysSincePrevious is < 1 or > 3650)
            return BadRequest(new { message = "minDaysSincePrevious must be between 1 and 3650." });

        var result = await actionService.DetectAnomaliesAsync(threshold, minDaysSincePrevious, ct: ct);
        return Ok(result);
    }

    /// <summary>
    /// Detect the admin review queue in one request, with movement and NAV-gap filters
    /// applied consistently before the UI renders rows.
    /// </summary>
    [HttpGet("review-queue")]
    public async Task<IActionResult> GetReviewQueue(
        [FromQuery] decimal threshold = 30m,
        [FromQuery] int minGapDays = 1,
        [FromQuery] string filter = "all",
        [FromQuery] bool includeRecorded = true,
        [FromQuery] int? schemeCode = null,
        [FromQuery] string? q = null,
        [FromQuery] string? actionType = null,
        [FromQuery] string priority = "all",
        [FromQuery] decimal? minConfidence = null,
        [FromQuery] decimal minAutoRecordConfidence = 0.9m,
        [FromQuery] int limit = 500,
        CancellationToken ct = default)
    {
        if (threshold is < 5 or > 99)
            return BadRequest(new { message = "threshold must be between 5 and 99." });

        if (minGapDays is < 1 or > 3650)
            return BadRequest(new { message = "minGapDays must be between 1 and 3650." });

        if (!ValidReviewFilters.Contains(filter, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "filter must be a valid review queue filter." });

        if (!ValidPriorities.Contains(priority, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "priority must be 'all', 'high', 'medium', or 'low'." });

        if (minConfidence is < 0m or > 1m)
            return BadRequest(new { message = "minConfidence must be between 0 and 1." });

        if (minAutoRecordConfidence is < 0.5m or > 1m)
            return BadRequest(new { message = "minAutoRecordConfidence must be between 0.5 and 1." });

        if (limit is < 1 or > 1000)
            return BadRequest(new { message = "limit must be between 1 and 1000." });

        if (actionType is not null && !ValidActionTypes.Contains(actionType, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "actionType must be 'Split', 'Merger', 'Bonus', 'Discontinuity', 'MissingData', or 'Other'." });

        var normalizedFilter = filter.ToLowerInvariant();
        var includeMovements = normalizedFilter is not "gaps";
        var includeGaps = normalizedFilter is "all" or "gaps" or "missing-data" or "needs-review";

        var anomalies = includeMovements
            ? await actionService.DetectAnomaliesAsync(threshold, minGapDays, ct: ct)
            : new List<NavAnomalyDto>();
        var gaps = includeGaps
            ? await navService.DetectAllNavGapsAsync(Math.Max(minGapDays, 7), ct)
            : new List<NavGapDto>();

        if (schemeCode.HasValue)
        {
            anomalies = anomalies.Where(a => a.SchemeCode == schemeCode.Value).ToList();
            gaps = gaps.Where(g => g.SchemeCode == schemeCode.Value).ToList();
        }

        var relevantSchemeCodes = anomalies
            .Select(a => a.SchemeCode)
            .Concat(gaps.Select(g => g.SchemeCode))
            .Distinct()
            .ToList();
        var actionDates = relevantSchemeCodes.Count == 0
            ? new HashSet<(int SchemeCode, string Date)>()
            : (await db.CorporateActions
                .AsNoTracking()
                .Where(a => relevantSchemeCodes.Contains(a.SchemeCode))
                .Select(a => new { a.SchemeCode, a.ActionDate })
                .ToListAsync(ct))
                .Select(a => (a.SchemeCode, FormatDate(a.ActionDate)))
                .ToHashSet();

        var rows = BuildReviewRows(anomalies, gaps, actionDates);

        rows = rows
            .Where(r => MatchesReviewFilter(r, normalizedFilter))
            .Where(r => includeRecorded || !r.HasCorporateAction)
            .Where(r => MatchesSearch(r, q))
            .Where(r => actionType is null || r.SuggestionTitle.Equals(actionType, StringComparison.OrdinalIgnoreCase))
            .Where(r => priority.Equals("all", StringComparison.OrdinalIgnoreCase) || r.Priority.Equals(priority, StringComparison.OrdinalIgnoreCase))
            .Where(r => !minConfidence.HasValue || ParseConfidence(r.Confidence) >= minConfidence.Value)
            .OrderByDescending(r => r.SortMagnitude)
            .ThenBy(r => r.SchemeName)
            .Take(limit)
            .ToList();

        return Ok(new CorporateActionReviewQueueDto
        {
            Rows = rows,
            Summary = BuildSummary(rows, minAutoRecordConfidence),
            Anomalies = rows.Where(r => r.Anomaly is not null).Select(r => r.Anomaly!).ToList(),
            Gaps = rows.Where(r => r.Gap is not null).Select(r => r.Gap!).ToList()
        });
    }

    /// <summary>
    /// Records high-confidence anomaly suggestions that are safe to automate.
    /// Currently limited to data discontinuities; split-like changes still require review.
    /// </summary>
    [HttpPost("anomalies/auto-record")]
    public async Task<IActionResult> AutoRecordAnomalies(
        [FromQuery] decimal threshold = 30m,
        [FromQuery] decimal minConfidence = 0.9m,
        CancellationToken ct = default)
    {
        if (threshold is < 5 or > 99)
            return BadRequest(new { message = "threshold must be between 5 and 99." });

        if (minConfidence is < 0.5m or > 1m)
            return BadRequest(new { message = "minConfidence must be between 0.5 and 1." });

        var result = await actionService.AutoRecordAnomaliesAsync(threshold, minConfidence, ct: ct);
        return Ok(result);
    }

    private static bool MatchesReviewFilter(CorporateActionReviewRowDto row, string filter)
    {
        return filter switch
        {
            "factor-actions" => row.HasMovement && row.SuggestionTitle is "Split" or "Merger" or "Bonus",
            "gaps" => row.HasGap,
            "missing-data" => row.SuggestionTitle == "MissingData" || row.HasGap,
            "large-moves" => row.HasMovement && row.SuggestionTitle == "Other",
            "recorded" => row.HasCorporateAction,
            "needs-review" => row.RequiresPreReview && !row.HasCorporateAction,
            _ => true
        };
    }

    private static bool MatchesSearch(CorporateActionReviewRowDto row, string? query)
    {
        if (string.IsNullOrWhiteSpace(query))
            return true;

        var search = query.Trim();
        return row.SchemeCode.ToString() == search
               || row.SchemeName.Contains(search, StringComparison.OrdinalIgnoreCase)
               || (row.FundHouse?.Contains(search, StringComparison.OrdinalIgnoreCase) ?? false);
    }

    private static List<CorporateActionReviewRowDto> BuildReviewRows(
        List<NavAnomalyDto> anomalies,
        List<NavGapDto> gaps,
        HashSet<(int SchemeCode, string Date)> actionDates)
    {
        var rows = new Dictionary<(int SchemeCode, string Date), CorporateActionReviewRowDto>();

        foreach (var gap in gaps)
        {
            var key = (gap.SchemeCode, gap.NextDate);
            var hasAction = actionDates.Contains(key);
            rows[key] = new CorporateActionReviewRowDto
            {
                Kind = "Gap",
                SchemeCode = gap.SchemeCode,
                SchemeName = gap.SchemeName,
                FundHouse = gap.FundHouse,
                Date = gap.NextDate,
                PreviousDate = gap.PreviousDate,
                DaysSincePrevious = gap.GapDays,
                Title = $"{gap.GapDays}d gap",
                Detail = $"{gap.MissingFrom} to {gap.MissingTo}",
                NavDetail = $"{gap.MissingCalendarDays} missing calendar days",
                SuggestionTitle = hasAction ? "MissingData" : "NAV Gap",
                SuggestionDetail = hasAction
                    ? "A MissingData/Discontinuity-style action is already recorded. Sync NAV if the source now has the missing range."
                    : "Repair from AMFI first when possible; otherwise review as MissingData.",
                Priority = "High",
                Confidence = hasAction ? "Recorded" : "Detected",
                HasCorporateAction = hasAction,
                IsAutoRecordable = false,
                RequiresPreReview = !hasAction,
                SortMagnitude = gap.GapDays,
                HasGap = true,
                CanRepairGap = !hasAction,
                CanSyncNav = true,
                Gap = gap
            };
        }

        foreach (var anomaly in anomalies)
        {
            var key = (anomaly.SchemeCode, anomaly.Date);
            if (rows.TryGetValue(key, out var existing))
            {
                existing.Kind = "Gap";
                existing.HasMovement = true;
                existing.Anomaly = anomaly;
                existing.HasCorporateAction = existing.HasCorporateAction || anomaly.HasCorporateAction;
                existing.IsAutoRecordable = anomaly.IsAutoRecordable && !existing.HasCorporateAction;
                existing.RequiresPreReview = !existing.HasCorporateAction && (existing.RequiresPreReview || anomaly.RequiresPreReview);
                existing.SuggestionTitle = anomaly.SuggestedActionType ?? existing.SuggestionTitle;
                existing.SuggestionDetail = anomaly.SuggestedNotes ?? existing.SuggestionDetail;
                existing.Confidence = existing.HasCorporateAction ? "Recorded" : $"{Math.Round(anomaly.Confidence * 100m)}%";
                existing.NavDetail = $"{existing.NavDetail}; NAV {anomaly.PreviousNav:0.####} to {anomaly.CurrentNav:0.####} ({anomaly.ChangePercent:0.##}% {anomaly.Direction.ToLowerInvariant()})";
                existing.Priority = MaxPriority(existing.Priority, anomaly.ReviewPriority);
                existing.SortMagnitude = Math.Max(existing.SortMagnitude, Math.Abs(anomaly.ChangePercent));
                existing.CanRepairGap = existing.CanRepairGap && !existing.HasCorporateAction;
                existing.CanSyncNav = true;
                continue;
            }

            rows[key] = new CorporateActionReviewRowDto
            {
                Kind = "Movement",
                SchemeCode = anomaly.SchemeCode,
                SchemeName = anomaly.SchemeName,
                FundHouse = anomaly.FundHouse,
                Date = anomaly.Date,
                PreviousDate = anomaly.PreviousDate,
                DaysSincePrevious = anomaly.DaysSincePrevious,
                Title = $"{anomaly.ChangePercent:0.##}% {anomaly.Direction}",
                Detail = $"{anomaly.PreviousDate} to {anomaly.Date} - {anomaly.DaysSincePrevious}d",
                NavDetail = $"{anomaly.PreviousNav:0.####} to {anomaly.CurrentNav:0.####}",
                SuggestionTitle = anomaly.SuggestedActionType ?? "Review",
                SuggestionDetail = anomaly.SuggestedNotes ?? "Manual review needed.",
                Priority = anomaly.ReviewPriority,
                Confidence = $"{Math.Round(anomaly.Confidence * 100m)}%",
                HasCorporateAction = anomaly.HasCorporateAction,
                IsAutoRecordable = anomaly.IsAutoRecordable,
                RequiresPreReview = anomaly.RequiresPreReview,
                SortMagnitude = Math.Abs(anomaly.ChangePercent),
                HasMovement = true,
                CanSyncNav = anomaly.SuggestedActionType == "MissingData",
                Anomaly = anomaly
            };
        }

        return rows.Values.ToList();
    }

    private static CorporateActionReviewSummaryDto BuildSummary(
        List<CorporateActionReviewRowDto> rows,
        decimal minAutoRecordConfidence)
    {
        return new CorporateActionReviewSummaryDto
        {
            PendingVisible = rows.Count(r => !r.HasCorporateAction),
            NavGaps = rows.Count(r => r.HasGap),
            AutoRecordable = rows.Count(r => r.IsAutoRecordable
                                             && !r.HasCorporateAction
                                             && ParseConfidence(r.Confidence) >= minAutoRecordConfidence),
            HighPriority = rows.Count(r => r.Priority == "High" && !r.HasCorporateAction),
            PreReview = rows.Count(r => r.RequiresPreReview && !r.HasCorporateAction),
            RecordedItems = rows.Count(r => r.HasCorporateAction)
        };
    }

    private static string MaxPriority(string left, string right)
    {
        return PriorityRank(right) > PriorityRank(left) ? right : left;
    }

    private static int PriorityRank(string priority)
    {
        return priority switch
        {
            "High" => 3,
            "Medium" => 2,
            "Low" => 1,
            _ => 0
        };
    }

    private static decimal ParseConfidence(string confidence)
    {
        return confidence.EndsWith('%') && decimal.TryParse(confidence.TrimEnd('%'), out var percent)
            ? percent / 100m
            : 0m;
    }

    private static string FormatDate(DateOnly date)
    {
        return date.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture);
    }

    /// <summary>
    /// Get all corporate actions for a fund.
    /// </summary>
    [HttpGet("{schemeCode:int}")]
    public async Task<IActionResult> GetActions(int schemeCode)
    {
        var actions = await actionService.GetActionsAsync(schemeCode);
        return Ok(actions);
    }

    /// <summary>
    /// Add a corporate action (split, merger, bonus, etc.) for a fund.
    /// </summary>
    [HttpPost("{schemeCode:int}")]
    public async Task<IActionResult> AddAction(int schemeCode, [FromBody] CreateCorporateActionRequest request)
    {
        if (!ValidActionTypes.Contains(request.ActionType, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "actionType must be 'Split', 'Merger', 'Bonus', 'Discontinuity', 'MissingData', or 'Other'." });

        var result = await actionService.AddActionAsync(schemeCode, request);
        return result is null
            ? NotFound(new { message = $"Fund with scheme code {schemeCode} not found." })
            : Created($"/api/corporateactions/{schemeCode}", result);
    }

    /// <summary>
    /// Update a corporate action after post-record review.
    /// </summary>
    [HttpPut("{schemeCode:int}/{actionId:int}")]
    public async Task<IActionResult> UpdateAction(
        int schemeCode,
        int actionId,
        [FromBody] UpdateCorporateActionRequest request)
    {
        if (!ValidActionTypes.Contains(request.ActionType, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "actionType must be 'Split', 'Merger', 'Bonus', 'Discontinuity', 'MissingData', or 'Other'." });

        var result = await actionService.UpdateActionAsync(schemeCode, actionId, request);
        return result is null
            ? NotFound(new { message = $"Corporate action {actionId} not found for scheme {schemeCode}." })
            : Ok(result);
    }

    /// <summary>
    /// Delete a corporate action by id.
    /// </summary>
    [HttpDelete("{schemeCode:int}/{actionId:int}")]
    public async Task<IActionResult> DeleteAction(int schemeCode, int actionId)
    {
        var deleted = await actionService.DeleteActionAsync(schemeCode, actionId);
        return deleted
            ? NoContent()
            : NotFound(new { message = $"Corporate action {actionId} not found for scheme {schemeCode}." });
    }
}
