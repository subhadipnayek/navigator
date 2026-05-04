using System.Globalization;
using Microsoft.AspNetCore.Mvc;
using NAVigator.Models.Dto.Portfolio;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
public class PortfolioController(IPortfolioSimulationService simulationService) : ControllerBase
{
    private const string DateFmt = "dd-MM-yyyy";
    private static readonly string[] ValidRebalance = ["None", "Monthly", "Quarterly", "Yearly"];
    private static readonly string[] ValidIntervals = ["daily", "weekly", "monthly"];

    /// <summary>
    /// Simulate SIP and/or lump-sum investment in a basket of mutual funds.
    /// Returns detailed metrics, per-fund breakdown, and time-series output.
    /// </summary>
    [HttpPost("simulate")]
    public async Task<IActionResult> Simulate([FromBody] SimulationRequest request, CancellationToken ct)
    {
        if (!ValidateBasket(request.Basket, out var basketError))
            return BadRequest(new { message = basketError });
        if (!ValidateDates(request.StartDate, request.EndDate, out var dateError))
            return BadRequest(new { message = dateError });
        if (request.SipAmount < 0)
            return BadRequest(new { message = "SIP amount cannot be negative." });
        if (request.LumpSumAmount < 0)
            return BadRequest(new { message = "Lump sum amount cannot be negative." });
        if (request.SipAmount == 0 && request.LumpSumAmount == 0)
            return BadRequest(new { message = "At least one of SIP amount or lump sum must be greater than zero." });
        if (request.SipDay is < 1 or > 28)
            return BadRequest(new { message = "SIP day must be between 1 and 28." });

        try
        {
            var result = await simulationService.SimulateAsync(request, ct);
            return Ok(result);
        }
        catch (ArgumentException ex)
        {
            return BadRequest(new { message = ex.Message });
        }
    }

    /// <summary>
    /// Compare multiple baskets and/or individual funds side-by-side.
    /// Supports normalization for fair comparison across different SIP amounts.
    /// </summary>
    [HttpPost("compare")]
    public async Task<IActionResult> Compare([FromBody] ComparisonRequest request, CancellationToken ct)
    {
        if (request.Entries is null || request.Entries.Count < 2)
            return BadRequest(new { message = "At least 2 entries are required for comparison." });
        if (request.Entries.Count > 10)
            return BadRequest(new { message = "Maximum 10 entries allowed for comparison." });
        if (!ValidateDates(request.StartDate, request.EndDate, out var dateError))
            return BadRequest(new { message = dateError });

        foreach (var entry in request.Entries)
        {
            if (string.IsNullOrWhiteSpace(entry.Name))
                return BadRequest(new { message = "Each comparison entry must have a name." });
            if (entry.Basket is null && !entry.SingleFundSchemeCode.HasValue)
                return BadRequest(new { message = $"Entry '{entry.Name}' must specify either Basket or SingleFundSchemeCode." });
            if (entry.Basket is not null && !ValidateBasket(entry.Basket, out var bErr))
                return BadRequest(new { message = $"Entry '{entry.Name}': {bErr}" });
            if (entry.SipAmount == 0 && entry.LumpSumAmount == 0)
                return BadRequest(new { message = $"Entry '{entry.Name}' must have SIP or lump sum > 0." });
        }

        try
        {
            var result = await simulationService.CompareAsync(request, ct);
            return Ok(result);
        }
        catch (ArgumentException ex)
        {
            return BadRequest(new { message = ex.Message });
        }
    }

    /// <summary>
    /// Run what-if analysis: test multiple allocation and contribution strategies
    /// to discover which approach yields the best risk-adjusted returns.
    /// </summary>
    [HttpPost("what-if")]
    public async Task<IActionResult> WhatIf([FromBody] WhatIfRequest request, CancellationToken ct)
    {
        if (request.Scenarios is null || request.Scenarios.Count < 2)
            return BadRequest(new { message = "At least 2 scenarios are required." });
        if (request.Scenarios.Count > 10)
            return BadRequest(new { message = "Maximum 10 scenarios allowed." });
        if (!ValidateDates(request.StartDate, request.EndDate, out var dateError))
            return BadRequest(new { message = dateError });

        foreach (var scenario in request.Scenarios)
        {
            if (string.IsNullOrWhiteSpace(scenario.Name))
                return BadRequest(new { message = "Each scenario must have a name." });
            if (!ValidateBasket(scenario.Basket, out var bErr))
                return BadRequest(new { message = $"Scenario '{scenario.Name}': {bErr}" });
            if (scenario.SipAmount == 0 && scenario.LumpSumAmount == 0)
                return BadRequest(new { message = $"Scenario '{scenario.Name}' must have SIP or lump sum > 0." });
        }

        try
        {
            var result = await simulationService.WhatIfAsync(request, ct);
            return Ok(result);
        }
        catch (ArgumentException ex)
        {
            return BadRequest(new { message = ex.Message });
        }
    }

    /// <summary>
    /// Rolling returns comparison across multiple baskets/funds.
    /// Answers: "Which basket consistently outperforms across time periods?"
    /// </summary>
    [HttpPost("rolling-comparison")]
    public async Task<IActionResult> RollingComparison([FromBody] RollingComparisonRequest request, CancellationToken ct)
    {
        if (request.Entries is null || request.Entries.Count < 2)
            return BadRequest(new { message = "At least 2 entries are required." });
        if (request.Entries.Count > 10)
            return BadRequest(new { message = "Maximum 10 entries allowed." });
        if (!ValidateDates(request.StartDate, request.EndDate, out var dateError))
            return BadRequest(new { message = dateError });
        if (request.PeriodInYears is < 1 or > 30)
            return BadRequest(new { message = "periodInYears must be between 1 and 30." });
        if (request.Interval is not null && !ValidIntervals.Contains(request.Interval, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "interval must be 'daily', 'weekly', or 'monthly'." });

        foreach (var entry in request.Entries)
        {
            if (string.IsNullOrWhiteSpace(entry.Name))
                return BadRequest(new { message = "Each entry must have a name." });
            if (entry.Basket is null && !entry.SingleFundSchemeCode.HasValue)
                return BadRequest(new { message = $"Entry '{entry.Name}' must specify either Basket or SingleFundSchemeCode." });
            if (entry.Basket is not null && !ValidateBasket(entry.Basket, out var bErr))
                return BadRequest(new { message = $"Entry '{entry.Name}': {bErr}" });
        }

        try
        {
            var result = await simulationService.RollingComparisonAsync(request, ct);
            return Ok(result);
        }
        catch (ArgumentException ex)
        {
            return BadRequest(new { message = ex.Message });
        }
    }

    // ─── Validators ───

    private static bool ValidateBasket(BasketDefinition? basket, out string error)
    {
        error = string.Empty;
        if (basket is null) { error = "Basket is required."; return false; }
        if (string.IsNullOrWhiteSpace(basket.Name)) { error = "Basket name is required."; return false; }
        if (basket.Funds is null || basket.Funds.Count == 0) { error = "Basket must contain at least one fund."; return false; }

        var total = basket.Funds.Sum(f => f.AllocationPercent);
        if (Math.Abs(total - 100m) > 0.01m) { error = $"Allocation percentages must sum to 100% (got {total}%)."; return false; }

        foreach (var fund in basket.Funds)
        {
            if (fund.SchemeCode <= 0) { error = $"Invalid scheme code: {fund.SchemeCode}."; return false; }
            if (fund.AllocationPercent <= 0) { error = $"Allocation for scheme {fund.SchemeCode} must be positive."; return false; }
        }

        if (!ValidRebalance.Contains(basket.RebalanceFrequency, StringComparer.OrdinalIgnoreCase))
        { error = $"RebalanceFrequency must be one of: {string.Join(", ", ValidRebalance)}."; return false; }

        return true;
    }

    private static bool ValidateDates(string start, string end, out string error)
    {
        error = string.Empty;
        if (!DateOnly.TryParseExact(start, DateFmt, CultureInfo.InvariantCulture, DateTimeStyles.None, out var s))
        { error = $"Invalid start date '{start}'. Use dd-MM-yyyy format."; return false; }
        if (!DateOnly.TryParseExact(end, DateFmt, CultureInfo.InvariantCulture, DateTimeStyles.None, out var e))
        { error = $"Invalid end date '{end}'. Use dd-MM-yyyy format."; return false; }
        if (s >= e) { error = "Start date must be before end date."; return false; }
        return true;
    }
}
