using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;

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

    public async Task<(DateOnly? StartDate, DateOnly? EndDate, int Count)> GetStatsAsync(int schemeCode)
    {
        var stats = await db.NavHistories
            .Where(n => n.SchemeCode == schemeCode)
            .GroupBy(n => n.SchemeCode)
            .Select(g => new
            {
                StartDate = g.Min(n => (DateOnly?)n.Date),
                EndDate = g.Max(n => (DateOnly?)n.Date),
                Count = g.Count()
            })
            .FirstOrDefaultAsync();

        return stats is null ? (null, null, 0) : (stats.StartDate, stats.EndDate, stats.Count);
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

    public async Task UpsertLatestNavSnapshotsAsync(
        List<AmfiLatestNavSnapshot> snapshots,
        DateTime checkedAtIst,
        CancellationToken ct = default)
    {
        if (snapshots.Count == 0)
            return;

        var schemeCodes = snapshots.Select(s => s.SchemeCode).Distinct().ToList();
        var validSchemeCodes = await db.Funds
            .Where(f => schemeCodes.Contains(f.SchemeCode))
            .Select(f => f.SchemeCode)
            .ToHashSetAsync(ct);
        var existingTrackers = await db.NavSyncTrackers
            .Where(t => schemeCodes.Contains(t.SchemeCode))
            .ToDictionaryAsync(t => t.SchemeCode, ct);

        foreach (var snapshot in snapshots)
        {
            if (!validSchemeCodes.Contains(snapshot.SchemeCode))
                continue;

            if (!existingTrackers.TryGetValue(snapshot.SchemeCode, out var tracker))
            {
                tracker = new NavSyncTracker
                {
                    SchemeCode = snapshot.SchemeCode,
                    UpdatedAtIst = checkedAtIst
                };
                await db.NavSyncTrackers.AddAsync(tracker, ct);
                existingTrackers[snapshot.SchemeCode] = tracker;
            }

            tracker.LatestNav = snapshot.LatestNav;
            tracker.LatestNavDate = snapshot.LatestNavDate;
            tracker.LastLatestNavCheckAtIst = checkedAtIst;
            tracker.UpdatedAtIst = checkedAtIst;
        }

        await db.SaveChangesAsync(ct);
    }

    public async Task UpdateTrackerHistoryStatsAsync(
        int schemeCode,
        DateOnly? startDate,
        DateOnly? endDate,
        int count,
        DateTime checkedAtIst,
        bool hadNewData,
        CancellationToken ct = default)
    {
        var tracker = await db.NavSyncTrackers.FindAsync([schemeCode], ct);
        if (tracker is null)
        {
            tracker = new NavSyncTracker
            {
                SchemeCode = schemeCode
            };
            await db.NavSyncTrackers.AddAsync(tracker, ct);
        }

        tracker.NavStartDate = startDate;
        tracker.NavEndDate = endDate;
        tracker.NavHistoryCount = count;
        tracker.LastHistoryCheckAtIst = checkedAtIst;
        tracker.NavHistoryUpdatedAtIst = hadNewData ? checkedAtIst : tracker.NavHistoryUpdatedAtIst;
        tracker.LastCheckHadNewData = hadNewData;
        tracker.UpdatedAtIst = checkedAtIst;

        await db.SaveChangesAsync(ct);
    }

    public async Task UpdateTrackerCorporateActionCountAsync(
        int schemeCode,
        int count,
        DateTime updatedAtIst,
        CancellationToken ct = default)
    {
        var tracker = await db.NavSyncTrackers.FindAsync([schemeCode], ct);
        if (tracker is null)
        {
            tracker = new NavSyncTracker
            {
                SchemeCode = schemeCode
            };
            await db.NavSyncTrackers.AddAsync(tracker, ct);
        }

        tracker.CorporateActionCount = count;
        tracker.UpdatedAtIst = updatedAtIst;

        await db.SaveChangesAsync(ct);
    }
}
