using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("CorporateActions")]
public class CorporateAction
{
    [Key]
    public int Id { get; set; }

    public int SchemeCode { get; set; }

    public DateOnly ActionDate { get; set; }

    /// <summary>
    /// Type of corporate action: Split, Merger, Bonus, Other.
    /// </summary>
    [MaxLength(50)]
    public required string ActionType { get; set; }

    /// <summary>
    /// Adjustment factor to normalize NAVs across the action.
    /// For a 10:1 split (FV 10 → 1), factor = 10 (pre-split NAVs are divided by 10).
    /// For a 1:5 reverse split (FV 1 → 5), factor = 0.2.
    /// For a 1:1 bonus, factor = 2.
    /// </summary>
    [Column(TypeName = "decimal(18,10)")]
    public decimal AdjustmentFactor { get; set; }

    [Column(TypeName = "decimal(18,6)")]
    public decimal? OldFaceValue { get; set; }

    [Column(TypeName = "decimal(18,6)")]
    public decimal? NewFaceValue { get; set; }

    [MaxLength(500)]
    public string? Notes { get; set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;

    [ForeignKey(nameof(SchemeCode))]
    public Fund Fund { get; set; } = null!;
}
