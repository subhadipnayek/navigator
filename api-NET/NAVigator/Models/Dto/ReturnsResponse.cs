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

    /// <summary>
    /// True if a data discontinuity exists between start and end dates.
    /// When true, the start date was clamped to the post-discontinuity period.
    /// </summary>
    public bool HasDiscontinuity { get; set; }
}
