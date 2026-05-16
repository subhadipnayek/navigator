using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto.Portfolio;

namespace NAVigator.Services;

public class UserPortfolioService(AppDbContext db, INavService navService) : IUserPortfolioService
{
    private static readonly string[] BuyTypes = ["Buy", "Purchase", "SIP", "SwitchIn"];
    private static readonly string[] SellTypes = ["Sell", "Redeem", "Redemption", "SwitchOut"];
    private static readonly JsonSerializerOptions DashboardCacheJsonOptions = new(JsonSerializerDefaults.Web);

    public async Task<List<UserPortfolioSummaryDto>> GetPortfoliosAsync(int userId, CancellationToken ct)
    {
        return await db.UserPortfolios
            .AsNoTracking()
            .Where(p => p.UserId == userId)
            .OrderBy(p => p.Name)
            .Select(p => new UserPortfolioSummaryDto(
                p.Id,
                p.Name,
                p.Transactions.Select(t => t.SchemeCode).Distinct().Count(),
                p.UpdatedAtUtc))
            .ToListAsync(ct);
    }

    public async Task<UserPortfolioSummaryDto> CreatePortfolioAsync(int userId, CreateUserPortfolioRequest request, CancellationToken ct)
    {
        var portfolio = new UserPortfolio
        {
            UserId = userId,
            Name = string.IsNullOrWhiteSpace(request.Name) ? "My Portfolio" : request.Name.Trim()
        };

        db.UserPortfolios.Add(portfolio);
        await db.SaveChangesAsync(ct);
        return new UserPortfolioSummaryDto(portfolio.Id, portfolio.Name, 0, portfolio.UpdatedAtUtc);
    }

    public async Task<PortfolioDashboardDto?> GetDashboardAsync(int userId, long portfolioId, bool showFullyExitedFunds, CancellationToken ct)
    {
        var portfolio = await db.UserPortfolios
            .AsNoTracking()
            .FirstOrDefaultAsync(p => p.Id == portfolioId && p.UserId == userId, ct);
        if (portfolio is null) return null;

        var transactions = await LoadTransactions(portfolioId, ct);
        var emptyStates = new List<string>();
        if (transactions.Count == 0)
            emptyStates.Add("No transactions have been added yet.");

        var fundCodes = transactions.Select(t => t.SchemeCode).Distinct().ToList();
        var latestNavs = await GetLatestNavsAsync(fundCodes, ct);
        var cacheKey = await BuildDashboardCacheKey(portfolio, fundCodes, showFullyExitedFunds, ct);
        if (fundCodes.Count == latestNavs.Count)
        {
            var cached = await db.UserPortfolioDashboardCaches
                .AsNoTracking()
                .FirstOrDefaultAsync(c =>
                    c.UserId == userId &&
                    c.PortfolioId == portfolioId &&
                    c.IncludeFullyExitedFunds == showFullyExitedFunds &&
                    c.DependencyKey == cacheKey,
                    ct);
            if (cached is not null)
            {
                var cachedDashboard = JsonSerializer.Deserialize<PortfolioDashboardDto>(cached.DashboardJson, DashboardCacheJsonOptions);
                if (cachedDashboard is not null)
                    return cachedDashboard;
            }
        }

        var missingNavCodes = fundCodes.Where(code => !latestNavs.ContainsKey(code)).ToList();
        foreach (var schemeCode in missingNavCodes)
        {
            await navService.GetNavHistoryAsync(schemeCode, ct: ct);
        }
        if (missingNavCodes.Count > 0)
        {
            latestNavs = await GetLatestNavsAsync(fundCodes, ct);
            cacheKey = await BuildDashboardCacheKey(portfolio, fundCodes, showFullyExitedFunds, ct);
        }
        var fundInsights = BuildFundInsights(transactions, latestNavs, emptyStates);
        var visibleFunds = showFullyExitedFunds ? fundInsights : fundInsights.Where(f => !f.IsFullyExited).ToList();
        var visibleFundCodes = visibleFunds.Select(f => f.SchemeCode).ToHashSet();
        var visibleTransactions = transactions.Where(t => visibleFundCodes.Contains(t.SchemeCode)).ToList();
        var metrics = BuildMetrics(visibleFunds, visibleTransactions);
        var fundAllocation = BuildAllocation(
            visibleFunds.Where(f => f.CurrentValue > 0).Select(f => (f.SchemeName, f.CurrentValue)));

        var dashboard = new PortfolioDashboardDto(
            portfolio.Id,
            portfolio.Name,
            metrics,
            visibleFunds,
            BuildAllocation(visibleFunds.Where(f => f.CurrentValue > 0).Select(f => (f.AssetClass, f.CurrentValue))),
            fundAllocation,
            BuildContributions(transactions, financialYear: true),
            BuildContributions(transactions, financialYear: false),
            BuildSipConsistency(transactions),
            emptyStates);
        await SaveDashboardCache(userId, portfolioId, showFullyExitedFunds, cacheKey, dashboard, ct);
        return dashboard;
    }

    public async Task<(List<PortfolioTransactionDto> Rows, YearTransactionSummaryDto Summary)?> GetTransactionsAsync(
        int userId,
        long portfolioId,
        string financialYear,
        CancellationToken ct)
    {
        if (!await db.UserPortfolios.AnyAsync(p => p.Id == portfolioId && p.UserId == userId, ct))
            return null;

        var (start, end) = ParseFinancialYear(financialYear);
        var rows = await db.PortfolioTransactions
            .AsNoTracking()
            .Include(t => t.Fund)
            .Where(t => t.PortfolioId == portfolioId && t.TransactionDate >= start && t.TransactionDate <= end)
            .OrderBy(t => t.TransactionDate)
            .ThenBy(t => t.Id)
            .Select(t => ToDto(t, null, null))
            .ToListAsync(ct);

        return (rows, BuildYearSummary(rows));
    }

    public async Task<SavePortfolioTransactionsResponse?> SaveTransactionsAsync(
        int userId,
        long portfolioId,
        SavePortfolioTransactionsRequest request,
        CancellationToken ct)
    {
        var portfolio = await db.UserPortfolios.FirstOrDefaultAsync(p => p.Id == portfolioId && p.UserId == userId, ct);
        if (portfolio is null) return null;

        var (start, end) = ParseFinancialYear(request.FinancialYear);
        var schemeCodes = request.Transactions.Select(t => t.SchemeCode).Distinct().ToList();
        var funds = await db.Funds
            .AsNoTracking()
            .Where(f => schemeCodes.Contains(f.SchemeCode))
            .ToDictionaryAsync(f => f.SchemeCode, ct);

        var staged = new List<PortfolioTransaction>();
        var rowErrors = new Dictionary<string, string>();
        var rowWarnings = new Dictionary<string, string>();
        foreach (var row in request.Transactions)
        {
            var rowKey = RowKey(row);
            if (!funds.TryGetValue(row.SchemeCode, out var fund))
            {
                rowErrors[rowKey] = "Fund is not available in the database.";
                continue;
            }
            if (!string.Equals(row.SchemeName, fund.SchemeName, StringComparison.Ordinal))
            {
                rowErrors[rowKey] = $"Scheme code {row.SchemeCode} must match exactly to '{fund.SchemeName}'.";
                continue;
            }
            if (!TryParseDate(row.TransactionDate, out var txDate) || txDate < start || txDate > end)
            {
                rowErrors[rowKey] = $"Date must fall within {request.FinancialYear}.";
                continue;
            }
            if (!IsBuy(row.TransactionType) && !IsSell(row.TransactionType))
            {
                rowErrors[rowKey] = "Type must be Buy, SIP, Sell, Redeem, SwitchIn, or SwitchOut.";
                continue;
            }
            if (row.Units <= 0 || row.Nav <= 0 || row.Amount < 0)
            {
                rowErrors[rowKey] = "Units and NAV must be positive; amount cannot be negative.";
                continue;
            }
            var expectedAmount = Math.Round(row.Units * row.Nav, 2);
            if (Math.Abs(row.Amount - expectedAmount) > 0.01m)
            {
                rowErrors[rowKey] = $"Amount must equal units x NAV ({expectedAmount:0.00}). Submitted amount was {row.Amount:0.00}.";
                continue;
            }

            staged.Add(new PortfolioTransaction
            {
                PortfolioId = portfolioId,
                SchemeCode = row.SchemeCode,
                TransactionDate = txDate,
                TransactionType = NormalizeType(row.TransactionType),
                Units = row.Units,
                Nav = row.Nav,
                Amount = expectedAmount,
                Notes = string.IsNullOrWhiteSpace(row.Notes) ? null : row.Notes.Trim()
            });
        }

        var existingOutsideYear = await db.PortfolioTransactions
            .AsNoTracking()
            .Where(t => t.PortfolioId == portfolioId && (t.TransactionDate < start || t.TransactionDate > end))
            .ToListAsync(ct);
        FlagNegativeUnitRows(existingOutsideYear.Concat(staged).ToList(), staged, rowWarnings, request.Transactions);

        if (rowErrors.Count > 0)
        {
            var invalidRows = request.Transactions.Select(r =>
            {
                funds.TryGetValue(r.SchemeCode, out var fund);
                rowErrors.TryGetValue(RowKey(r), out var error);
                return ToDto(r, string.IsNullOrWhiteSpace(r.SchemeName) ? fund?.SchemeName ?? "Unknown fund" : r.SchemeName, error);
            }).ToList();
            return new SavePortfolioTransactionsResponse(false, "Fix highlighted rows before saving.", 0, invalidRows, BuildYearSummary(invalidRows));
        }

        var existingYearRows = await db.PortfolioTransactions
            .Where(t => t.PortfolioId == portfolioId && t.TransactionDate >= start && t.TransactionDate <= end)
            .ToListAsync(ct);
        db.PortfolioTransactions.RemoveRange(existingYearRows);
        db.PortfolioTransactions.AddRange(staged);
        portfolio.UpdatedAtUtc = DateTime.UtcNow;
        await db.SaveChangesAsync(ct);

        var result = await GetTransactionsAsync(userId, portfolioId, request.FinancialYear, ct);
        var transactionRows = ApplyImportWarnings(result!.Value.Rows, request.Transactions, rowWarnings);
        return new SavePortfolioTransactionsResponse(true, "Transactions saved.", staged.Count, transactionRows, result.Value.Summary);
    }

    public async Task<int?> GetImportDeletePreviewAsync(int userId, long portfolioId, DateOnly startDate, DateOnly endDate, CancellationToken ct)
    {
        if (!await db.UserPortfolios.AnyAsync(p => p.Id == portfolioId && p.UserId == userId, ct))
            return null;

        return await db.PortfolioTransactions
            .AsNoTracking()
            .CountAsync(t => t.PortfolioId == portfolioId && t.TransactionDate >= startDate && t.TransactionDate <= endDate, ct);
    }

    public async Task<ImportPortfolioTransactionsResponse?> ImportTransactionsAsync(
        int userId,
        long portfolioId,
        string financialYear,
        string importMode,
        DateOnly startDate,
        DateOnly endDate,
        Stream fileStream,
        string fileName,
        CancellationToken ct)
    {
        var portfolio = await db.UserPortfolios.FirstOrDefaultAsync(p => p.Id == portfolioId && p.UserId == userId, ct);
        if (portfolio is null) return null;

        List<ExcelTransactionDraft> drafts;
        try
        {
            drafts = ExcelTransactionWorkbookParser.Parse(fileStream, fileName);
        }
        catch (InvalidDataException ex)
        {
            var empty = await GetTransactionsAsync(userId, portfolioId, financialYear, ct);
            return EmptyImportResponse(ex.Message, empty!.Value.Rows, empty.Value.Summary, [ex.Message]);
        }

        if (drafts.Count == 0)
        {
            var empty = await GetTransactionsAsync(userId, portfolioId, financialYear, ct);
            return EmptyImportResponse("The Excel file did not contain any transaction rows.", empty!.Value.Rows, empty.Value.Summary, ["The Excel file did not contain any transaction rows."]);
        }

        var upserts = new List<PortfolioTransactionUpsertDto>();
        var rowErrors = new Dictionary<string, string>();
        var rowWarnings = new Dictionary<string, string>();
        var skippedOutOfRange = 0;
        var fundKeys = drafts
            .Select(d => (SchemeCode: NormalizeOptional(d.SchemeCode), Isin: NormalizeOptional(d.Isin)))
            .Where(x => int.TryParse(x.SchemeCode, out _) || !string.IsNullOrWhiteSpace(x.Isin))
            .ToList();
        var schemeCodes = fundKeys
            .Select(x => int.TryParse(x.SchemeCode, out var schemeCode) ? schemeCode : (int?)null)
            .Where(x => x.HasValue)
            .Select(x => x!.Value)
            .Distinct()
            .ToList();
        var isins = fundKeys
            .Select(x => x.Isin)
            .Where(x => !string.IsNullOrWhiteSpace(x))
            .Distinct(StringComparer.OrdinalIgnoreCase)
            .ToList();
        var funds = await db.Funds
            .AsNoTracking()
            .Where(f => schemeCodes.Contains(f.SchemeCode) || isins.Contains(f.IsinGrowth!) || isins.Contains(f.IsinDivReinvestment!))
            .ToListAsync(ct);

        foreach (var draft in drafts)
        {
            var clientRowId = $"excel-{draft.RowNumber}";
            var rowKey = clientRowId;
            if (!TryParseImportDate(draft.TradeDate, out var txDate))
            {
                rowErrors[rowKey] = $"Unrecognised date format: '{draft.TradeDate ?? ""}'.";
                continue;
            }
            if (txDate < startDate || txDate > endDate)
            {
                skippedOutOfRange++;
                continue;
            }

            var fund = MatchFund(draft, funds);
            if (fund.Error is not null || fund.Value is null)
            {
                rowErrors[rowKey] = fund.Error ?? "Fund match failed.";
                continue;
            }
            if (!TryParseDecimal(draft.Price, out var nav) || nav <= 0)
            {
                rowErrors[rowKey] = "Price is required or invalid.";
                continue;
            }
            if (!TryParseDecimal(draft.Quantity, out var units) || units <= 0)
            {
                rowErrors[rowKey] = "Quantity is required or invalid.";
                continue;
            }
            if (!IsBuy(draft.TradeType ?? string.Empty) && !IsSell(draft.TradeType ?? string.Empty))
            {
                rowErrors[rowKey] = "Trade Type must be Buy, SIP, Sell, Redeem, SwitchIn, or SwitchOut.";
                continue;
            }

            upserts.Add(new PortfolioTransactionUpsertDto(
                null,
                clientRowId,
                fund.Value.SchemeCode,
                fund.Value.SchemeName,
                txDate.ToString("yyyy-MM-dd"),
                NormalizeType(draft.TradeType ?? string.Empty),
                units,
                nav,
                Math.Round(units * nav, 2),
                NormalizeOptional(draft.Notes)));
        }

        var staged = upserts.Select(row => new PortfolioTransaction
        {
            PortfolioId = portfolioId,
            SchemeCode = row.SchemeCode,
            TransactionDate = DateOnly.ParseExact(row.TransactionDate, "yyyy-MM-dd"),
            TransactionType = NormalizeType(row.TransactionType),
            Units = row.Units,
            Nav = row.Nav,
            Amount = Math.Round(row.Units * row.Nav, 2),
            Notes = string.IsNullOrWhiteSpace(row.Notes) ? null : row.Notes.Trim()
        }).ToList();

        if (staged.Count > 0)
        {
            var rowsToKeepForUnitCheck = string.Equals(importMode, "Replace", StringComparison.OrdinalIgnoreCase)
                ? await db.PortfolioTransactions
                    .AsNoTracking()
                    .Where(t => t.PortfolioId == portfolioId && (t.TransactionDate < startDate || t.TransactionDate > endDate))
                    .ToListAsync(ct)
                : await db.PortfolioTransactions
                    .AsNoTracking()
                    .Where(t => t.PortfolioId == portfolioId)
                    .ToListAsync(ct);
            FlagNegativeUnitRows(rowsToKeepForUnitCheck.Concat(staged).ToList(), staged, rowWarnings, upserts);
        }

        if (staged.Count == 0)
        {
            var result = await GetTransactionsAsync(userId, portfolioId, financialYear, ct);
            return new ImportPortfolioTransactionsResponse(
                false,
                BuildImportMessage(0, rowErrors.Count, skippedOutOfRange, rowWarnings.Count),
                drafts.Count,
                0,
                0,
                skippedOutOfRange,
                null,
                null,
                0,
                0,
                FormatImportErrors(rowErrors),
                FormatImportErrors(rowWarnings),
                result!.Value.Rows,
                result.Value.Summary);
        }

        var replacedCount = 0;
        if (string.Equals(importMode, "Replace", StringComparison.OrdinalIgnoreCase))
        {
            var existingRangeRows = await db.PortfolioTransactions
                .Where(t => t.PortfolioId == portfolioId && t.TransactionDate >= startDate && t.TransactionDate <= endDate)
                .ToListAsync(ct);
            replacedCount = existingRangeRows.Count;
            db.PortfolioTransactions.RemoveRange(existingRangeRows);
        }

        db.PortfolioTransactions.AddRange(staged);
        portfolio.UpdatedAtUtc = DateTime.UtcNow;
        await db.SaveChangesAsync(ct);

        var updated = await GetTransactionsAsync(userId, portfolioId, financialYear, ct);
        var transactionRows = ApplyImportWarnings(updated!.Value.Rows, upserts, rowWarnings);
        var importedStart = staged.Min(t => t.TransactionDate);
        var importedEnd = staged.Max(t => t.TransactionDate);
        return new ImportPortfolioTransactionsResponse(
            true,
            BuildImportMessage(staged.Count, rowErrors.Count, skippedOutOfRange, rowWarnings.Count),
            drafts.Count,
            staged.Count,
            replacedCount,
            skippedOutOfRange,
            importedStart.ToString("yyyy-MM-dd"),
            importedEnd.ToString("yyyy-MM-dd"),
            staged.Sum(t => t.Amount),
            staged.Select(t => t.SchemeCode).Distinct().Count(),
            FormatImportErrors(rowErrors),
            FormatImportErrors(rowWarnings),
            transactionRows,
            updated.Value.Summary);
    }

    public async Task<PortfolioBenchmarkResponse?> CompareBenchmarkAsync(int userId, long portfolioId, PortfolioBenchmarkRequest request, CancellationToken ct)
    {
        var transactions = await LoadTransactions(portfolioId, ct);
        if (transactions.Count == 0) return null;

        var benchmarkCodes = request.BenchmarkType.Equals("basket", StringComparison.OrdinalIgnoreCase) && request.Basket is not null
            ? request.Basket.Funds.Select(f => f.SchemeCode).Distinct().ToList()
            : request.SchemeCode.HasValue ? [request.SchemeCode.Value] : new List<int>();
        if (benchmarkCodes.Count == 0)
            return null;

        var benchmarkNavs = await navService.EnsureNavHistoriesAsync(benchmarkCodes, ct: ct);
        var benchmarkFirstNavDate = benchmarkNavs.Values.SelectMany(rows => rows).Select(n => (DateOnly?)n.Date).Min();
        var benchmarkLastNavDate = benchmarkNavs.Values.SelectMany(rows => rows).Select(n => (DateOnly?)n.Date).Max();
        if (!benchmarkFirstNavDate.HasValue || !benchmarkLastNavDate.HasValue)
            return null;

        var portfolioStart = transactions.Min(t => t.TransactionDate);
        var requestedEnd = ParseBenchmarkDate(request.EndDate) ?? DateOnly.FromDateTime(DateTime.UtcNow);
        var requestedStart = ParseBenchmarkDate(request.StartDate)
            ?? (request.DurationYears.HasValue ? requestedEnd.AddYears(-request.DurationYears.Value) : (DateOnly?)null);
        var effectiveStart = MaxDate(portfolioStart, benchmarkFirstNavDate.Value, requestedStart ?? DateOnly.MinValue);
        var effectiveEnd = MinDate(requestedEnd, benchmarkLastNavDate.Value, DateOnly.FromDateTime(DateTime.UtcNow));
        if (effectiveStart > effectiveEnd)
            return null;

        var scopedTransactions = transactions
            .Where(t => t.TransactionDate >= effectiveStart && t.TransactionDate <= effectiveEnd)
            .ToList();
        if (scopedTransactions.Count == 0)
            return null;

        var portfolioMetric = await SimulateActualPortfolio(scopedTransactions, effectiveEnd, ct);
        var benchmark = request.BenchmarkType.Equals("basket", StringComparison.OrdinalIgnoreCase) && request.Basket is not null
            ? await SimulateBenchmarkBasket(scopedTransactions, request.Basket, effectiveEnd, ct)
            : await SimulateBenchmarkFund(scopedTransactions, request.SchemeCode, effectiveEnd, ct);

        var rolling = request.RollingPeriodYears is > 0 && benchmarkCodes.Count == 1
            ? BuildBenchmarkRollingReturn(benchmarkNavs[benchmarkCodes[0]], request.RollingPeriodYears.Value, effectiveStart, effectiveEnd)
            : null;

        var alpha = portfolioMetric.XirrPercent.HasValue && benchmark.XirrPercent.HasValue
            ? portfolioMetric.XirrPercent.Value - benchmark.XirrPercent.Value
            : (decimal?)null;

        var verdict = alpha switch
        {
            null => "Insufficient data for XIRR comparison.",
            > 0 => "Portfolio is ahead of the benchmark.",
            < 0 => "Portfolio is trailing the benchmark.",
            _ => "Portfolio is in line with the benchmark."
        };

        var note = effectiveStart == portfolioStart
            ? "Comparison starts from the first portfolio transaction in the selected window."
            : $"Comparison starts from benchmark NAV availability on {effectiveStart:yyyy-MM-dd}.";

        return new PortfolioBenchmarkResponse(
            benchmark.Name,
            effectiveStart.ToString("yyyy-MM-dd"),
            effectiveEnd.ToString("yyyy-MM-dd"),
            note,
            portfolioMetric,
            benchmark,
            rolling,
            alpha,
            verdict);
    }

    private async Task<List<PortfolioTransaction>> LoadTransactions(long portfolioId, CancellationToken ct) =>
        await db.PortfolioTransactions
            .AsNoTracking()
            .Include(t => t.Fund)
            .Where(t => t.PortfolioId == portfolioId)
            .OrderBy(t => t.TransactionDate)
            .ThenBy(t => t.Id)
            .ToListAsync(ct);

    private async Task<Dictionary<int, NavHistory>> GetLatestNavsAsync(List<int> schemeCodes, CancellationToken ct)
    {
        var rows = await db.NavHistories
            .AsNoTracking()
            .Where(n => schemeCodes.Contains(n.SchemeCode) && n.Date <= DateOnly.FromDateTime(DateTime.UtcNow))
            .OrderByDescending(n => n.Date)
            .ToListAsync(ct);
        return rows.GroupBy(n => n.SchemeCode).ToDictionary(g => g.Key, g => g.First());
    }

    private async Task<string> BuildDashboardCacheKey(
        UserPortfolio portfolio,
        List<int> schemeCodes,
        bool includeFullyExitedFunds,
        CancellationToken ct)
    {
        var distinctCodes = schemeCodes.Distinct().OrderBy(code => code).ToList();
        var trackers = await db.NavSyncTrackers
            .AsNoTracking()
            .Where(t => distinctCodes.Contains(t.SchemeCode))
            .Select(t => new
            {
                t.SchemeCode,
                t.LatestNav,
                t.LatestNavDate,
                t.NavHistoryCount,
                t.CorporateActionCount,
                t.NavHistoryUpdatedAtIst,
                t.UpdatedAtIst
            })
            .ToListAsync(ct);
        var trackerByCode = trackers.ToDictionary(t => t.SchemeCode);

        var corporateActionKeys = await db.CorporateActions
            .AsNoTracking()
            .Where(c => distinctCodes.Contains(c.SchemeCode))
            .GroupBy(c => c.SchemeCode)
            .Select(g => new
            {
                SchemeCode = g.Key,
                Count = g.Count(),
                LastCreatedAtUtc = g.Max(c => c.CreatedAtUtc)
            })
            .ToListAsync(ct);
        var corporateActionByCode = corporateActionKeys.ToDictionary(c => c.SchemeCode);

        var parts = new List<string>
        {
            $"portfolio:{portfolio.Id}:{portfolio.UpdatedAtUtc.Ticks}",
            $"includeExited:{includeFullyExitedFunds}"
        };

        foreach (var code in distinctCodes)
        {
            trackerByCode.TryGetValue(code, out var tracker);
            corporateActionByCode.TryGetValue(code, out var action);
            parts.Add(string.Join(':',
                code,
                tracker?.LatestNav?.ToString(CultureInfo.InvariantCulture) ?? "nav-null",
                tracker?.LatestNavDate?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ?? "date-null",
                tracker?.NavHistoryCount.ToString(CultureInfo.InvariantCulture) ?? "history-null",
                tracker?.CorporateActionCount.ToString(CultureInfo.InvariantCulture) ?? "tracker-ca-null",
                tracker?.NavHistoryUpdatedAtIst?.Ticks.ToString(CultureInfo.InvariantCulture) ?? "history-updated-null",
                tracker?.UpdatedAtIst.Ticks.ToString(CultureInfo.InvariantCulture) ?? "tracker-updated-null",
                action?.Count.ToString(CultureInfo.InvariantCulture) ?? "ca-null",
                action?.LastCreatedAtUtc.Ticks.ToString(CultureInfo.InvariantCulture) ?? "ca-created-null"));
        }

        var rawKey = string.Join('|', parts);
        return Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(rawKey)));
    }

    private async Task SaveDashboardCache(
        int userId,
        long portfolioId,
        bool includeFullyExitedFunds,
        string dependencyKey,
        PortfolioDashboardDto dashboard,
        CancellationToken ct)
    {
        var json = JsonSerializer.Serialize(dashboard, DashboardCacheJsonOptions);
        var cache = await db.UserPortfolioDashboardCaches
            .FirstOrDefaultAsync(c =>
                c.UserId == userId &&
                c.PortfolioId == portfolioId &&
                c.IncludeFullyExitedFunds == includeFullyExitedFunds,
                ct);

        if (cache is null)
        {
            db.UserPortfolioDashboardCaches.Add(new UserPortfolioDashboardCache
            {
                UserId = userId,
                PortfolioId = portfolioId,
                IncludeFullyExitedFunds = includeFullyExitedFunds,
                DependencyKey = dependencyKey,
                DashboardJson = json
            });
        }
        else
        {
            cache.DependencyKey = dependencyKey;
            cache.DashboardJson = json;
            cache.UpdatedAtUtc = DateTime.UtcNow;
        }

        await db.SaveChangesAsync(ct);
    }

    private static List<PortfolioFundInsightDto> BuildFundInsights(
        List<PortfolioTransaction> transactions,
        Dictionary<int, NavHistory> latestNavs,
        List<string> emptyStates)
    {
        var insights = new List<PortfolioFundInsightDto>();
        foreach (var group in transactions.GroupBy(t => t.SchemeCode))
        {
            var first = group.First();
            latestNavs.TryGetValue(group.Key, out var latestNav);
            var ledger = BuildLedger(group.OrderBy(t => t.TransactionDate).ThenBy(t => t.Id));
            var units = ledger.Units;
            var invested = ledger.Invested;
            var redemptions = group.Where(t => IsSell(t.TransactionType)).Sum(t => t.Amount);
            var currentValue = latestNav is null ? 0 : Math.Round(units * latestNav.Nav, 2);
            var isFullyExited = latestNav is not null
                ? Math.Abs(currentValue) < 10
                : units == 0;
            var totalGain = redemptions + currentValue - invested;
            var xirr = BuildXirr(group.ToList(), currentValue, latestNav?.Date);
            var firstDate = group.Min(t => t.TransactionDate);
            var lastDate = units > 0 && latestNav is not null ? latestNav.Date : group.Max(t => t.TransactionDate);

            if (units > 0 && latestNav is null)
                emptyStates.Add($"{first.Fund.SchemeName} has units but no NAV is available.");
            if (units < 0)
                emptyStates.Add($"{first.Fund.SchemeName} has negative units and needs transaction review.");

            insights.Add(new PortfolioFundInsightDto(
                group.Key,
                first.Fund.SchemeName,
                first.Fund.FundHouse,
                first.Fund.SchemeCategory,
                ClassifyAsset(first.Fund),
                units,
                invested,
                currentValue,
                ledger.RealizedPnl,
                currentValue - ledger.RemainingCost,
                invested == 0 ? 0 : Math.Round(totalGain / invested * 100, 2),
                CalculateCagr(invested, redemptions + currentValue, firstDate, lastDate),
                xirr,
                Math.Max(0, lastDate.DayNumber - firstDate.DayNumber),
                isFullyExited,
                latestNav?.Nav,
                latestNav?.Date.ToString("dd-MM-yyyy"),
                latestNav is not null));
        }
        return insights.OrderByDescending(f => f.CurrentValue).ThenBy(f => f.SchemeName).ToList();
    }

    private static PortfolioMetricsDto BuildMetrics(List<PortfolioFundInsightDto> funds, List<PortfolioTransaction> transactions)
    {
        var totalInvested = transactions.Where(t => IsBuy(t.TransactionType)).Sum(t => t.Amount);
        var totalWithdrawn = transactions.Where(t => IsSell(t.TransactionType)).Sum(t => t.Amount);
        var invested = totalInvested - totalWithdrawn;
        var current = funds.Sum(f => f.CurrentValue);
        var realizedPnl = funds.Sum(f => f.RealizedPnl);
        var unrealizedPnl = funds.Sum(f => f.UnrealizedPnl);
        var gain = current - invested;
        var start = transactions.Count == 0 ? DateOnly.FromDateTime(DateTime.UtcNow) : transactions.Min(t => t.TransactionDate);
        var end = DateOnly.FromDateTime(DateTime.UtcNow);
        var xirr = BuildXirr(transactions, current, end);

        return new PortfolioMetricsDto(
            invested,
            current,
            realizedPnl,
            unrealizedPnl,
            invested == 0 ? 0 : Math.Round(gain / invested * 100, 2),
            CalculateCagr(totalInvested, totalWithdrawn + current, start, end),
            xirr,
            funds.Count,
            funds.Count(f => !f.IsFullyExited),
            funds.Count(f => f.IsFullyExited));
    }

    private static List<AllocationSliceDto> BuildAllocation(IEnumerable<(string Label, decimal Value)> rows)
    {
        var grouped = rows
            .GroupBy(r => r.Label)
            .Select(g => (Label: g.Key, Value: g.Sum(r => r.Value)))
            .Where(r => r.Value > 0)
            .OrderByDescending(r => r.Value)
            .ToList();
        var total = grouped.Sum(r => r.Value);
        return grouped.Select(r => new AllocationSliceDto(r.Label, r.Value, total == 0 ? 0 : Math.Round(r.Value / total * 100, 2))).ToList();
    }

    private static List<ContributionPeriodDto> BuildContributions(List<PortfolioTransaction> rows, bool financialYear)
    {
        return rows
            .GroupBy(t => financialYear ? ToFinancialYear(t.TransactionDate) : t.TransactionDate.Year.ToString())
            .OrderBy(g => g.Key)
            .Select(g =>
            {
                var invested = g.Where(t => IsBuy(t.TransactionType)).Sum(t => t.Amount);
                var redeemed = g.Where(t => IsSell(t.TransactionType)).Sum(t => t.Amount);
                return new ContributionPeriodDto(g.Key, invested, redeemed, invested - redeemed);
            })
            .ToList();
    }

    private static SipConsistencyDto BuildSipConsistency(List<PortfolioTransaction> rows)
    {
        var buyRows = rows.Where(t => IsBuy(t.TransactionType)).ToList();
        if (buyRows.Count == 0) return new SipConsistencyDto(0, 0, 0, 0);
        var first = buyRows.Min(t => t.TransactionDate);
        var last = buyRows.Max(t => t.TransactionDate);
        var activeMonths = ((last.Year - first.Year) * 12) + last.Month - first.Month + 1;
        var investedMonths = buyRows.Select(t => new { t.TransactionDate.Year, t.TransactionDate.Month }).Distinct().Count();
        return new SipConsistencyDto(activeMonths, investedMonths, Math.Round((decimal)investedMonths / activeMonths * 100, 2), Math.Round(buyRows.Sum(t => t.Amount) / activeMonths, 2));
    }

    private async Task<PortfolioBenchmarkMetricDto> SimulateActualPortfolio(List<PortfolioTransaction> rows, DateOnly endDate, CancellationToken ct)
    {
        var codes = rows.Select(t => t.SchemeCode).Distinct().ToList();
        var navs = await navService.EnsureNavHistoriesAsync(codes, to: endDate, ct: ct);
        var units = codes.ToDictionary(code => code, _ => 0m);
        foreach (var tx in rows)
            units[tx.SchemeCode] += IsBuy(tx.TransactionType) ? tx.Units : -tx.Units;

        var current = 0m;
        foreach (var item in units)
        {
            var nav = navs.GetValueOrDefault(item.Key)?.LastOrDefault(n => n.Date <= endDate);
            if (nav is not null)
                current += item.Value * nav.Nav;
        }

        current = Math.Round(current, 2);
        var invested = rows.Where(t => IsBuy(t.TransactionType)).Sum(t => t.Amount);
        var redemptions = rows.Where(t => IsSell(t.TransactionType)).Sum(t => t.Amount);
        var cashFlows = rows
            .Select(t => (t.TransactionDate.ToDateTime(TimeOnly.MinValue), (double)(IsBuy(t.TransactionType) ? -t.Amount : t.Amount)))
            .ToList();
        cashFlows.Add((endDate.ToDateTime(TimeOnly.MinValue), (double)current));
        return new PortfolioBenchmarkMetricDto(
            invested,
            current + redemptions,
            invested == 0 ? 0 : Math.Round((current + redemptions - invested) / invested * 100, 2),
            XirrCalculator.Calculate(cashFlows));
    }

    private async Task<NamedMetric> SimulateBenchmarkFund(List<PortfolioTransaction> rows, int? schemeCode, DateOnly endDate, CancellationToken ct)
    {
        if (schemeCode is null) return new NamedMetric("Select a benchmark", 0, 0, 0, null);
        var fund = await db.Funds.AsNoTracking().FirstOrDefaultAsync(f => f.SchemeCode == schemeCode.Value, ct);
        var navs = await navService.EnsureNavHistoriesAsync([schemeCode.Value], to: endDate, ct: ct);
        return SimulateBenchmark(rows, [(schemeCode.Value, 100m)], navs.SelectMany(kvp => kvp.Value).ToLookup(n => n.SchemeCode), fund?.SchemeName ?? "Benchmark", endDate);
    }

    private async Task<NamedMetric> SimulateBenchmarkBasket(List<PortfolioTransaction> rows, BasketDefinition basket, DateOnly endDate, CancellationToken ct)
    {
        var codes = basket.Funds.Select(f => f.SchemeCode).ToList();
        var navs = await navService.EnsureNavHistoriesAsync(codes, to: endDate, ct: ct);
        return SimulateBenchmark(rows, basket.Funds.Select(f => (f.SchemeCode, f.AllocationPercent)).ToList(), navs.SelectMany(kvp => kvp.Value).ToLookup(n => n.SchemeCode), basket.Name, endDate);
    }

    private static NamedMetric SimulateBenchmark(List<PortfolioTransaction> rows, List<(int SchemeCode, decimal Allocation)> basket, ILookup<int, NavHistory> navs, string name, DateOnly endDate)
    {
        var units = basket.ToDictionary(b => b.SchemeCode, _ => 0m);
        var invested = 0m;
        var withdrawn = 0m;
        foreach (var tx in rows)
        {
            foreach (var item in basket)
            {
                var nav = navs[item.SchemeCode].LastOrDefault(n => n.Date <= tx.TransactionDate);
                if (nav is null) continue;
                var allocated = tx.Amount * item.Allocation / 100m;
                units[item.SchemeCode] += IsBuy(tx.TransactionType) ? allocated / nav.Nav : -allocated / nav.Nav;
            }
            if (IsBuy(tx.TransactionType)) invested += tx.Amount;
            if (IsSell(tx.TransactionType)) withdrawn += tx.Amount;
        }

        var current = 0m;
        foreach (var item in basket)
        {
            var latest = navs[item.SchemeCode].LastOrDefault(n => n.Date <= endDate);
            if (latest is not null) current += units[item.SchemeCode] * latest.Nav;
        }
        current = Math.Round(current, 2);
        var cashFlows = rows.Select(t => (t.TransactionDate.ToDateTime(TimeOnly.MinValue), (double)(IsBuy(t.TransactionType) ? -t.Amount : t.Amount))).ToList();
        cashFlows.Add((endDate.ToDateTime(TimeOnly.MinValue), (double)current));
        var totalValue = current + withdrawn;
        return new NamedMetric(name, invested, totalValue, invested == 0 ? 0 : Math.Round((totalValue - invested) / invested * 100, 2), XirrCalculator.Calculate(cashFlows));
    }

    private static BenchmarkRollingReturnDto? BuildBenchmarkRollingReturn(List<NavHistory> navs, int periodYears, DateOnly start, DateOnly end)
    {
        var sorted = navs.Where(n => n.Date >= start.AddYears(-periodYears) && n.Date <= end).OrderBy(n => n.Date).ToList();
        var returns = new List<decimal>();
        foreach (var endNav in sorted.Where(n => n.Date >= start))
        {
            var startDate = endNav.Date.AddYears(-periodYears);
            var startNav = FindClosestNavValue(sorted, startDate, 7);
            if (startNav is null || startNav.Value.Nav <= 0) continue;
            var years = (endNav.Date.DayNumber - startNav.Value.Date.DayNumber) / 365.25d;
            if (years <= 0) continue;
            returns.Add(Math.Round(((decimal)Math.Pow((double)(endNav.Nav / startNav.Value.Nav), 1 / years) - 1) * 100, 2));
        }

        return returns.Count == 0
            ? new BenchmarkRollingReturnDto(periodYears, 0, null, null, null)
            : new BenchmarkRollingReturnDto(periodYears, returns.Count, returns.Min(), returns.Max(), Math.Round(returns.Average(), 2));
    }

    private static void FlagNegativeUnitRows(List<PortfolioTransaction> allRows, List<PortfolioTransaction> stagedRows, Dictionary<string, string> rowErrors, List<PortfolioTransactionUpsertDto> requestRows)
    {
        var stagedBySignature = stagedRows
            .Select((row, index) => (row, index))
            .ToDictionary(x => Signature(x.row, x.index), x => x.index);

        foreach (var group in allRows.OrderBy(t => t.TransactionDate).ThenBy(t => t.Id).GroupBy(t => t.SchemeCode))
        {
            var units = 0m;
            foreach (var row in group)
            {
                units += SignedUnits(row);
                if (units < -0.000001m)
                {
                    var signature = stagedRows.Select((r, i) => (r, i)).FirstOrDefault(x => ReferenceEquals(x.r, row));
                    if (signature.r is not null && signature.i < requestRows.Count)
                    {
                        rowErrors[RowKey(requestRows[signature.i])] = "This transaction would make units negative for the fund.";
                    }
                }
            }
        }
    }

    private static decimal? BuildXirr(List<PortfolioTransaction> rows, decimal terminalValue, DateOnly? terminalDate)
    {
        var cashFlows = rows
            .Select(t => (t.TransactionDate.ToDateTime(TimeOnly.MinValue), (double)(IsBuy(t.TransactionType) ? -t.Amount : t.Amount)))
            .ToList();
        if (terminalValue > 0 && terminalDate.HasValue)
            cashFlows.Add((terminalDate.Value.ToDateTime(TimeOnly.MinValue), (double)terminalValue));
        return XirrCalculator.Calculate(cashFlows);
    }

    private static decimal? CalculateCagr(decimal invested, decimal finalValue, DateOnly start, DateOnly end)
    {
        if (invested <= 0 || finalValue <= 0 || end <= start) return null;
        var years = (end.DayNumber - start.DayNumber) / 365.25d;
        if (years <= 0) return null;
        return Math.Round(((decimal)Math.Pow((double)(finalValue / invested), 1 / years) - 1) * 100, 2);
    }

    private static FundLedger BuildLedger(IEnumerable<PortfolioTransaction> rows)
    {
        var units = 0m;
        var remainingCost = 0m;
        var invested = 0m;
        var realizedPnl = 0m;

        foreach (var row in rows)
        {
            if (IsBuy(row.TransactionType))
            {
                units += row.Units;
                remainingCost += row.Amount;
                invested += row.Amount;
                continue;
            }

            var averageCost = units <= 0 ? 0 : remainingCost / units;
            var removedCost = Math.Min(remainingCost, averageCost * row.Units);
            units -= row.Units;
            remainingCost -= removedCost;
            realizedPnl += row.Amount - removedCost;
        }

        return new FundLedger(units, remainingCost, invested, realizedPnl);
    }

    private static YearTransactionSummaryDto BuildYearSummary(List<PortfolioTransactionDto> rows) =>
        new(rows.Where(r => IsBuy(r.TransactionType)).Sum(r => r.Amount), rows.Count, rows.Select(r => r.SchemeCode).Distinct().Count());

    private static ImportPortfolioTransactionsResponse EmptyImportResponse(
        string message,
        List<PortfolioTransactionDto> rows,
        YearTransactionSummaryDto summary,
        List<string> rowErrors) =>
        new(false, message, 0, 0, 0, 0, null, null, 0, 0, rowErrors, [], rows, summary);

    private static PortfolioTransactionDto ToDto(PortfolioTransaction t, string? clientRowId, string? error) =>
        new(t.Id, clientRowId, t.SchemeCode, t.Fund.SchemeName, t.TransactionDate.ToString("yyyy-MM-dd"), t.TransactionType, t.Units, t.Nav, t.Amount, t.Notes, error);

    private static PortfolioTransactionDto ToDto(PortfolioTransactionUpsertDto t, string schemeName, string? error) =>
        new(t.Id, t.ClientRowId, t.SchemeCode, schemeName, t.TransactionDate, NormalizeType(t.TransactionType), t.Units, t.Nav, t.Amount == 0 ? Math.Round(t.Units * t.Nav, 2) : t.Amount, t.Notes, error);

    private static (DateOnly Start, DateOnly End) ParseFinancialYear(string value)
    {
        var startYear = int.Parse(value[..4]);
        return (new DateOnly(startYear, 4, 1), new DateOnly(startYear + 1, 3, 31));
    }

    private static bool TryParseDate(string value, out DateOnly date) =>
        DateOnly.TryParseExact(value, "yyyy-MM-dd", out date) || DateOnly.TryParseExact(value, "dd-MM-yyyy", out date);

    private static DateOnly? ParseBenchmarkDate(string? value) =>
        string.IsNullOrWhiteSpace(value)
            ? null
            : DateOnly.TryParseExact(value, "yyyy-MM-dd", out var date)
                ? date
                : null;

    private static DateOnly MaxDate(params DateOnly[] dates) => dates.Max();

    private static DateOnly MinDate(params DateOnly[] dates) => dates.Min();

    private static (DateOnly Date, decimal Nav)? FindClosestNavValue(List<NavHistory> sortedAsc, DateOnly targetDate, int toleranceDays)
    {
        var best = sortedAsc
            .Select(n => new { n.Date, n.Nav, Distance = Math.Abs(n.Date.DayNumber - targetDate.DayNumber) })
            .Where(x => x.Distance <= toleranceDays)
            .OrderBy(x => x.Distance)
            .ThenByDescending(x => x.Date)
            .FirstOrDefault();
        return best is null ? null : (best.Date, best.Nav);
    }

    private static bool TryParseImportDate(string? value, out DateOnly date)
    {
        var text = NormalizeOptional(value) ?? string.Empty;
        var formats = new[]
        {
            "yyyy-MM-dd",
            "dd/MM/yyyy",
            "d/M/yyyy",
            "MM/dd/yyyy",
            "M/d/yyyy",
            "dd-MM-yyyy",
            "d-M-yyyy",
            "dd MMM yyyy",
            "d MMM yyyy",
            "MMM dd, yyyy",
            "MMM d, yyyy"
        };

        return DateOnly.TryParseExact(text, formats, CultureInfo.InvariantCulture, DateTimeStyles.AllowWhiteSpaces, out date);
    }

    private static string ToFinancialYear(DateOnly date)
    {
        var startYear = date.Month >= 4 ? date.Year : date.Year - 1;
        return $"{startYear}-{(startYear + 1) % 100:00}";
    }

    private static (Fund? Value, string? Error) MatchFund(ExcelTransactionDraft row, List<Fund> funds)
    {
        var isin = NormalizeOptional(row.Isin);
        var schemeCodeText = NormalizeOptional(row.SchemeCode);
        var schemeName = NormalizeOptional(row.SchemeName);
        var hasSchemeCode = int.TryParse(schemeCodeText, out var schemeCode);

        if (string.IsNullOrWhiteSpace(isin) && !hasSchemeCode)
            return (null, "SchemeCode and ISIN both missing.");

        var matches = funds.Where(f =>
        {
            var codeMatches = hasSchemeCode && f.SchemeCode == schemeCode;
            var isinMatches = !string.IsNullOrWhiteSpace(isin) &&
                (string.Equals(f.IsinGrowth, isin, StringComparison.OrdinalIgnoreCase) ||
                 string.Equals(f.IsinDivReinvestment, isin, StringComparison.OrdinalIgnoreCase));
            return hasSchemeCode && !string.IsNullOrWhiteSpace(isin)
                ? codeMatches && isinMatches
                : codeMatches || isinMatches;
        }).Take(5).ToList();

        if (matches.Count == 0)
            return (null, "SchemeCode and ISIN not found in database.");

        var match = matches.Count == 1
            ? matches[0]
            : matches.FirstOrDefault(f => string.Equals(f.SchemeName, schemeName, StringComparison.OrdinalIgnoreCase));

        return match is null
            ? (null, "Multiple fund matches found; SchemeName did not disambiguate.")
            : (match, null);
    }

    private static bool TryParseDecimal(string? value, out decimal result)
    {
        var normalized = NormalizeOptional(value)?.Replace(",", string.Empty);
        return decimal.TryParse(normalized, NumberStyles.Number, CultureInfo.InvariantCulture, out result) ||
               decimal.TryParse(normalized, NumberStyles.Number, CultureInfo.CurrentCulture, out result);
    }

    private static List<string> FormatImportErrors(Dictionary<string, string> rowErrors) =>
        rowErrors
            .OrderBy(x => int.TryParse(x.Key.Replace("excel-", string.Empty), out var rowNumber) ? rowNumber : int.MaxValue)
            .Select(x => $"Row {x.Key.Replace("excel-", string.Empty)} ({x.Value})")
            .ToList();

    private static List<PortfolioTransactionDto> ApplyImportWarnings(
        List<PortfolioTransactionDto> rows,
        List<PortfolioTransactionUpsertDto> importedRows,
        Dictionary<string, string> rowWarnings)
    {
        if (rowWarnings.Count == 0) return rows;

        var warningsBySignature = importedRows
            .Where(row => rowWarnings.ContainsKey(RowKey(row)))
            .GroupBy(row => ImportSignature(row))
            .ToDictionary(g => g.Key, g => rowWarnings[RowKey(g.First())]);

        return rows.Select(row =>
            warningsBySignature.TryGetValue(ImportSignature(row), out var warning)
                ? row with { Error = warning }
                : row).ToList();
    }

    private static string BuildImportMessage(int importedCount, int invalidCount, int outOfRangeCount, int warningCount)
    {
        var parts = new List<string> { $"{importedCount} transaction{(importedCount == 1 ? "" : "s")} imported" };
        if (outOfRangeCount > 0)
            parts.Add($"{outOfRangeCount} row{(outOfRangeCount == 1 ? "" : "s")} skipped outside selected date range");
        if (invalidCount > 0)
            parts.Add($"{invalidCount} row{(invalidCount == 1 ? "" : "s")} skipped with errors");
        if (warningCount > 0)
            parts.Add($"{warningCount} row{(warningCount == 1 ? "" : "s")} need review");
        return string.Join(". ", parts) + ".";
    }

    private static string? NormalizeOptional(string? value) =>
        string.IsNullOrWhiteSpace(value) ? null : value.Trim();

    private static string ClassifyAsset(Fund fund)
    {
        var text = $"{fund.SchemeType} {fund.SchemeCategory} {fund.SchemeName}".ToLowerInvariant();
        if (text.Contains("gold")) return "Gold";
        if (text.Contains("international") || text.Contains("overseas") || text.Contains("global")) return "International";
        if (text.Contains("debt") || text.Contains("liquid") || text.Contains("gilt") || text.Contains("bond")) return "Debt";
        if (text.Contains("equity") || text.Contains("elss") || text.Contains("index")) return "Equity";
        return "Other";
    }

    private static decimal SignedUnits(PortfolioTransaction t) => IsBuy(t.TransactionType) ? t.Units : -t.Units;
    private static bool IsBuy(string type) => BuyTypes.Contains(type, StringComparer.OrdinalIgnoreCase);
    private static bool IsSell(string type) => SellTypes.Contains(type, StringComparer.OrdinalIgnoreCase);
    private static string NormalizeType(string type) => IsSell(type) ? "Sell" : "Buy";
    private static string RowKey(PortfolioTransactionUpsertDto row) => row.ClientRowId ?? row.Id?.ToString() ?? $"{row.SchemeCode}:{row.TransactionDate}:{row.Units}:{row.Amount}";
    private static string Signature(PortfolioTransaction row, int index) => $"{index}:{row.SchemeCode}:{row.TransactionDate}:{row.Units}:{row.Amount}";
    private static string ImportSignature(PortfolioTransactionUpsertDto row) => $"{row.SchemeCode}:{row.TransactionDate}:{row.Units}:{row.Amount}";
    private static string ImportSignature(PortfolioTransactionDto row) => $"{row.SchemeCode}:{row.TransactionDate}:{row.Units}:{row.Amount}";

    private record NamedMetric(string Name, decimal InvestedAmount, decimal CurrentValue, decimal AbsoluteReturnPercent, decimal? XirrPercent)
        : PortfolioBenchmarkMetricDto(InvestedAmount, CurrentValue, AbsoluteReturnPercent, XirrPercent);

    private record FundLedger(decimal Units, decimal RemainingCost, decimal Invested, decimal RealizedPnl);
}
