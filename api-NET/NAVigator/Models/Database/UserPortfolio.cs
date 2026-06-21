using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("UserPortfolios")]
public class UserPortfolio
{
    public long Id { get; set; }

    public int UserId { get; set; }

    [MaxLength(120)]
    public required string Name { get; set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public DateTime UpdatedAtUtc { get; set; } = DateTime.UtcNow;

    [ForeignKey(nameof(UserId))]
    public AppUser User { get; set; } = null!;

    public ICollection<PortfolioTransaction> Transactions { get; set; } = [];
}
