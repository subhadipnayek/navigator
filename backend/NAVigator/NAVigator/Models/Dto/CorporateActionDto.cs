namespace NAVigator.Models.Dto;

public class CorporateActionDto
{
    public int Id { get; set; }
    public int SchemeCode { get; set; }
    public required string ActionDate { get; set; }   // "dd-MM-yyyy"
    public required string ActionType { get; set; }    // Split, Merger, Bonus, Other
    public decimal AdjustmentFactor { get; set; }
    public decimal? OldFaceValue { get; set; }
    public decimal? NewFaceValue { get; set; }
    public string? Notes { get; set; }
}
