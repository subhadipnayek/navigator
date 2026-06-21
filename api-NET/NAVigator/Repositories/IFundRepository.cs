using NAVigator.Models.Database;

namespace NAVigator.Repositories;

public interface IFundRepository
{
    Task<List<Fund>> GetAllAsync();
    Task<Fund?> GetBySchemeCodeAsync(int schemeCode);
    Task<List<Fund>> SearchAsync(string query, int maxResults = 50);
    Task BulkUpsertAsync(List<Fund> funds);
    Task<int> GetCountAsync();
}
