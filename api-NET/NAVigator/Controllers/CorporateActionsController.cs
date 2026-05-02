using Microsoft.AspNetCore.Mvc;
using Microsoft.AspNetCore.Authorization;
using NAVigator.Models.Dto;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
[Authorize(Roles = "Admin")]
public class CorporateActionsController(ICorporateActionService actionService) : ControllerBase
{
    private static readonly string[] ValidActionTypes = ["Split", "Merger", "Bonus", "Discontinuity", "MissingData", "Other"];

    /// <summary>
    /// Detect NAV anomalies across all funds. Returns large day-over-day NAV changes
    /// that may indicate splits, mergers, bonuses, or data discontinuities.
    /// </summary>
    [HttpGet("anomalies")]
    public async Task<IActionResult> DetectAnomalies(
        [FromQuery] decimal threshold = 30m,
        CancellationToken ct = default)
    {
        if (threshold is < 5 or > 99)
            return BadRequest(new { message = "threshold must be between 5 and 99." });

        var result = await actionService.DetectAnomaliesAsync(threshold, ct);
        return Ok(result);
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

        var result = await actionService.AutoRecordAnomaliesAsync(threshold, minConfidence, ct);
        return Ok(result);
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
