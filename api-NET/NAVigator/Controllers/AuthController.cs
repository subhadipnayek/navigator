using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.Mvc;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Dto;
using NAVigator.Services;

namespace NAVigator.Controllers;

[ApiController]
[Route("api/[controller]")]
public class AuthController(IAuthService authService, AppDbContext db) : ControllerBase
{
    private static readonly string[] ValidRoles = ["Admin", "User"];

    [HttpPost("register")]
    [AllowAnonymous]
    public async Task<IActionResult> Register([FromBody] RegisterRequest request, CancellationToken ct)
    {
        var validationError = ValidateRegister(request);
        if (validationError is not null)
            return BadRequest(new { message = validationError });

        var result = await authService.RegisterAsync(request, ct);
        return result is null
            ? Conflict(new { message = "Username or email is already registered." })
            : Ok(result);
    }

    [HttpPost("login")]
    [AllowAnonymous]
    public async Task<IActionResult> Login([FromBody] LoginRequest request, CancellationToken ct)
    {
        if (string.IsNullOrWhiteSpace(request.UsernameOrEmail) || string.IsNullOrWhiteSpace(request.Password))
            return BadRequest(new { message = "Username/email and password are required." });

        var result = await authService.LoginAsync(request, ct);
        return result is null
            ? Unauthorized(new { message = "Invalid username/email or password." })
            : Ok(result);
    }

    [HttpGet("me")]
    [Authorize]
    public IActionResult Me()
    {
        var id = int.TryParse(User.FindFirst(System.Security.Claims.ClaimTypes.NameIdentifier)?.Value, out var userId)
            ? userId
            : 0;

        return Ok(new AuthUserDto(
            id,
            User.Identity?.Name ?? string.Empty,
            User.FindFirst(System.Security.Claims.ClaimTypes.Email)?.Value ?? string.Empty,
            User.FindFirst(System.Security.Claims.ClaimTypes.Role)?.Value ?? string.Empty));
    }

    [HttpGet("users")]
    [Authorize(Roles = "Admin")]
    public async Task<IActionResult> GetUsers(CancellationToken ct)
    {
        var users = await db.Users
            .OrderBy(u => u.Username)
            .Select(u => new AuthUserDto(u.Id, u.Username, u.Email, u.Role))
            .ToListAsync(ct);

        return Ok(users);
    }

    [HttpPut("users/{userId:int}/role")]
    [Authorize(Roles = "Admin")]
    public async Task<IActionResult> UpdateRole(
        int userId,
        [FromBody] UpdateUserRoleRequest request,
        CancellationToken ct)
    {
        if (!ValidRoles.Contains(request.Role, StringComparer.OrdinalIgnoreCase))
            return BadRequest(new { message = "Role must be 'Admin' or 'User'." });

        var user = await db.Users.FindAsync([userId], ct);
        if (user is null)
            return NotFound(new { message = $"User {userId} was not found." });

        user.Role = ValidRoles.Single(r => r.Equals(request.Role, StringComparison.OrdinalIgnoreCase));
        await db.SaveChangesAsync(ct);

        return Ok(new AuthUserDto(user.Id, user.Username, user.Email, user.Role));
    }

    private static string? ValidateRegister(RegisterRequest request)
    {
        if (string.IsNullOrWhiteSpace(request.Username) || request.Username.Trim().Length < 3)
            return "Username must be at least 3 characters.";

        if (string.IsNullOrWhiteSpace(request.Email) || !request.Email.Contains('@'))
            return "A valid email is required.";

        if (string.IsNullOrWhiteSpace(request.Password) || request.Password.Length < 8)
            return "Password must be at least 8 characters.";

        return null;
    }
}
