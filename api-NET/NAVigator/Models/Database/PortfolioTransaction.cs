using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("PortfolioTransactions")]
public class PortfolioTransaction
{
    public long Id { get; set; }

    public long PortfolioId { get; set; }

    public int SchemeCode { get; set; }

    public DateOnly TransactionDate { get; set; }

    [MaxLength(20)]
    public required string TransactionType { get; set; }

    [Column(TypeName = "decimal(18,6)")]
    public decimal Units { get; set; }

    [Column(TypeName = "decimal(18,4)")]
    public decimal Nav { get; set; }

    [Column(TypeName = "decimal(18,2)")]
    public decimal Amount { get; set; }

    [MaxLength(240)]
    public string? Notes { get; set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public DateTime UpdatedAtUtc { get; set; } = DateTime.UtcNow;

    [ForeignKey(nameof(PortfolioId))]
    public UserPortfolio Portfolio { get; set; } = null!;

    [ForeignKey(nameof(SchemeCode))]
    public Fund Fund { get; set; } = null!;
}
