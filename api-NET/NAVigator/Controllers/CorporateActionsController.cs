using Microsoft.AspNetCore.Mvc;
using NAVigator.Models.Dto;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
public class CorporateActionsController(ICorporateActionService actionService) : ControllerBase
{
    private static readonly string[] ValidActionTypes = ["Split", "Merger", "Bonus", "Other"];

    /// <summary>
    /// Detect NAV anomalies across all funds. Returns large day-over-day NAV changes
    /// that may indicate splits, mergers, or bonuses.
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
            return BadRequest(new { message = "actionType must be 'Split', 'Merger', 'Bonus', or 'Other'." });

        var result = await actionService.AddActionAsync(schemeCode, request);
        return result is null
            ? NotFound(new { message = $"Fund with scheme code {schemeCode} not found." })
            : Created($"/api/corporateactions/{schemeCode}", result);
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
