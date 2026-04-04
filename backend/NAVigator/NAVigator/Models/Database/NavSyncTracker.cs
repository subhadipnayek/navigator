using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

/// <summary>
/// Tracks per-fund NAV sync attempts to avoid hammering the external API.
/// If we checked the API and it had no newer NAV, we wait at least
/// <see cref="CooldownMinutes"/> before checking again.
/// </summary>
[Table("NavSyncTrackers")]
public class NavSyncTracker
{
    [Key]
    [DatabaseGenerated(DatabaseGeneratedOption.None)]
    public int SchemeCode { get; set; }

    /// <summary>Last time we called the external NAV API for this fund.</summary>
    public DateTime LastApiCheckUtc { get; set; }

    /// <summary>The latest NAV date that was available on the API during the last check.</summary>
    public DateOnly? LatestNavDateFromApi { get; set; }

    /// <summary>
    /// Whether the last API check returned newer data than what we already had.
    /// False means the API had nothing new — so we should cool down.
    /// </summary>
    public bool LastCheckHadNewData { get; set; }

    [ForeignKey(nameof(SchemeCode))]
    public Fund Fund { get; set; } = null!;
}
