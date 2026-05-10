namespace NAVigator.Models.Dto.Portfolio;

public record UserPortfolioSummaryDto(long Id, string Name, int FundCount, DateTime UpdatedAtUtc);

public record CreateUserPortfolioRequest(string Name);

public record PortfolioTransactionDto(
    long? Id,
    string? ClientRowId,
    int SchemeCode,
    string SchemeName,
    string TransactionDate,
    string TransactionType,
    decimal Units,
    decimal Nav,
    decimal Amount,
    string? Notes,
    string? Error);

public record SavePortfolioTransactionsRequest(string FinancialYear, List<PortfolioTransactionUpsertDto> Transactions);

public record PortfolioTransactionUpsertDto(
    long? Id,
    string? ClientRowId,
    int SchemeCode,
    string SchemeName,
    string TransactionDate,
    string TransactionType,
    decimal Units,
    decimal Nav,
    decimal Amount,
    string? Notes);

public record SavePortfolioTransactionsResponse(
    bool IsValid,
    string? Message,
    int SavedCount,
    List<PortfolioTransactionDto> Transactions,
    YearTransactionSummaryDto Summary);

public record ImportPortfolioTransactionsResponse(
    bool IsValid,
    string? Message,
    int TotalRows,
    int ImportedCount,
    int ReplacedCount,
    int SkippedOutOfRangeCount,
    string? ImportedDateStart,
    string? ImportedDateEnd,
    decimal TotalAmountImported,
    int FundCount,
    List<string> RowErrors,
    List<string> RowWarnings,
    List<PortfolioTransactionDto> Transactions,
    YearTransactionSummaryDto Summary);

public record ImportDeletePreviewResponse(int DeleteCount);

public record YearTransactionSummaryDto(decimal TotalInvested, int TransactionCount, int FundsCovered);

public record PortfolioDashboardDto(
    long PortfolioId,
    string PortfolioName,
    PortfolioMetricsDto Metrics,
    List<PortfolioFundInsightDto> Funds,
    List<AllocationSliceDto> AssetAllocation,
    List<AllocationSliceDto> FundAllocation,
    List<ContributionPeriodDto> FinancialYearContributions,
    List<ContributionPeriodDto> CalendarYearContributions,
    SipConsistencyDto SipConsistency,
    List<string> EmptyStateMessages);

public record PortfolioMetricsDto(
    decimal InvestedAmount,
    decimal CurrentValue,
    decimal RealizedPnl,
    decimal UnrealizedPnl,
    decimal AbsoluteReturnPercent,
    decimal? CagrPercent,
    decimal? XirrPercent,
    int FundCount,
    int ActiveFundCount,
    int FullyExitedFundCount);

public record PortfolioFundInsightDto(
    int SchemeCode,
    string SchemeName,
    string? FundHouse,
    string? SchemeCategory,
    string AssetClass,
    decimal Units,
    decimal InvestedAmount,
    decimal CurrentValue,
    decimal RealizedPnl,
    decimal UnrealizedPnl,
    decimal AbsoluteReturnPercent,
    decimal? CagrPercent,
    decimal? XirrPercent,
    int HoldingPeriodDays,
    bool IsFullyExited,
    decimal? LatestNav,
    string? LatestNavDate,
    bool HasNav);

public record AllocationSliceDto(string Label, decimal Value, decimal Percent);

public record ContributionPeriodDto(string Period, decimal Invested, decimal Redemptions, decimal NetContribution);

public record SipConsistencyDto(int ActiveMonths, int InvestedMonths, decimal ConsistencyPercent, decimal AverageMonthlyInvestment);

public record PortfolioBenchmarkRequest(string BenchmarkType, int? SchemeCode, BasketDefinition? Basket);

public record PortfolioBenchmarkResponse(
    string BenchmarkName,
    PortfolioBenchmarkMetricDto Portfolio,
    PortfolioBenchmarkMetricDto Benchmark,
    decimal? AlphaPercent,
    string Verdict);

public record PortfolioBenchmarkMetricDto(
    decimal InvestedAmount,
    decimal CurrentValue,
    decimal AbsoluteReturnPercent,
    decimal? XirrPercent);

public record UserGridPreferenceDto(string GridKey, string? StateJson);

public record SaveUserGridPreferenceRequest(string StateJson);

public record MatchPortfolioFundRequest(List<MatchPortfolioFundRowDto> Rows);

public record MatchPortfolioFundRowDto(
    int RowNumber,
    string? Isin,
    string? SchemeCode,
    string? SchemeName);

public record MatchPortfolioFundResponse(List<MatchedPortfolioFundDto> Rows);

public record MatchedPortfolioFundDto(
    int RowNumber,
    int? SchemeCode,
    string? SchemeName,
    string? Error);
