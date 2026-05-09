using System.Security.Claims;
using Microsoft.EntityFrameworkCore;
using Microsoft.AspNetCore.Mvc;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto.Portfolio;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/user-portfolios")]
public class UserPortfoliosController(IUserPortfolioService portfolioService, AppDbContext db) : ControllerBase
{
    [HttpGet]
    public async Task<IActionResult> GetPortfolios(CancellationToken ct)
    {
        var rows = await portfolioService.GetPortfoliosAsync(GetUserId(), ct);
        return Ok(rows);
    }

    [HttpPost]
    public async Task<IActionResult> CreatePortfolio([FromBody] CreateUserPortfolioRequest request, CancellationToken ct)
    {
        var row = await portfolioService.CreatePortfolioAsync(GetUserId(), request, ct);
        return Ok(row);
    }

    [HttpGet("{portfolioId:long}/dashboard")]
    public async Task<IActionResult> GetDashboard(long portfolioId, [FromQuery] bool showFullyExitedFunds, CancellationToken ct)
    {
        var dashboard = await portfolioService.GetDashboardAsync(GetUserId(), portfolioId, showFullyExitedFunds, ct);
        return dashboard is null ? NotFound() : Ok(dashboard);
    }

    [HttpGet("{portfolioId:long}/transactions")]
    public async Task<IActionResult> GetTransactions(long portfolioId, [FromQuery] string financialYear, CancellationToken ct)
    {
        if (!IsValidFinancialYear(financialYear))
            return BadRequest(new { message = "financialYear must use the format 2023-24." });

        var result = await portfolioService.GetTransactionsAsync(GetUserId(), portfolioId, financialYear, ct);
        return result is null ? NotFound() : Ok(new { transactions = result.Value.Rows, summary = result.Value.Summary });
    }

    [HttpPut("{portfolioId:long}/transactions")]
    public async Task<IActionResult> SaveTransactions(long portfolioId, [FromBody] SavePortfolioTransactionsRequest request, CancellationToken ct)
    {
        if (!IsValidFinancialYear(request.FinancialYear))
            return BadRequest(new { message = "financialYear must use the format 2023-24." });

        var result = await portfolioService.SaveTransactionsAsync(GetUserId(), portfolioId, request, ct);
        if (result is null) return NotFound();
        return result.IsValid ? Ok(result) : BadRequest(result);
    }

    [HttpPost("{portfolioId:long}/benchmark")]
    public async Task<IActionResult> CompareBenchmark(long portfolioId, [FromBody] PortfolioBenchmarkRequest request, CancellationToken ct)
    {
        var result = await portfolioService.CompareBenchmarkAsync(GetUserId(), portfolioId, request, ct);
        return result is null ? NotFound() : Ok(result);
    }

    [HttpGet("grid-preferences/{gridKey}")]
    public async Task<IActionResult> GetGridPreference(string gridKey, CancellationToken ct)
    {
        var userId = GetUserId();
        var preference = await db.UserGridPreferences
            .AsNoTracking()
            .FirstOrDefaultAsync(p => p.UserId == userId && p.GridKey == gridKey, ct);

        return Ok(new UserGridPreferenceDto(gridKey, preference?.StateJson));
    }

    [HttpPut("grid-preferences/{gridKey}")]
    public async Task<IActionResult> SaveGridPreference(string gridKey, [FromBody] SaveUserGridPreferenceRequest request, CancellationToken ct)
    {
        if (string.IsNullOrWhiteSpace(request.StateJson))
            return BadRequest(new { message = "Grid state cannot be empty." });

        var userId = GetUserId();
        var preference = await db.UserGridPreferences
            .FirstOrDefaultAsync(p => p.UserId == userId && p.GridKey == gridKey, ct);

        if (preference is null)
        {
            preference = new UserGridPreference
            {
                UserId = userId,
                GridKey = gridKey,
                StateJson = request.StateJson
            };
            db.UserGridPreferences.Add(preference);
        }
        else
        {
            preference.StateJson = request.StateJson;
            preference.UpdatedAtUtc = DateTime.UtcNow;
        }

        await db.SaveChangesAsync(ct);
        return Ok(new UserGridPreferenceDto(gridKey, preference.StateJson));
    }

    private int GetUserId()
    {
        var claim = User.FindFirstValue(ClaimTypes.NameIdentifier);
        if (!int.TryParse(claim, out var userId))
            throw new InvalidOperationException("Authenticated user id claim is missing.");
        return userId;
    }

    private static bool IsValidFinancialYear(string value) =>
        value.Length == 7 &&
        int.TryParse(value[..4], out _) &&
        value[4] == '-' &&
        int.TryParse(value[5..], out _);
}
