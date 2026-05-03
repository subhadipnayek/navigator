using System.Globalization;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Dto;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Authorize(Roles = "Admin")]
[Route("api/admin")]
public class AdminController(
    AppDbContext db,
    IFundService fundService,
    INavService navService) : ControllerBase
{
    private static readonly string[] ValidNavStatuses = ["all", "with-nav", "without-nav"];
    private const int LikelyDiscontinuedGraceDays = 30;

    [HttpGet("funds/summary")]
    public async Task<IActionResult> GetFundsSummary(CancellationToken ct)
    {
        var totalFunds = await db.Funds.CountAsync(ct);
        var fundsWithNav = await db.NavHistories.Select(n => n.SchemeCode).Distinct().CountAsync(ct);
        var navRecords = await db.NavHistories.LongCountAsync(ct);
        var corporateActions = await db.CorporateActions.CountAsync(ct);
        var oldestNav = await db.NavHistories.MinAsync(n => (DateOnly?)n.Date, ct);
        var latestNav = await db.NavHistories.MaxAsync(n => (DateOnly?)n.Date, ct);

        return Ok(new AdminDatabaseSummaryDto
        {
            TotalFunds = totalFunds,
            FundsWithNavHistory = fundsWithNav,
            FundsWithoutNavHistory = totalFunds - fundsWithNav,
            NavHistoryRecords = navRecords,
            CorporateActions = corporateActions,
            OldestNavDate = FormatDate(oldestNav),
            LatestNavDate = FormatDate(latestNav)
        });
    }

    [HttpGet("funds")]
    public async Task<IActionResult> GetFunds(
        [FromQuery] string? q = null,
        [FromQuery] string navStatus = "all",
        [FromQuery] int page = 1,
        [FromQuery] int pageSize = 50,
        CancellationToken ct = default)
    {
        if (!ValidNavStatuses.Contains(navStatus, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "navStatus must be 'all', 'with-nav', or 'without-nav'." });

        if (page < 1)
            return BadRequest(new { message = "page must be greater than or equal to 1." });

        if (pageSize is < 1 or > 200)
            return BadRequest(new { message = "pageSize must be between 1 and 200." });

        var query = db.Funds.AsNoTracking();

        if (!string.IsNullOrWhiteSpace(q))
        {
            var search = q.Trim();
            query = query.Where(f =>
                EF.Functions.ILike(f.SchemeName, $"%{search}%")
                || EF.Functions.ILike(f.FundHouse!, $"%{search}%")
                || f.SchemeCode.ToString() == search);
        }

        query = navStatus.ToLowerInvariant() switch
        {
            "with-nav" => query.Where(f => f.NavHistories.Any()),
            "without-nav" => query.Where(f => !f.NavHistories.Any()),
            _ => query
        };

        var totalItems = await query.CountAsync(ct);
        var rawItems = await query
            .Select(f => new
            {
                f.SchemeCode,
                f.SchemeName,
                f.FundHouse,
                f.SchemeType,
                f.SchemeCategory,
                f.LatestNav,
                f.LatestNavDate,
                f.NavStartDate,
                f.NavEndDate,
                LastApiCheckUtc = f.NavSyncTracker == null ? (DateTime?)null : f.NavSyncTracker.LastApiCheckUtc,
                NavHistoryCount = f.NavHistories.Count(),
                FirstStoredNavDate = f.NavHistories.Min(n => (DateOnly?)n.Date),
                LastStoredNavDate = f.NavHistories.Max(n => (DateOnly?)n.Date),
                CorporateActionCount = f.CorporateActions.Count()
            })
            .OrderByDescending(f => f.NavHistoryCount)
            .ThenBy(f => f.SchemeName)
            .Skip((page - 1) * pageSize)
            .Take(pageSize)
            .ToListAsync(ct);
        var items = rawItems.Select(f => new AdminFundStorageDto
        {
            SchemeCode = f.SchemeCode,
            SchemeName = f.SchemeName,
            FundHouse = f.FundHouse,
            SchemeType = f.SchemeType,
            SchemeCategory = f.SchemeCategory,
            LatestNav = f.LatestNav,
            LatestNavDate = FormatDate(f.LatestNavDate),
            NavStartDate = FormatDate(f.NavStartDate),
            NavEndDate = FormatDate(f.NavEndDate),
            LastSuccessfulNavCheckUtc = FormatUtc(f.LastApiCheckUtc),
            NavStatus = GetNavStatus(f.LatestNavDate, f.LastApiCheckUtc),
            IsLikelyDiscontinued = IsLikelyDiscontinued(f.LatestNavDate, f.LastApiCheckUtc),
            NavHistoryCount = f.NavHistoryCount,
            FirstStoredNavDate = FormatDate(f.FirstStoredNavDate),
            LastStoredNavDate = FormatDate(f.LastStoredNavDate),
            CorporateActionCount = f.CorporateActionCount
        }).ToList();

        return Ok(new PagedResponseDto<AdminFundStorageDto>
        {
            Page = page,
            PageSize = pageSize,
            TotalItems = totalItems,
            TotalPages = (int)Math.Ceiling(totalItems / (double)pageSize),
            Items = items
        });
    }

    [HttpPost("funds/sync")]
    public async Task<IActionResult> SyncFundMaster(CancellationToken ct)
    {
        var result = await fundService.SyncFundMasterListAsync(ct);
        return Ok(result);
    }

    [HttpPost("funds/{schemeCode:int}/nav-history/sync")]
    public async Task<IActionResult> SyncFundNavHistory(int schemeCode, CancellationToken ct)
    {
        var result = await navService.GetNavHistoryAsync(schemeCode, ct: ct);
        return result is null
            ? NotFound(new { message = $"Fund with scheme code {schemeCode} not found." })
            : Ok(result);
    }

    [HttpDelete("funds/{schemeCode:int}/nav-history")]
    public async Task<IActionResult> DeleteFundNavHistory(int schemeCode, CancellationToken ct)
    {
        var fundExists = await db.Funds.AnyAsync(f => f.SchemeCode == schemeCode, ct);
        if (!fundExists)
            return NotFound(new { message = $"Fund with scheme code {schemeCode} not found." });

        await db.NavHistories.Where(n => n.SchemeCode == schemeCode).ExecuteDeleteAsync(ct);
        await db.NavSyncTrackers.Where(t => t.SchemeCode == schemeCode).ExecuteDeleteAsync(ct);
        await db.Funds
            .Where(f => f.SchemeCode == schemeCode)
            .ExecuteUpdateAsync(setters => setters
                .SetProperty(f => f.NavStartDate, (DateOnly?)null)
                .SetProperty(f => f.NavEndDate, (DateOnly?)null), ct);
        return NoContent();
    }

    [HttpDelete("funds/{schemeCode:int}")]
    public async Task<IActionResult> DeleteFund(int schemeCode, CancellationToken ct)
    {
        var fund = await db.Funds.FindAsync([schemeCode], ct);
        if (fund is null)
            return NotFound(new { message = $"Fund with scheme code {schemeCode} not found." });

        db.Funds.Remove(fund);
        await db.SaveChangesAsync(ct);
        return NoContent();
    }

    private static string? FormatDate(DateOnly? date)
    {
        return date?.ToString("dd-MM-yyyy", CultureInfo.InvariantCulture);
    }

    private static string? FormatUtc(DateTime? dateTime)
    {
        return dateTime?.ToUniversalTime().ToString("O", CultureInfo.InvariantCulture);
    }

    private static string GetNavStatus(DateOnly? latestNavDate, DateTime? lastSuccessfulCheckUtc)
    {
        if (!latestNavDate.HasValue)
            return "Unknown";

        var today = GetIndiaToday();
        if (today.DayNumber - latestNavDate.Value.DayNumber <= LikelyDiscontinuedGraceDays)
            return "Active";

        if (!lastSuccessfulCheckUtc.HasValue)
            return "NeedsCheck";

        return IsLikelyDiscontinued(latestNavDate, lastSuccessfulCheckUtc)
            ? "LikelyDiscontinued"
            : "NeedsCheck";
    }

    private static bool IsLikelyDiscontinued(DateOnly? latestNavDate, DateTime? lastSuccessfulCheckUtc)
    {
        if (!latestNavDate.HasValue || !lastSuccessfulCheckUtc.HasValue)
            return false;

        var lastCheckDate = DateOnly.FromDateTime(TimeZoneInfo.ConvertTimeFromUtc(
            lastSuccessfulCheckUtc.Value.ToUniversalTime(),
            TimeZoneInfo.FindSystemTimeZoneById("India Standard Time")));
        return lastCheckDate.DayNumber - latestNavDate.Value.DayNumber > LikelyDiscontinuedGraceDays;
    }

    private static DateOnly GetIndiaToday()
        => DateOnly.FromDateTime(TimeZoneInfo.ConvertTimeFromUtc(
            DateTime.UtcNow, TimeZoneInfo.FindSystemTimeZoneById("India Standard Time")));
}
