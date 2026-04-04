using System.Diagnostics;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;
using NAVigator.Repositories;

namespace NAVigator.Services;

public class FundService(
    IFundRepository fundRepo,
    IAmfiService amfiService,
    ILogger<FundService> logger) : IFundService
{
    public async Task<List<FundDto>> GetAllFundsAsync()
    {
        var funds = await fundRepo.GetAllAsync();
        return funds.Select(MapToDto).ToList();
    }

    public async Task<FundDto?> GetFundAsync(int schemeCode)
    {
        var fund = await fundRepo.GetBySchemeCodeAsync(schemeCode);
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
        LatestNav = fund.LatestNav,
        LatestNavDate = fund.LatestNavDate?.ToString("dd-MMM-yyyy"),
        FundHouse = fund.FundHouse,
        SchemeType = fund.SchemeType,
        SchemeCategory = fund.SchemeCategory
    };
}
