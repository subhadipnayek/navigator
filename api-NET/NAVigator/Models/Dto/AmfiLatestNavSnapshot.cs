namespace NAVigator.Models.Dto;

public class AmfiLatestNavSnapshot
{
    public int SchemeCode { get; set; }
    public decimal LatestNav { get; set; }
    public DateOnly LatestNavDate { get; set; }
}
