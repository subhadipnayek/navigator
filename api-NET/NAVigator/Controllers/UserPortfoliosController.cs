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

    [HttpPost("fund-matches")]
    public async Task<IActionResult> MatchFunds([FromBody] MatchPortfolioFundRequest request, CancellationToken ct)
    {
        if (request.Rows.Count == 0)
            return Ok(new MatchPortfolioFundResponse([]));

        var results = new List<MatchedPortfolioFundDto>();
        foreach (var row in request.Rows)
        {
            var isin = NormalizeOptional(row.Isin);
            var schemeCodeText = NormalizeOptional(row.SchemeCode);
            var schemeName = NormalizeOptional(row.SchemeName);
            var hasSchemeCode = int.TryParse(schemeCodeText, out var schemeCode);

            if (string.IsNullOrWhiteSpace(isin) && !hasSchemeCode)
            {
                results.Add(new MatchedPortfolioFundDto(row.RowNumber, null, null, "SchemeCode and ISIN both missing."));
                continue;
            }

            var candidates = db.Funds.AsNoTracking().AsQueryable();
            if (hasSchemeCode && !string.IsNullOrWhiteSpace(isin))
            {
                candidates = candidates.Where(f =>
                    f.SchemeCode == schemeCode &&
                    (f.IsinGrowth == isin || f.IsinDivReinvestment == isin));
            }
            else if (hasSchemeCode)
            {
                candidates = candidates.Where(f => f.SchemeCode == schemeCode);
            }
            else
            {
                candidates = candidates.Where(f => f.IsinGrowth == isin || f.IsinDivReinvestment == isin);
            }

            var matches = await candidates.Take(5).ToListAsync(ct);
            if (matches.Count == 0)
            {
                results.Add(new MatchedPortfolioFundDto(row.RowNumber, null, null, "SchemeCode and ISIN not found in database."));
                continue;
            }

            var match = matches.Count == 1
                ? matches[0]
                : matches.FirstOrDefault(f => string.Equals(f.SchemeName, schemeName, StringComparison.OrdinalIgnoreCase));

            if (match is null)
            {
                results.Add(new MatchedPortfolioFundDto(row.RowNumber, null, null, "Multiple fund matches found; SchemeName did not disambiguate."));
                continue;
            }

            results.Add(new MatchedPortfolioFundDto(row.RowNumber, match.SchemeCode, match.SchemeName, null));
        }

        return Ok(new MatchPortfolioFundResponse(results));
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

    private static string? NormalizeOptional(string? value) =>
        string.IsNullOrWhiteSpace(value) ? null : value.Trim();
}
