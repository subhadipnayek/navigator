using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public interface IAmfiService
{
    /// <summary>Fetches and parses the AMFI scheme master list export.</summary>
    Task<List<Fund>> FetchFundMasterListAsync(CancellationToken ct = default);

    /// <summary>Fetches and parses the NAVAll.txt latest NAV snapshot for all schemes.</summary>
    Task<List<AmfiLatestNavSnapshot>> FetchLatestNavSnapshotsAsync(CancellationToken ct = default);

    /// <summary>Fetches NAV history for a specific scheme from AMFI historical NAV API.</summary>
    Task<MfApiResponse?> FetchNavHistoryAsync(
        int schemeCode,
        DateOnly from,
        DateOnly to,
        CancellationToken ct = default,
        bool stopAtFirstOlderEmptyChunk = true);
}
