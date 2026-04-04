using NAVigator.Models.Dto;

namespace NAVigator.Services;

public interface IFundService
{
    Task<List<FundDto>> GetAllFundsAsync();
    Task<FundDto?> GetFundAsync(int schemeCode);
    Task<List<FundDto>> SearchFundsAsync(string query);
    Task<FundSyncResultDto> SyncFundMasterListAsync(CancellationToken ct = default);
    Task<int> GetFundCountAsync();
}
