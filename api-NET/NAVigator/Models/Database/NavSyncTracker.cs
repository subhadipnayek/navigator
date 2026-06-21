using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

/// <summary>
/// Per-fund NAV cache and sync state. The source of truth remains
/// Funds for scheme metadata and NavHistories for the series itself.
/// </summary>
[Table("NavSyncTrackers")]
public class NavSyncTracker
{
    [Key]
    [DatabaseGenerated(DatabaseGeneratedOption.None)]
    public int SchemeCode { get; set; }

    [Column(TypeName = "decimal(18,6)")]
    public decimal? LatestNav { get; set; }

    public DateOnly? LatestNavDate { get; set; }

    public DateOnly? NavStartDate { get; set; }

    public DateOnly? NavEndDate { get; set; }

    public int NavHistoryCount { get; set; }

    public int CorporateActionCount { get; set; }

    /// <summary>Last time NAVAll.txt was checked for the latest NAV snapshot, in IST.</summary>
    [Column(TypeName = "timestamp without time zone")]
    public DateTime? LastLatestNavCheckAtIst { get; set; }

    /// <summary>Last time AMFI historical NAV API was checked for this fund, in IST.</summary>
    [Column(TypeName = "timestamp without time zone")]
    public DateTime? LastHistoryCheckAtIst { get; set; }

    /// <summary>Last time this fund's stored NAV history cache was changed, in IST.</summary>
    [Column(TypeName = "timestamp without time zone")]
    public DateTime? NavHistoryUpdatedAtIst { get; set; }

    [Column(TypeName = "timestamp without time zone")]
    public DateTime UpdatedAtIst { get; set; }

    /// <summary>
    /// Whether the last historical API check returned newer data than what we already had.
    /// False means the API had nothing new, so we should cool down for this fund.
    /// </summary>
    public bool LastCheckHadNewData { get; set; }

    [ForeignKey(nameof(SchemeCode))]
    public Fund Fund { get; set; } = null!;
}
