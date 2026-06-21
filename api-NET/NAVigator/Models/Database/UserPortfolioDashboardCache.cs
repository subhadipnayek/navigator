using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("UserPortfolioDashboardCaches")]
public class UserPortfolioDashboardCache
{
    public long Id { get; set; }

    public int UserId { get; set; }

    public long PortfolioId { get; set; }

    public bool IncludeFullyExitedFunds { get; set; }

    [MaxLength(128)]
    public required string DependencyKey { get; set; }

    public required string DashboardJson { get; set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;

    public DateTime UpdatedAtUtc { get; set; } = DateTime.UtcNow;

    [ForeignKey(nameof(UserId))]
    public AppUser User { get; set; } = null!;

    [ForeignKey(nameof(PortfolioId))]
    public UserPortfolio Portfolio { get; set; } = null!;
}
