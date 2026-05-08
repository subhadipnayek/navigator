using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Services;

namespace NAVigator.Services;

/*
public class NavSyncWorker(
    IServiceProvider serviceProvider,
    ILogger<NavSyncWorker> logger) : BackgroundService
{
    private const int BatchSize = 10;
    private static readonly TimeSpan PollingInterval = TimeSpan.FromMinutes(5);
    private static readonly TimeSpan ApiDelay = TimeSpan.FromSeconds(3); // Polite rate limit

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        logger.LogInformation("NavSyncWorker starting...");

        try
        {
            // Wait a bit before first run to let the API start up completely
            await Task.Delay(TimeSpan.FromSeconds(15), stoppingToken);
        }
        catch (OperationCanceledException)
        {
            // Expected during shutdown
            return;
        }

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                await SyncStaleFundsAsync(stoppingToken);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                logger.LogError(ex, "An error occurred while syncing stale funds.");
            }

            try
            {
                // Sleep until next polling cycle
                await Task.Delay(PollingInterval, stoppingToken);
            }
            catch (OperationCanceledException)
            {
                // Expected during shutdown
                break;
            }
        }

        logger.LogInformation("NavSyncWorker stopping...");
    }

    private async Task SyncStaleFundsAsync(CancellationToken stoppingToken)
    {
        using var scope = serviceProvider.CreateScope();
        var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
        var navService = scope.ServiceProvider.GetRequiredService<INavService>();

        // Find the most stale funds.
        // A fund is stale if it has NEVER been synced (NavSyncTracker is null)
        // OR it was last synced more than 24 hours ago.
        var staleThreshold = DateTime.UtcNow.AddHours(-24);

        var staleFunds = await db.Funds
            .Include(f => f.NavSyncTracker)
            .Where(f => f.NavSyncTracker == null || f.NavSyncTracker.LastHistoryCheckAtIst < staleThreshold)
            .OrderBy(f => f.NavSyncTracker != null) // nulls first (never synced)
            .ThenBy(f => f.NavSyncTracker!.LastHistoryCheckAtIst) // then oldest synced
            .Take(BatchSize)
            .Select(f => f.SchemeCode)
            .ToListAsync(stoppingToken);

        if (staleFunds.Count == 0)
        {
            logger.LogDebug("No stale funds found. Database is fully up to date.");
            return;
        }

        logger.LogInformation("Found {Count} stale funds. Beginning background sync batch...", staleFunds.Count);

        foreach (var schemeCode in staleFunds)
        {
            if (stoppingToken.IsCancellationRequested)
                break;

            try
            {
                logger.LogInformation("Background Syncing NAV for fund {SchemeCode}", schemeCode);
                
                // Fetch the NAV history. This method handles deduplication, db storage,
                // anomaly detection triggering, and updating the NavSyncTracker automatically!
                await navService.GetNavHistoryAsync(schemeCode, ct: stoppingToken);
                
                // Sleep to avoid hammering the external AMFI API
                await Task.Delay(ApiDelay, stoppingToken);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                logger.LogWarning(ex, "Failed to sync fund {SchemeCode} in background.", schemeCode);
            }
        }

        logger.LogInformation("Completed background sync batch of {Count} funds.", staleFunds.Count);

        // Also run a global auto-record sweep in case there are eligible anomalies
        // that weren't caught during the sync phase.
        var actionService = scope.ServiceProvider.GetRequiredService<ICorporateActionService>();
        await actionService.AutoRecordAnomaliesAsync(ct: stoppingToken);
    }
}
*/
