namespace NAVigator.Models.Dto;

public class FundDto
{
    public int SchemeCode { get; set; }
    public string? IsinGrowth { get; set; }
    public string? IsinDivReinvestment { get; set; }
    public required string SchemeName { get; set; }
    public decimal? LatestNav { get; set; }
    public string? LatestNavDate { get; set; }
    public string? FundHouse { get; set; }
    public string? SchemeType { get; set; }
    public string? SchemeCategory { get; set; }
}
