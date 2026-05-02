using System.ComponentModel.DataAnnotations;

namespace NAVigator.Models.Dto;

public class CreateCorporateActionRequest
{
    [Required]
    public DateOnly ActionDate { get; set; }

    /// <summary>Split, Merger, Bonus, Discontinuity, MissingData, Other</summary>
    [Required]
    [MaxLength(50)]
    public required string ActionType { get; set; }

    /// <summary>
    /// Factor to adjust pre-action NAVs. E.g. 10 for a 10:1 split, 0.2 for a 1:5 reverse split.
    /// Ignored for series-break actions such as Discontinuity and MissingData.
    /// </summary>
    [Required]
    [Range(0.0001, 10000)]
    public decimal AdjustmentFactor { get; set; }

    [MaxLength(500)]
    public string? Notes { get; set; }
}
