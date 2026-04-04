using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("Funds")]
public class Fund
{
    [Key]
    [DatabaseGenerated(DatabaseGeneratedOption.None)]
    public int SchemeCode { get; set; }

    [MaxLength(20)]
    public string? IsinGrowth { get; set; }

    [MaxLength(20)]
    public string? IsinDivReinvestment { get; set; }

    [MaxLength(500)]
    public required string SchemeName { get; set; }

    [Column(TypeName = "decimal(18,6)")]
    public decimal? LatestNav { get; set; }

    public DateOnly? LatestNavDate { get; set; }

    [MaxLength(300)]
    public string? FundHouse { get; set; }

    [MaxLength(200)]
    public string? SchemeType { get; set; }

    [MaxLength(300)]
    public string? SchemeCategory { get; set; }

    public DateTime CreatedAt { get; set; } = DateTime.UtcNow;
    public DateTime UpdatedAt { get; set; } = DateTime.UtcNow;

    public ICollection<NavHistory> NavHistories { get; set; } = [];
    public ICollection<CorporateAction> CorporateActions { get; set; } = [];
    public NavSyncTracker? NavSyncTracker { get; set; }
}
