using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public interface IAmfiService
{
    /// <summary>Fetches and parses the AMFI NAVAll.txt master list.</summary>
    Task<List<Fund>> FetchFundMasterListAsync(CancellationToken ct = default);

    /// <summary>Fetches NAV history for a specific scheme from the mfapi.in API.</summary>
    Task<MfApiResponse?> FetchNavHistoryAsync(int schemeCode, CancellationToken ct = default);
}
