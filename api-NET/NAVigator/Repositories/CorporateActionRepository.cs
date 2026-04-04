using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;

namespace NAVigator.Repositories;

public class CorporateActionRepository(AppDbContext db) : ICorporateActionRepository
{
    public async Task<List<CorporateAction>> GetBySchemeCodeAsync(int schemeCode)
    {
        return await db.CorporateActions
            .AsNoTracking()
            .Where(ca => ca.SchemeCode == schemeCode)
            .OrderBy(ca => ca.ActionDate)
            .ToListAsync();
    }

    public async Task<CorporateAction?> GetByIdAsync(int id)
    {
        return await db.CorporateActions.FindAsync(id);
    }

    public async Task<CorporateAction> AddAsync(CorporateAction action)
    {
        db.CorporateActions.Add(action);
        await db.SaveChangesAsync();
        return action;
    }

    public async Task DeleteAsync(CorporateAction action)
    {
        db.CorporateActions.Remove(action);
        await db.SaveChangesAsync();
    }

    public async Task<HashSet<int>> GetSchemeCodesWithActionsAsync()
    {
        return (await db.CorporateActions
            .Select(ca => ca.SchemeCode)
            .Distinct()
            .ToListAsync())
            .ToHashSet();
    }
}
