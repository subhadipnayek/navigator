namespace NAVigator.Models.Dto;

public record RegisterRequest(string Username, string Email, string Password);

public record LoginRequest(string UsernameOrEmail, string Password);

public record AuthUserDto(int Id, string Username, string Email, string Role);

public record AuthResponse(string Token, DateTime ExpiresAtUtc, AuthUserDto User);

public record UpdateUserRoleRequest(string Role);
