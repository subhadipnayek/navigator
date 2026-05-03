using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;

namespace NAVigator.Repositories;

public class FundRepository(AppDbContext db) : IFundRepository
{
    public async Task<List<Fund>> GetAllAsync()
    {
        return await db.Funds
            .AsNoTracking()
            .Include(f => f.NavSyncTracker)
            .OrderBy(f => f.SchemeName)
            .ToListAsync();
    }

    public async Task<Fund?> GetBySchemeCodeAsync(int schemeCode)
    {
        return await db.Funds
            .Include(f => f.NavSyncTracker)
            .FirstOrDefaultAsync(f => f.SchemeCode == schemeCode);
    }

    public async Task<List<Fund>> SearchAsync(string query, int maxResults = 50)
    {
        return await db.Funds
            .AsNoTracking()
            .Include(f => f.NavSyncTracker)
            .Where(f => EF.Functions.ILike(f.SchemeName, $"%{query}%")
                     || EF.Functions.ILike(f.FundHouse!, $"%{query}%")
                     || f.SchemeCode.ToString() == query)
            .OrderBy(f => f.SchemeName)
            .Take(maxResults)
            .ToListAsync();
    }

    public async Task BulkUpsertAsync(List<Fund> funds)
    {
        var existingCodes = await db.Funds
            .Select(f => f.SchemeCode)
            .ToHashSetAsync();

        var toAdd = new List<Fund>();
        var toUpdate = new List<Fund>();

        foreach (var fund in funds)
        {
            if (existingCodes.Contains(fund.SchemeCode))
                toUpdate.Add(fund);
            else
                toAdd.Add(fund);
        }

        if (toAdd.Count > 0)
            await db.Funds.AddRangeAsync(toAdd);

        foreach (var fund in toUpdate)
        {
            var existing = await db.Funds.FindAsync(fund.SchemeCode);
            if (existing is null) continue;

            existing.SchemeName = fund.SchemeName;
            existing.IsinGrowth = fund.IsinGrowth;
            existing.IsinDivReinvestment = fund.IsinDivReinvestment;
            existing.LatestNav = fund.LatestNav;
            existing.LatestNavDate = fund.LatestNavDate;
            existing.FundHouse = fund.FundHouse;
            existing.SchemeType = fund.SchemeType;
            existing.SchemeCategory = fund.SchemeCategory;
            existing.UpdatedAt = DateTime.UtcNow;
        }

        await db.SaveChangesAsync();
    }

    public async Task UpdateLatestNavAsync(int schemeCode, decimal nav, DateOnly date)
    {
        var fund = await db.Funds.FindAsync(schemeCode);
        if (fund is null) return;

        fund.LatestNav = nav;
        fund.LatestNavDate = date;
        fund.UpdatedAt = DateTime.UtcNow;
        await db.SaveChangesAsync();
    }

    public async Task UpdateNavDateRangeAsync(int schemeCode, DateOnly? startDate, DateOnly? endDate)
    {
        var fund = await db.Funds.FindAsync(schemeCode);
        if (fund is null) return;

        fund.NavStartDate = startDate;
        fund.NavEndDate = endDate;
        fund.UpdatedAt = DateTime.UtcNow;
        await db.SaveChangesAsync();
    }

    public async Task<int> GetCountAsync()
    {
        return await db.Funds.CountAsync();
    }
}
