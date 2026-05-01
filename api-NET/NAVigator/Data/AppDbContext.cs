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

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        base.OnModelCreating(modelBuilder);

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
        });
    }
}
