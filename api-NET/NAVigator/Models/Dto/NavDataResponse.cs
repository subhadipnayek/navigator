namespace NAVigator.Models.Dto;

public class NavDataResponse
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public string? LatestNavDate { get; set; }
    public string? NavStartDate { get; set; }
    public string? NavEndDate { get; set; }
    public string? LastSuccessfulNavCheckUtc { get; set; }
    public string NavStatus { get; set; } = "Unknown";
    public bool IsLikelyDiscontinued { get; set; }
    public bool WasFetchedFromApi { get; set; }
    public bool IsAdjusted { get; set; }
    public List<NavHistoryDto> Data { get; set; } = [];
    public List<CorporateActionDto> CorporateActions { get; set; } = [];
}
