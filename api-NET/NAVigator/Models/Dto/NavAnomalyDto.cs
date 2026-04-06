namespace NAVigator.Models.Dto;

public class NavAnomalyDto
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public required string Date { get; set; }
    public required string PreviousDate { get; set; }
    public int DaysSincePrevious { get; set; }
    public decimal PreviousNav { get; set; }
    public decimal CurrentNav { get; set; }
    public decimal ChangePercent { get; set; }
    public required string Direction { get; set; }  // "Drop" or "Spike"
    public bool HasCorporateAction { get; set; }
}
