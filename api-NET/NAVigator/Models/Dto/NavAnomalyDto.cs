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
    public string? SuggestedActionType { get; set; }
    public decimal? SuggestedAdjustmentFactor { get; set; }
    public string? SuggestedNotes { get; set; }
    public decimal Confidence { get; set; }
    public string ReviewPriority { get; set; } = "Medium";
    public bool IsAutoRecordable { get; set; }
    public bool RequiresPreReview { get; set; }
}
