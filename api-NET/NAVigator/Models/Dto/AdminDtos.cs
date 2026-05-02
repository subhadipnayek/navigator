namespace NAVigator.Models.Dto;

public class AdminDatabaseSummaryDto
{
    public int TotalFunds { get; set; }
    public int FundsWithNavHistory { get; set; }
    public int FundsWithoutNavHistory { get; set; }
    public long NavHistoryRecords { get; set; }
    public int CorporateActions { get; set; }
    public string? OldestNavDate { get; set; }
    public string? LatestNavDate { get; set; }
}

public class AdminFundStorageDto
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public string? SchemeType { get; set; }
    public string? SchemeCategory { get; set; }
    public decimal? LatestNav { get; set; }
    public string? LatestNavDate { get; set; }
    public int NavHistoryCount { get; set; }
    public string? FirstStoredNavDate { get; set; }
    public string? LastStoredNavDate { get; set; }
    public int CorporateActionCount { get; set; }
    public bool HasNavHistory => NavHistoryCount > 0;
}

public class PagedResponseDto<T>
{
    public int Page { get; set; }
    public int PageSize { get; set; }
    public int TotalItems { get; set; }
    public int TotalPages { get; set; }
    public List<T> Items { get; set; } = [];
}
