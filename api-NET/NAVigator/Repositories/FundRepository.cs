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
        // Enforce minimum query length to avoid expensive full-table ILIKE scans
        if (query.Trim().Length < 2)
            return [];

        var trimmed = query.Trim();

        // If the query is a pure integer, search by exact scheme code first
        if (int.TryParse(trimmed, out var schemeCode))
        {
            var exactMatch = await db.Funds
                .AsNoTracking()
                .Include(f => f.NavSyncTracker)
                .Where(f => f.SchemeCode == schemeCode)
                .ToListAsync();

            if (exactMatch.Count > 0)
                return exactMatch;
        }

        var terms = trimmed.Split(' ', StringSplitOptions.RemoveEmptyEntries);
        bool endsWithSpace = query.EndsWith(" ");

        var queryable = db.Funds
            .AsNoTracking()
            .Include(f => f.NavSyncTracker)
            .AsQueryable();

        for (int i = 0; i < terms.Length; i++)
        {
            var term = terms[i];
            bool isCompletedWord = (i < terms.Length - 1) || endsWithSpace;

            if (isCompletedWord)
            {
                // Enforce boundaries on BOTH sides
                // It must either start the string or have a space before it, AND end the string or have a space after it.
                var matchMidSpace = $"% {term} %";
                var matchStartSpace = $"{term} %";
                var matchMidEnd = $"% {term}";
                
                queryable = queryable.Where(f => 
                    f.SchemeName == term ||
                    EF.Functions.ILike(f.SchemeName, matchMidSpace) || 
                    EF.Functions.ILike(f.SchemeName, matchStartSpace) ||
                    EF.Functions.ILike(f.SchemeName, matchMidEnd) ||
                    f.FundHouse == term ||
                    EF.Functions.ILike(f.FundHouse!, matchMidSpace) || 
                    EF.Functions.ILike(f.FundHouse!, matchStartSpace) ||
                    EF.Functions.ILike(f.FundHouse!, matchMidEnd));
            }
            else
            {
                // Enforce boundary only on the START side (prefix match)
                // It must either start the string or have a space before it.
                var matchMidStart = $"% {term}%";
                var matchStartOnly = $"{term}%";

                queryable = queryable.Where(f => 
                    EF.Functions.ILike(f.SchemeName, matchMidStart) || 
                    EF.Functions.ILike(f.SchemeName, matchStartOnly) ||
                    EF.Functions.ILike(f.FundHouse!, matchMidStart) || 
                    EF.Functions.ILike(f.FundHouse!, matchStartOnly));
            }
        }

        return await queryable
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
            existing.LaunchDate = fund.LaunchDate;
            existing.FundHouse = fund.FundHouse;
            existing.SchemeType = fund.SchemeType;
            existing.SchemeCategory = fund.SchemeCategory;
            existing.UpdatedAt = DateTime.UtcNow;
        }

        await db.SaveChangesAsync();
    }

    public async Task<int> GetCountAsync()
    {
        return await db.Funds.CountAsync();
    }
}
