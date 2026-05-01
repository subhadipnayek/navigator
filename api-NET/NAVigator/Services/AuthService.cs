using System.IdentityModel.Tokens.Jwt;
using System.Security.Claims;
using System.Text;
using Microsoft.EntityFrameworkCore;
using Microsoft.IdentityModel.Tokens;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public class AuthService(
    AppDbContext db,
    IConfiguration configuration,
    IPasswordHasher passwordHasher) : IAuthService
{
    public async Task<AuthResponse?> RegisterAsync(RegisterRequest request, CancellationToken ct)
    {
        var username = request.Username.Trim();
        var email = request.Email.Trim();
        var normalizedUsername = Normalize(username);
        var normalizedEmail = Normalize(email);

        var exists = await db.Users.AnyAsync(
            u => u.NormalizedUsername == normalizedUsername || u.NormalizedEmail == normalizedEmail,
            ct);

        if (exists)
            return null;

        var isFirstUser = !await db.Users.AnyAsync(ct);
        var user = new AppUser
        {
            Username = username,
            NormalizedUsername = normalizedUsername,
            Email = email,
            NormalizedEmail = normalizedEmail,
            PasswordHash = passwordHasher.HashPassword(request.Password),
            Role = isFirstUser ? "Admin" : "User"
        };

        db.Users.Add(user);
        await db.SaveChangesAsync(ct);

        return CreateResponse(user);
    }

    public async Task<AuthResponse?> LoginAsync(LoginRequest request, CancellationToken ct)
    {
        var normalized = Normalize(request.UsernameOrEmail);
        var user = await db.Users.SingleOrDefaultAsync(
            u => u.NormalizedUsername == normalized || u.NormalizedEmail == normalized,
            ct);

        if (user is null || !passwordHasher.VerifyPassword(user, request.Password))
            return null;

        return CreateResponse(user);
    }

    private AuthResponse CreateResponse(AppUser user)
    {
        var key = configuration["Jwt:Key"];
        if (string.IsNullOrWhiteSpace(key))
            throw new InvalidOperationException("JWT signing key is not configured.");

        var issuer = configuration["Jwt:Issuer"] ?? "NAVigator";
        var audience = configuration["Jwt:Audience"] ?? "NAVigator";
        var expiresAtUtc = DateTime.UtcNow.AddHours(GetExpiryHours());

        var claims = new[]
        {
            new Claim(JwtRegisteredClaimNames.Sub, user.Id.ToString()),
            new Claim(JwtRegisteredClaimNames.UniqueName, user.Username),
            new Claim(JwtRegisteredClaimNames.Email, user.Email),
            new Claim(ClaimTypes.Email, user.Email),
            new Claim(ClaimTypes.NameIdentifier, user.Id.ToString()),
            new Claim(ClaimTypes.Name, user.Username),
            new Claim(ClaimTypes.Role, user.Role)
        };

        var credentials = new SigningCredentials(
            new SymmetricSecurityKey(Encoding.UTF8.GetBytes(key)),
            SecurityAlgorithms.HmacSha256);

        var token = new JwtSecurityToken(
            issuer: issuer,
            audience: audience,
            claims: claims,
            expires: expiresAtUtc,
            signingCredentials: credentials);

        return new AuthResponse(
            new JwtSecurityTokenHandler().WriteToken(token),
            expiresAtUtc,
            new AuthUserDto(user.Id, user.Username, user.Email, user.Role));
    }

    private int GetExpiryHours() =>
        int.TryParse(configuration["Jwt:ExpiresHours"], out var hours) && hours > 0 ? hours : 8;

    private static string Normalize(string value) => value.Trim().ToUpperInvariant();
}
