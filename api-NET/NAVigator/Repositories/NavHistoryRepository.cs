using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;

namespace NAVigator.Repositories;

public class NavHistoryRepository(AppDbContext db) : INavHistoryRepository
{
    public async Task<List<NavHistory>> GetBySchemeCodeAsync(int schemeCode, DateOnly? from = null, DateOnly? to = null)
    {
        var query = db.NavHistories
            .AsNoTracking()
            .Where(n => n.SchemeCode == schemeCode);

        if (from.HasValue)
            query = query.Where(n => n.Date >= from.Value);

        if (to.HasValue)
            query = query.Where(n => n.Date <= to.Value);

        return await query
            .OrderByDescending(n => n.Date)
            .ToListAsync();
    }

    public async Task<(DateOnly? StartDate, DateOnly? EndDate)> GetDateRangeAsync(int schemeCode)
    {
        var range = await db.NavHistories
            .Where(n => n.SchemeCode == schemeCode)
            .GroupBy(n => n.SchemeCode)
            .Select(g => new
            {
                StartDate = g.Min(n => (DateOnly?)n.Date),
                EndDate = g.Max(n => (DateOnly?)n.Date)
            })
            .FirstOrDefaultAsync();

        return range is null ? (null, null) : (range.StartDate, range.EndDate);
    }

    public async Task<DateOnly?> GetLatestNavDateAsync(int schemeCode)
    {
        return await db.NavHistories
            .Where(n => n.SchemeCode == schemeCode)
            .MaxAsync(n => (DateOnly?)n.Date);
    }

    public async Task<NavHistory?> GetLatestNavEntryAsync(int schemeCode)
    {
        return await db.NavHistories
            .AsNoTracking()
            .Where(n => n.SchemeCode == schemeCode)
            .OrderByDescending(n => n.Date)
            .FirstOrDefaultAsync();
    }

    public async Task AddRangeAsync(List<NavHistory> entries)
    {
        if (entries.Count == 0) return;

        // Get existing dates for this scheme to avoid duplicates
        var schemeCode = entries[0].SchemeCode;
        var existingDates = await db.NavHistories
            .Where(n => n.SchemeCode == schemeCode)
            .Select(n => n.Date)
            .ToHashSetAsync();

        var newEntries = entries.Where(e => !existingDates.Contains(e.Date)).ToList();

        if (newEntries.Count > 0)
        {
            try
            {
                await db.NavHistories.AddRangeAsync(newEntries);
                await db.SaveChangesAsync();
            }
            catch (DbUpdateException) when (newEntries.Count > 0)
            {
                // A concurrent request may have inserted overlapping rows.
                // Detach the failed entities and retry with a fresh duplicate check.
                foreach (var entry in newEntries)
                    db.Entry(entry).State = EntityState.Detached;

                var nowExisting = await db.NavHistories
                    .Where(n => n.SchemeCode == schemeCode)
                    .Select(n => n.Date)
                    .ToHashSetAsync();

                var retryEntries = newEntries.Where(e => !nowExisting.Contains(e.Date)).ToList();
                if (retryEntries.Count > 0)
                {
                    await db.NavHistories.AddRangeAsync(retryEntries);
                    await db.SaveChangesAsync();
                }
            }
        }
    }

    public async Task<NavSyncTracker?> GetSyncTrackerAsync(int schemeCode)
    {
        return await db.NavSyncTrackers.FindAsync(schemeCode);
    }

    public async Task UpsertSyncTrackerAsync(NavSyncTracker tracker)
    {
        var existing = await db.NavSyncTrackers.FindAsync(tracker.SchemeCode);
        if (existing is null)
        {
            await db.NavSyncTrackers.AddAsync(tracker);
        }
        else
        {
            existing.LastApiCheckUtc = tracker.LastApiCheckUtc;
            existing.LatestNavDateFromApi = tracker.LatestNavDateFromApi;
            existing.LastCheckHadNewData = tracker.LastCheckHadNewData;
        }

        await db.SaveChangesAsync();
    }
}
