namespace NAVigator.Models.Dto;

public class NavDataResponse
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public string? LatestNavDate { get; set; }
    public bool WasFetchedFromApi { get; set; }
    public List<NavHistoryDto> Data { get; set; } = [];
}
