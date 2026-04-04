using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class CorporateActionService(
    AppDbContext db,
    ICorporateActionRepository actionRepo,
    IFundRepository fundRepo,
    ILogger<CorporateActionService> logger) : ICorporateActionService
{
    private static readonly string[] ValidActionTypes = ["Split", "Merger", "Bonus", "Other"];

    public async Task<List<NavAnomalyDto>> DetectAnomaliesAsync(decimal thresholdPercent = 30m, CancellationToken ct = default)
    {
        logger.LogInformation("Detecting NAV anomalies with threshold {Threshold}%", thresholdPercent);

        // Get scheme codes that already have corporate actions recorded
        var schemesWithActions = await actionRepo.GetSchemeCodesWithActionsAsync();

        // Query all funds that have at least 2 NAV entries, looking for large day-over-day swings.
        // We do this in SQL for performance rather than loading all NAV history into memory.
        var anomalies = await db.NavHistories
            .AsNoTracking()
            .GroupBy(n => n.SchemeCode)
            .Where(g => g.Count() > 1)
            .SelectMany(g => g
                .OrderBy(n => n.Date)
                .Select(n => new { n.SchemeCode, n.Date, n.Nav }))
            .ToListAsync(ct);

        // Process in memory — group by scheme, detect day-over-day anomalies
        var grouped = anomalies
            .GroupBy(n => n.SchemeCode)
            .ToDictionary(g => g.Key, g => g.OrderBy(n => n.Date).ToList());

        // Get all corporate action dates for quick lookup
        var allActions = await db.CorporateActions
            .AsNoTracking()
            .Select(ca => new { ca.SchemeCode, ca.ActionDate })
            .ToListAsync(ct);
        var actionLookup = allActions
            .ToLookup(a => a.SchemeCode, a => a.ActionDate);

        var result = new List<NavAnomalyDto>();

        // Load fund names in bulk
        var schemeCodes = grouped.Keys.ToList();
        var funds = await db.Set<Fund>()
            .AsNoTracking()
            .Where(f => schemeCodes.Contains(f.SchemeCode))
            .Select(f => new { f.SchemeCode, f.SchemeName, f.FundHouse })
            .ToDictionaryAsync(f => f.SchemeCode, ct);

        foreach (var (schemeCode, navEntries) in grouped)
        {
            var actionDates = actionLookup[schemeCode].ToHashSet();

            for (int i = 1; i < navEntries.Count; i++)
            {
                var prev = navEntries[i - 1];
                var curr = navEntries[i];

                if (prev.Nav <= 0) continue;

                var changePercent = (curr.Nav - prev.Nav) / prev.Nav * 100m;

                if (Math.Abs(changePercent) >= thresholdPercent)
                {
                    funds.TryGetValue(schemeCode, out var fund);

                    result.Add(new NavAnomalyDto
                    {
                        SchemeCode = schemeCode,
                        SchemeName = fund?.SchemeName ?? "Unknown",
                        FundHouse = fund?.FundHouse,
                        Date = curr.Date.ToString("dd-MM-yyyy"),
                        PreviousNav = prev.Nav,
                        CurrentNav = curr.Nav,
                        ChangePercent = Math.Round(changePercent, 2),
                        Direction = changePercent < 0 ? "Drop" : "Spike",
                        HasCorporateAction = actionDates.Contains(curr.Date)
                    });
                }
            }
        }

        logger.LogInformation("Found {Count} anomalies across all funds", result.Count);

        return result
            .OrderByDescending(a => Math.Abs(a.ChangePercent))
            .ToList();
    }

    public async Task<List<CorporateActionDto>> GetActionsAsync(int schemeCode)
    {
        var actions = await actionRepo.GetBySchemeCodeAsync(schemeCode);

        return actions.Select(a => new CorporateActionDto
        {
            Id = a.Id,
            SchemeCode = a.SchemeCode,
            ActionDate = a.ActionDate.ToString("dd-MM-yyyy"),
            ActionType = a.ActionType,
            AdjustmentFactor = a.AdjustmentFactor,
            OldFaceValue = a.OldFaceValue,
            NewFaceValue = a.NewFaceValue,
            Notes = a.Notes
        }).ToList();
    }

    public async Task<CorporateActionDto?> AddActionAsync(int schemeCode, CreateCorporateActionRequest request)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
            return null;

        if (!ValidActionTypes.Contains(request.ActionType, StringComparer.OrdinalIgnoreCase))
            return null;

        var action = new CorporateAction
        {
            SchemeCode = schemeCode,
            ActionDate = request.ActionDate,
            ActionType = request.ActionType,
            AdjustmentFactor = request.AdjustmentFactor,
            OldFaceValue = request.OldFaceValue,
            NewFaceValue = request.NewFaceValue,
            Notes = request.Notes
        };

        await actionRepo.AddAsync(action);

        logger.LogInformation(
            "Added corporate action for {SchemeCode}: {ActionType} on {Date} with factor {Factor}",
            schemeCode, request.ActionType, request.ActionDate, request.AdjustmentFactor);

        return new CorporateActionDto
        {
            Id = action.Id,
            SchemeCode = action.SchemeCode,
            ActionDate = action.ActionDate.ToString("dd-MM-yyyy"),
            ActionType = action.ActionType,
            AdjustmentFactor = action.AdjustmentFactor,
            OldFaceValue = action.OldFaceValue,
            NewFaceValue = action.NewFaceValue,
            Notes = action.Notes
        };
    }

    public async Task<bool> DeleteActionAsync(int schemeCode, int actionId)
    {
        var action = await actionRepo.GetByIdAsync(actionId);
        if (action is null || action.SchemeCode != schemeCode)
            return false;

        await actionRepo.DeleteAsync(action);

        logger.LogInformation("Deleted corporate action {Id} for scheme {SchemeCode}", actionId, schemeCode);
        return true;
    }

    public async Task<decimal> GetCumulativeAdjustmentFactorAsync(int schemeCode, DateOnly fromDate, DateOnly toDate)
    {
        var actions = await actionRepo.GetBySchemeCodeAsync(schemeCode);

        // Multiply adjustment factors for all actions between fromDate (exclusive) and toDate (inclusive).
        // This gives the cumulative factor to adjust the fromDate NAV to be comparable with toDate NAV.
        decimal cumulativeFactor = 1m;

        foreach (var action in actions)
        {
            if (action.ActionDate > fromDate && action.ActionDate <= toDate)
            {
                cumulativeFactor *= action.AdjustmentFactor;
            }
        }

        return cumulativeFactor;
    }
}
