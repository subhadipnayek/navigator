using NAVigator.Models.Dto.Portfolio;

namespace NAVigator.Services;

public interface IUserPortfolioService
{
    Task<List<UserPortfolioSummaryDto>> GetPortfoliosAsync(int userId, CancellationToken ct);
    Task<UserPortfolioSummaryDto> CreatePortfolioAsync(int userId, CreateUserPortfolioRequest request, CancellationToken ct);
    Task<PortfolioDashboardDto?> GetDashboardAsync(int userId, long portfolioId, bool showFullyExitedFunds, CancellationToken ct);
    Task<(List<PortfolioTransactionDto> Rows, YearTransactionSummaryDto Summary)?> GetTransactionsAsync(int userId, long portfolioId, string financialYear, CancellationToken ct);
    Task<SavePortfolioTransactionsResponse?> SaveTransactionsAsync(int userId, long portfolioId, SavePortfolioTransactionsRequest request, CancellationToken ct);
    Task<int?> GetImportDeletePreviewAsync(int userId, long portfolioId, DateOnly startDate, DateOnly endDate, CancellationToken ct);
    Task<ImportPortfolioTransactionsResponse?> ImportTransactionsAsync(int userId, long portfolioId, string financialYear, string importMode, DateOnly startDate, DateOnly endDate, Stream fileStream, string fileName, CancellationToken ct);
    Task<PortfolioBenchmarkResponse?> CompareBenchmarkAsync(int userId, long portfolioId, PortfolioBenchmarkRequest request, CancellationToken ct);
}
