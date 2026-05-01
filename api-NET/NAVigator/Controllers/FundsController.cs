using Microsoft.AspNetCore.Mvc;
using Microsoft.AspNetCore.Authorization;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
public class FundsController(IFundService fundService) : ControllerBase
{
    /// <summary>Get all funds from the database.</summary>
    [HttpGet]
    public async Task<IActionResult> GetAll()
    {
        var funds = await fundService.GetAllFundsAsync();
        return Ok(funds);
    }

    /// <summary>Get a single fund by scheme code.</summary>
    [HttpGet("{schemeCode:int}")]
    public async Task<IActionResult> GetBySchemeCode(int schemeCode)
    {
        var fund = await fundService.GetFundAsync(schemeCode);
        return fund is null ? NotFound() : Ok(fund);
    }

    /// <summary>Search funds by name, fund house, or scheme code.</summary>
    [HttpGet("search")]
    public async Task<IActionResult> Search([FromQuery] string q)
    {
        if (string.IsNullOrWhiteSpace(q))
            return BadRequest("Query parameter 'q' is required.");

        var funds = await fundService.SearchFundsAsync(q);
        return Ok(funds);
    }

    /// <summary>
    /// Trigger a manual sync of the fund master list from AMFI.
    /// This downloads and upserts ~20,000+ fund records.
    /// </summary>
    [HttpPost("sync")]
    [Authorize(Roles = "Admin")]
    public async Task<IActionResult> Sync(CancellationToken ct)
    {
        var result = await fundService.SyncFundMasterListAsync(ct);
        return Ok(result);
    }

    /// <summary>Get the count of funds in the database.</summary>
    [HttpGet("count")]
    public async Task<IActionResult> GetCount()
    {
        var count = await fundService.GetFundCountAsync();
        return Ok(new { count });
    }
}
