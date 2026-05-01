using NAVigator.Models.Database;

namespace NAVigator.Services;

public interface IPasswordHasher
{
    string HashPassword(string password);
    bool VerifyPassword(AppUser user, string password);
}
