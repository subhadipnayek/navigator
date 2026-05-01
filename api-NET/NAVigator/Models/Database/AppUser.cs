using System.ComponentModel.DataAnnotations;
using System.ComponentModel.DataAnnotations.Schema;

namespace NAVigator.Models.Database;

[Table("AppUsers")]
public class AppUser
{
    public int Id { get; set; }

    [MaxLength(80)]
    public required string Username { get; set; }

    [MaxLength(80)]
    public required string NormalizedUsername { get; set; }

    [MaxLength(256)]
    public required string Email { get; set; }

    [MaxLength(256)]
    public required string NormalizedEmail { get; set; }

    [MaxLength(500)]
    public required string PasswordHash { get; set; }

    [MaxLength(30)]
    public required string Role { get; set; }

    public DateTime CreatedAtUtc { get; set; } = DateTime.UtcNow;
}
