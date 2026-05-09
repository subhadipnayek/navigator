using System.Text;
using System.Threading.RateLimiting;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.Authorization;
using Microsoft.AspNetCore.RateLimiting;
using Microsoft.EntityFrameworkCore;
using Microsoft.IdentityModel.Tokens;
using NAVigator.Data;
using NAVigator.Repositories;
using NAVigator.Services;
using Npgsql.EntityFrameworkCore.PostgreSQL;
using Polly;
using Polly.Extensions.Http;
using EFCore.NamingConventions;

var builder = WebApplication.CreateBuilder(args);

// ----- EF Core -----
builder.Services.AddDbContext<AppDbContext>(options =>
    options.UseNpgsql(builder.Configuration.GetConnectionString("DefaultConnection"))
           .UseSnakeCaseNamingConvention());

// ----- Repositories -----
builder.Services.AddScoped<IFundRepository, FundRepository>();
builder.Services.AddScoped<INavHistoryRepository, NavHistoryRepository>();
builder.Services.AddScoped<ICorporateActionRepository, CorporateActionRepository>();

// ----- Services -----
builder.Services.AddScoped<IAmfiService, AmfiService>();
builder.Services.AddScoped<IFundService, FundService>();
builder.Services.AddScoped<INavService, NavService>();
builder.Services.AddScoped<ICorporateActionService, CorporateActionService>();
builder.Services.AddScoped<IPortfolioSimulationService, PortfolioSimulationService>();
builder.Services.AddScoped<IUserPortfolioService, UserPortfolioService>();
builder.Services.AddScoped<IAuthService, AuthService>();
builder.Services.AddSingleton<IPasswordHasher, Pbkdf2PasswordHasher>();

// ----- HTTP Clients with Polly retry -----
var retryPolicy = HttpPolicyExtensions
    .HandleTransientHttpError()
    .WaitAndRetryAsync(3, attempt => TimeSpan.FromSeconds(Math.Pow(2, attempt)));

builder.Services.AddHttpClient("Amfi", client =>
{
    client.Timeout = TimeSpan.FromMinutes(2);
}).AddPolicyHandler(retryPolicy);

builder.Services.AddHttpClient("AmfiHistory", client =>
{
    client.Timeout = TimeSpan.FromSeconds(30);
}).ConfigurePrimaryHttpMessageHandler(() => new HttpClientHandler
{
    AutomaticDecompression = System.Net.DecompressionMethods.All
}).AddPolicyHandler(retryPolicy);

// builder.Services.AddHostedService<NavSyncWorker>();

// ----- Controllers & OpenAPI -----
builder.Services.AddControllers();
builder.Services.AddOpenApi();

var jwtKey = builder.Configuration["Jwt:Key"];
if (string.IsNullOrWhiteSpace(jwtKey))
    throw new InvalidOperationException("JWT signing key is not configured.");

builder.Services
    .AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(options =>
    {
        options.TokenValidationParameters = new TokenValidationParameters
        {
            ValidateIssuer = true,
            ValidateAudience = true,
            ValidateLifetime = true,
            ValidateIssuerSigningKey = true,
            ValidIssuer = builder.Configuration["Jwt:Issuer"] ?? "NAVigator",
            ValidAudience = builder.Configuration["Jwt:Audience"] ?? "NAVigator",
            IssuerSigningKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes(jwtKey)),
            ClockSkew = TimeSpan.FromMinutes(2)
        };
    });

builder.Services.AddAuthorization(options =>
{
    options.FallbackPolicy = new AuthorizationPolicyBuilder()
        .RequireAuthenticatedUser()
        .Build();
});

builder.Services.AddRateLimiter(options =>
{
    options.RejectionStatusCode = StatusCodes.Status429TooManyRequests;
    options.AddFixedWindowLimiter("login", limiter =>
    {
        limiter.PermitLimit = 5;
        limiter.Window = TimeSpan.FromMinutes(1);
        limiter.QueueProcessingOrder = QueueProcessingOrder.OldestFirst;
        limiter.QueueLimit = 0;
    });
});

builder.Services.AddCors(options =>
{
    options.AddPolicy("CorsPolicy", policy =>
    {
        policy.WithOrigins(
                "http://localhost:4200",  // Angular dev server
                "http://127.0.0.1:4200",
                "http://localhost:4201",
                "http://127.0.0.1:4201",
                "http://localhost:4000"   // Docker UI container
              )
              .AllowAnyHeader()
              .AllowAnyMethod();
    });
});

var app = builder.Build();

// ----- Auto-migrate database on startup (unconditional for Docker QA support) -----
using var scope = app.Services.CreateScope();
var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
var logger = scope.ServiceProvider.GetRequiredService<ILogger<Program>>();
var pendingMigrations = (await db.Database.GetPendingMigrationsAsync()).ToList();
if (pendingMigrations.Count > 0)
{
    logger.LogInformation(
        "Applying {Count} pending EF migration(s): {Migrations}",
        pendingMigrations.Count,
        string.Join(", ", pendingMigrations));
}
else
{
    logger.LogInformation("Database is up to date. No pending EF migrations found.");
}

await db.Database.MigrateAsync();

// Backfill cached counts if they haven't been populated yet
await db.Database.ExecuteSqlRawAsync(@"
    UPDATE ""NavSyncTrackers"" t SET
        nav_history_count = sub.cnt,
        corporate_action_count = sub.ca_cnt
    FROM (
        SELECT t2.scheme_code,
               (SELECT COUNT(*) FROM ""NavHistories"" n WHERE n.scheme_code = t2.scheme_code) AS cnt,
               (SELECT COUNT(*) FROM ""CorporateActions"" ca WHERE ca.scheme_code = t2.scheme_code) AS ca_cnt
        FROM ""NavSyncTrackers"" t2
        WHERE t2.nav_history_count = 0
    ) sub
    WHERE t.scheme_code = sub.scheme_code
      AND t.nav_history_count = 0");

app.MapOpenApi();

if (!app.Environment.IsDevelopment())
{
    app.UseHttpsRedirection();
}

app.UseCors("CorsPolicy");
app.UseRateLimiter();
app.UseAuthentication();
app.UseAuthorization();
app.MapControllers();

app.Run();
