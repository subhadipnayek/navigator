using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;
using Microsoft.EntityFrameworkCore;

namespace NAVigator.Models.Database;

[Table("NavHistories")]
[Index(nameof(SchemeCode), nameof(Date), IsUnique = true)]
public class NavHistory
{
    [Key]
    public long Id { get; set; }

    public int SchemeCode { get; set; }

    public DateOnly Date { get; set; }

    [Column(TypeName = "decimal(18,6)")]
    public decimal Nav { get; set; }

    [ForeignKey(nameof(SchemeCode))]
    public Fund Fund { get; set; } = null!;
}
