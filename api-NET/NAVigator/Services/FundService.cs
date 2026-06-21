using System.Diagnostics;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class FundService(
    IFundRepository fundRepo,
    INavHistoryRepository navRepo,
    IAmfiService amfiService,
    INavHistoryProvider navHistoryProvider,
    ILogger<FundService> logger) : IFundService
{
    private const int LikelyDiscontinuedGraceDays = 30;

    public async Task<List<FundDto>> GetAllFundsAsync()
    {
        var funds = await fundRepo.GetAllAsync();
        return funds.Select(MapToDto).ToList();
    }

    public async Task<FundDto?> GetFundAsync(int schemeCode)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        if (fund is null)
            return null;

        if (fund.NavSyncTracker?.LastLatestNavCheckAtIst is not { } lastCheck
            || IndiaTime.Now() - lastCheck >= TimeSpan.FromHours(1))
        {
            await navHistoryProvider.RefreshLatestNavTrackerIfStaleAsync(schemeCode);
            fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
        }

        return fund is null ? null : MapToDto(fund);
    }

    public async Task<List<FundDto>> SearchFundsAsync(string query)
    {
        var funds = await fundRepo.SearchAsync(query);
        return funds.Select(MapToDto).ToList();
    }

    public async Task<FundSyncResultDto> SyncFundMasterListAsync(CancellationToken ct = default)
    {
        var sw = Stopwatch.StartNew();

        logger.LogInformation("Starting fund master list sync from AMFI...");
        var funds = await amfiService.FetchFundMasterListAsync(ct);

        var existingCount = await fundRepo.GetCountAsync();
        await fundRepo.BulkUpsertAsync(funds);
        var latestNavSnapshots = await amfiService.FetchLatestNavSnapshotsAsync(ct);
        await navRepo.UpsertLatestNavSnapshotsAsync(latestNavSnapshots, IndiaTime.Now(), ct);
        var newCount = await fundRepo.GetCountAsync();

        sw.Stop();

        var result = new FundSyncResultDto
        {
            TotalParsed = funds.Count,
            Added = newCount - existingCount,
            Updated = existingCount > 0 ? funds.Count - (newCount - existingCount) : 0,
            Unchanged = 0,
            Duration = sw.Elapsed
        };

        logger.LogInformation(
            "Fund sync complete: {Parsed} parsed, {Added} added, {Updated} updated in {Duration}",
            result.TotalParsed, result.Added, result.Updated, result.Duration);

        return result;
    }

    public async Task<int> GetFundCountAsync()
    {
        return await fundRepo.GetCountAsync();
    }

    private static FundDto MapToDto(Fund fund) => new()
    {
        SchemeCode = fund.SchemeCode,
        IsinGrowth = fund.IsinGrowth,
        IsinDivReinvestment = fund.IsinDivReinvestment,
        SchemeName = fund.SchemeName,
        LatestNav = fund.NavSyncTracker?.LatestNav,
        LatestNavDate = FormatDate(fund.NavSyncTracker?.LatestNavDate),
        NavStartDate = FormatDate(fund.NavSyncTracker?.NavStartDate),
        NavEndDate = FormatDate(fund.NavSyncTracker?.NavEndDate),
        LastSuccessfulNavCheckUtc = FormatIst(fund.NavSyncTracker?.LastHistoryCheckAtIst ?? fund.NavSyncTracker?.LastLatestNavCheckAtIst),
        NavStatus = GetNavStatus(fund.NavSyncTracker?.LatestNavDate, fund.NavSyncTracker?.LastLatestNavCheckAtIst),
        IsLikelyDiscontinued = IsLikelyDiscontinued(fund.NavSyncTracker?.LatestNavDate, fund.NavSyncTracker?.LastLatestNavCheckAtIst),
        LaunchDate = FormatDate(fund.LaunchDate),
        FundHouse = fund.FundHouse,
        SchemeType = fund.SchemeType,
        SchemeCategory = fund.SchemeCategory
    };

    private static string? FormatDate(DateOnly? date)
        => date?.ToString("dd-MM-yyyy");

    private static string? FormatIst(DateTime? dateTime)
        => dateTime?.ToString("O");

    private static string GetNavStatus(DateOnly? latestNavDate, DateTime? lastSuccessfulCheckUtc)
    {
        if (!latestNavDate.HasValue)
            return "Unknown";

        var today = IndiaTime.Today();
        if (today.DayNumber - latestNavDate.Value.DayNumber <= LikelyDiscontinuedGraceDays)
            return "Active";

        if (!lastSuccessfulCheckUtc.HasValue)
            return "NeedsCheck";

        return IsLikelyDiscontinued(latestNavDate, lastSuccessfulCheckUtc)
            ? "LikelyDiscontinued"
            : "NeedsCheck";
    }

    private static bool IsLikelyDiscontinued(DateOnly? latestNavDate, DateTime? lastSuccessfulCheckAtIst)
    {
        if (!latestNavDate.HasValue || !lastSuccessfulCheckAtIst.HasValue)
            return false;

        var lastCheckDate = DateOnly.FromDateTime(lastSuccessfulCheckAtIst.Value);
        return lastCheckDate.DayNumber - latestNavDate.Value.DayNumber > LikelyDiscontinuedGraceDays;
    }
}
