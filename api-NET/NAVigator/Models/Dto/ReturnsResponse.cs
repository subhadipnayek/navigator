namespace NAVigator.Models.Dto;

public class ReturnsResponse
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public decimal? AbsoluteReturn { get; set; }
    public decimal? Cagr { get; set; }
    public string? StartDate { get; set; }
    public decimal? StartNav { get; set; }
    public string? EndDate { get; set; }
    public decimal? EndNav { get; set; }
}
