namespace NAVigator.Models.Dto;

public class RollingReturnResponse
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public int PeriodInYears { get; set; }
    public int DataPoints { get; set; }
    public decimal? MinReturn { get; set; }
    public decimal? MaxReturn { get; set; }
    public decimal? AverageReturn { get; set; }
    public decimal? MedianReturn { get; set; }
    public List<RollingReturnDto> Data { get; set; } = [];
}
