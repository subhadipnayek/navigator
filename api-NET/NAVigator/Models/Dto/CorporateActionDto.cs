namespace NAVigator.Models.Dto;

public class CorporateActionDto
{
    public int Id { get; set; }
    public int SchemeCode { get; set; }
    public required string ActionDate { get; set; }   // "dd-MM-yyyy"
    public required string ActionType { get; set; }    // Split, Merger, Bonus, Discontinuity, MissingData, Other
    public decimal AdjustmentFactor { get; set; }
    public string? Notes { get; set; }
}
