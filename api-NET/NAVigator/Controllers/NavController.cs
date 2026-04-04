using Microsoft.AspNetCore.Mvc;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
public class NavController(INavService navService) : ControllerBase
{
    private static readonly string[] ValidIntervals = ["daily", "weekly", "monthly"];

    /// <summary>
    /// Get NAV history for a fund by scheme code.
    /// Supports optional date range filtering, interval-based sampling,
    /// and split-adjusted NAVs (adjusted=true divides pre-action NAVs by cumulative factor).
    /// </summary>
    [HttpGet("{schemeCode:int}")]
    public async Task<IActionResult> GetNavHistory(
        int schemeCode,
        [FromQuery] DateOnly? from = null,
        [FromQuery] DateOnly? to = null,
        [FromQuery] string? interval = null,
        [FromQuery] bool adjusted = false,
        CancellationToken ct = default)
    {
        if (from.HasValue && to.HasValue && from.Value >= to.Value)
            return BadRequest(new { message = "'from' must be before 'to'." });

        if (!ValidateInterval(interval, out var error))
            return BadRequest(new { message = error });

        var result = await navService.GetNavHistoryAsync(schemeCode, from, to, interval, adjusted, ct);
        return result is null
            ? NotFound(new { message = $"Fund with scheme code {schemeCode} not found." })
            : Ok(result);
    }

    /// <summary>
    /// Get point-to-point returns (absolute + CAGR) between the specified dates.
    /// </summary>
    [HttpGet("{schemeCode:int}/returns")]
    public async Task<IActionResult> GetReturns(
        int schemeCode,
        [FromQuery] DateOnly startDate,
        [FromQuery] DateOnly endDate,
        CancellationToken ct)
    {
        if (startDate >= endDate)
            return BadRequest(new { message = "startDate must be before endDate." });

        var result = await navService.GetReturnsAsync(schemeCode, startDate, endDate, ct);
        return result is null
            ? NotFound(new { message = $"Fund with scheme code {schemeCode} not found." })
            : Ok(result);
    }

    /// <summary>
    /// Get rolling returns (annualized CAGR) for every possible window of the given period.
    /// Includes summary statistics: min, max, average, median.
    /// Supports interval-based sampling to reduce data points.
    /// </summary>
    [HttpGet("{schemeCode:int}/rolling-returns")]
    public async Task<IActionResult> GetRollingReturns(
        int schemeCode,
        [FromQuery] int periodInYears = 1,
        [FromQuery] DateOnly? from = null,
        [FromQuery] DateOnly? to = null,
        [FromQuery] string? interval = null,
        CancellationToken ct = default)
    {
        if (periodInYears < 1 || periodInYears > 30)
            return BadRequest(new { message = "periodInYears must be between 1 and 30." });

        if (from.HasValue && to.HasValue && from.Value >= to.Value)
            return BadRequest(new { message = "'from' must be before 'to'." });

        if (!ValidateInterval(interval, out var error))
            return BadRequest(new { message = error });

        var result = await navService.GetRollingReturnsAsync(schemeCode, periodInYears, from, to, interval, ct);
        return result is null
            ? NotFound(new { message = $"Fund with scheme code {schemeCode} not found." })
            : Ok(result);
    }

    private static bool ValidateInterval(string? interval, out string? error)
    {
        error = null;
        if (interval is not null && !ValidIntervals.Contains(interval, StringComparer.OrdinalIgnoreCase))
        {
            error = "interval must be 'daily', 'weekly', or 'monthly'.";
            return false;
        }
        return true;
    }
}
