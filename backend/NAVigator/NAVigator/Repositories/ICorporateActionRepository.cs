using NAVigator.Models.Database;

namespace NAVigator.Repositories;

public interface ICorporateActionRepository
{
    Task<List<CorporateAction>> GetBySchemeCodeAsync(int schemeCode);
    Task<CorporateAction?> GetByIdAsync(int id);
    Task<CorporateAction> AddAsync(CorporateAction action);
    Task DeleteAsync(CorporateAction action);

    /// <summary>
    /// Returns scheme codes that already have at least one corporate action recorded.
    /// </summary>
    Task<HashSet<int>> GetSchemeCodesWithActionsAsync();
}
