using Microsoft.EntityFrameworkCore;
using NAVigator.Data;
using NAVigator.Models.Database;
using NAVigator.Models.Dto.Portfolio;

namespace NAVigator.Services;

public class UserPortfolioService(AppDbContext db) : IUserPortfolioService
{
    private static readonly string[] BuyTypes = ["Buy", "Purchase", "SIP", "SwitchIn"];
    private static readonly string[] SellTypes = ["Sell", "Redeem", "Redemption", "SwitchOut"];

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
        var fundInsights = BuildFundInsights(transactions, latestNavs, emptyStates);
        var visibleFunds = showFullyExitedFunds ? fundInsights : fundInsights.Where(f => !f.IsFullyExited).ToList();
        var metrics = BuildMetrics(fundInsights, transactions);
        var fundAllocation = BuildAllocation(
            fundInsights.Where(f => f.CurrentValue > 0).Select(f => (f.SchemeName, f.CurrentValue)));

        return new PortfolioDashboardDto(
            portfolio.Id,
            portfolio.Name,
            metrics,
            visibleFunds,
            BuildAllocation(fundInsights.Where(f => f.CurrentValue > 0).Select(f => (f.AssetClass, f.CurrentValue))),
            fundAllocation,
            BuildContributions(transactions, financialYear: true),
            BuildContributions(transactions, financialYear: false),
            BuildSipConsistency(transactions),
            emptyStates);
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
        FlagNegativeUnitRows(existingOutsideYear.Concat(staged).ToList(), staged, rowErrors, request.Transactions);

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
        return new SavePortfolioTransactionsResponse(true, "Transactions saved.", staged.Count, result!.Value.Rows, result.Value.Summary);
    }

    public async Task<PortfolioBenchmarkResponse?> CompareBenchmarkAsync(int userId, long portfolioId, PortfolioBenchmarkRequest request, CancellationToken ct)
    {
        var dashboard = await GetDashboardAsync(userId, portfolioId, true, ct);
        if (dashboard is null) return null;

        var transactions = await LoadTransactions(portfolioId, ct);
        var portfolioRedemptions = transactions.Where(t => IsSell(t.TransactionType)).Sum(t => t.Amount);
        var benchmark = request.BenchmarkType.Equals("basket", StringComparison.OrdinalIgnoreCase) && request.Basket is not null
            ? await SimulateBenchmarkBasket(transactions, request.Basket, ct)
            : await SimulateBenchmarkFund(transactions, request.SchemeCode, ct);

        var portfolioMetric = new PortfolioBenchmarkMetricDto(
            dashboard.Metrics.InvestedAmount,
            dashboard.Metrics.CurrentValue + portfolioRedemptions,
            dashboard.Metrics.AbsoluteReturnPercent,
            dashboard.Metrics.XirrPercent);

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

        return new PortfolioBenchmarkResponse(benchmark.Name, portfolioMetric, benchmark, alpha, verdict);
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
            var totalGain = redemptions + currentValue - invested;
            var xirr = BuildXirr(group.ToList(), currentValue, latestNav?.Date);
            var firstDate = group.Min(t => t.TransactionDate);
            var lastDate = units > 0 && latestNav is not null ? latestNav.Date : group.Max(t => t.TransactionDate);

            if (units > 0 && latestNav is null)
                emptyStates.Add($"{first.Fund.SchemeName} has units but no NAV is available.");

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
                units <= 0,
                latestNav?.Nav,
                latestNav?.Date.ToString("dd-MM-yyyy"),
                latestNav is not null));
        }
        return insights.OrderByDescending(f => f.CurrentValue).ThenBy(f => f.SchemeName).ToList();
    }

    private static PortfolioMetricsDto BuildMetrics(List<PortfolioFundInsightDto> funds, List<PortfolioTransaction> transactions)
    {
        var invested = funds.Sum(f => f.InvestedAmount);
        var current = funds.Sum(f => f.CurrentValue);
        var realizedPnl = funds.Sum(f => f.RealizedPnl);
        var unrealizedPnl = funds.Sum(f => f.UnrealizedPnl);
        var redemptions = transactions.Where(t => IsSell(t.TransactionType)).Sum(t => t.Amount);
        var gain = current + redemptions - invested;
        var start = transactions.Count == 0 ? DateOnly.FromDateTime(DateTime.UtcNow) : transactions.Min(t => t.TransactionDate);
        var end = DateOnly.FromDateTime(DateTime.UtcNow);
        var xirr = BuildXirr(transactions, current, end);

        return new PortfolioMetricsDto(
            invested,
            current,
            realizedPnl,
            unrealizedPnl,
            invested == 0 ? 0 : Math.Round(gain / invested * 100, 2),
            CalculateCagr(invested, current + redemptions, start, end),
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

    private async Task<NamedMetric> SimulateBenchmarkFund(List<PortfolioTransaction> rows, int? schemeCode, CancellationToken ct)
    {
        if (schemeCode is null) return new NamedMetric("Select a benchmark", 0, 0, 0, null);
        var fund = await db.Funds.AsNoTracking().FirstOrDefaultAsync(f => f.SchemeCode == schemeCode.Value, ct);
        var navs = await db.NavHistories.AsNoTracking().Where(n => n.SchemeCode == schemeCode.Value).OrderBy(n => n.Date).ToListAsync(ct);
        return SimulateBenchmark(rows, [(schemeCode.Value, 100m)], navs.ToLookup(n => n.SchemeCode), fund?.SchemeName ?? "Benchmark");
    }

    private async Task<NamedMetric> SimulateBenchmarkBasket(List<PortfolioTransaction> rows, BasketDefinition basket, CancellationToken ct)
    {
        var codes = basket.Funds.Select(f => f.SchemeCode).ToList();
        var navs = await db.NavHistories.AsNoTracking().Where(n => codes.Contains(n.SchemeCode)).OrderBy(n => n.Date).ToListAsync(ct);
        return SimulateBenchmark(rows, basket.Funds.Select(f => (f.SchemeCode, f.AllocationPercent)).ToList(), navs.ToLookup(n => n.SchemeCode), basket.Name);
    }

    private static NamedMetric SimulateBenchmark(List<PortfolioTransaction> rows, List<(int SchemeCode, decimal Allocation)> basket, ILookup<int, NavHistory> navs, string name)
    {
        var units = basket.ToDictionary(b => b.SchemeCode, _ => 0m);
        var invested = 0m;
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
        }

        var current = 0m;
        foreach (var item in basket)
        {
            var latest = navs[item.SchemeCode].LastOrDefault();
            if (latest is not null) current += units[item.SchemeCode] * latest.Nav;
        }
        current = Math.Round(current, 2);
        var cashFlows = rows.Select(t => (t.TransactionDate.ToDateTime(TimeOnly.MinValue), (double)(IsBuy(t.TransactionType) ? -t.Amount : t.Amount))).ToList();
        cashFlows.Add((DateTime.UtcNow.Date, (double)current));
        return new NamedMetric(name, invested, current, invested == 0 ? 0 : Math.Round((current - invested) / invested * 100, 2), XirrCalculator.Calculate(cashFlows));
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

    private static string ToFinancialYear(DateOnly date)
    {
        var startYear = date.Month >= 4 ? date.Year : date.Year - 1;
        return $"{startYear}-{(startYear + 1) % 100:00}";
    }

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

    private record NamedMetric(string Name, decimal InvestedAmount, decimal CurrentValue, decimal AbsoluteReturnPercent, decimal? XirrPercent)
        : PortfolioBenchmarkMetricDto(InvestedAmount, CurrentValue, AbsoluteReturnPercent, XirrPercent);

    private record FundLedger(decimal Units, decimal RemainingCost, decimal Invested, decimal RealizedPnl);
}
