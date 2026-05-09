using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("UserGridPreferences")]
public class UserGridPreference
{
    public long Id { get; set; }

    public int UserId { get; set; }

    [MaxLength(120)]
    public required string GridKey { get; set; }

    [Column(TypeName = "jsonb")]
    public required string StateJson { get; set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
    public DateTime UpdatedAtUtc { get; set; } = DateTime.UtcNow;

    [ForeignKey(nameof(UserId))]
    public AppUser User { get; set; } = null!;
}
