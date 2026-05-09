using Microsoft.EntityFrameworkCore;
using NAVigator.Models.Database;

namespace NAVigator.Data;

public class AppDbContext(DbContextOptions<AppDbContext> options) : DbContext(options)
{
    public DbSet<AppUser> Users => Set<AppUser>();
    public DbSet<Fund> Funds => Set<Fund>();
    public DbSet<NavHistory> NavHistories => Set<NavHistory>();
    public DbSet<NavSyncTracker> NavSyncTrackers => Set<NavSyncTracker>();
    public DbSet<CorporateAction> CorporateActions => Set<CorporateAction>();
    public DbSet<UserPortfolio> UserPortfolios => Set<UserPortfolio>();
    public DbSet<PortfolioTransaction> PortfolioTransactions => Set<PortfolioTransaction>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        base.OnModelCreating(modelBuilder);

        modelBuilder.HasPostgresExtension("pg_trgm");

        modelBuilder.Entity<Fund>(e =>
        {
            e.HasMany(f => f.NavHistories)
             .WithOne(n => n.Fund)
             .HasForeignKey(n => n.SchemeCode)
             .OnDelete(DeleteBehavior.Cascade);

            e.HasOne(f => f.NavSyncTracker)
             .WithOne(t => t.Fund)
             .HasForeignKey<NavSyncTracker>(t => t.SchemeCode)
             .OnDelete(DeleteBehavior.Cascade);

            e.HasMany(f => f.CorporateActions)
             .WithOne(ca => ca.Fund)
             .HasForeignKey(ca => ca.SchemeCode)
             .OnDelete(DeleteBehavior.Cascade);

            // Add GIN indexes with pg_trgm operator classes to accelerate ILIKE "%query%" searches
            e.HasIndex(f => f.SchemeName)
             .HasMethod("gin")
             .HasOperators("gin_trgm_ops");

            e.HasIndex(f => f.FundHouse)
             .HasMethod("gin")
             .HasOperators("gin_trgm_ops");
        });

        modelBuilder.Entity<NavHistory>(e =>
        {
            e.HasIndex(n => n.SchemeCode);
            e.HasIndex(n => new { n.SchemeCode, n.Date }).IsUnique();
        });

        modelBuilder.Entity<CorporateAction>(e =>
        {
            e.HasIndex(ca => ca.SchemeCode);
            e.HasIndex(ca => new { ca.SchemeCode, ca.ActionDate });
        });

        modelBuilder.Entity<AppUser>(e =>
        {
            e.HasIndex(u => u.NormalizedUsername).IsUnique();
            e.HasIndex(u => u.NormalizedEmail).IsUnique();

            e.HasMany<UserPortfolio>()
             .WithOne(p => p.User)
             .HasForeignKey(p => p.UserId)
             .OnDelete(DeleteBehavior.Cascade);
        });

        modelBuilder.Entity<UserPortfolio>(e =>
        {
            e.HasIndex(p => new { p.UserId, p.Name }).IsUnique();

            e.HasMany(p => p.Transactions)
             .WithOne(t => t.Portfolio)
             .HasForeignKey(t => t.PortfolioId)
             .OnDelete(DeleteBehavior.Cascade);
        });

        modelBuilder.Entity<PortfolioTransaction>(e =>
        {
            e.HasIndex(t => new { t.PortfolioId, t.TransactionDate });
            e.HasIndex(t => new { t.PortfolioId, t.SchemeCode, t.TransactionDate });

            e.HasOne(t => t.Fund)
             .WithMany()
             .HasForeignKey(t => t.SchemeCode)
             .OnDelete(DeleteBehavior.Restrict);
        });
    }
}
