namespace NAVigator.Models.Dto.Portfolio;

public class ComparisonResponse
{
    public required string StartDate { get; set; }
    public required string EndDate { get; set; }
    public bool IsNormalized { get; set; }
    public List<ComparisonEntryResult> Results { get; set; } = [];

    /// <summary>
    /// Aligned time-series for overlay charting across all entries.
    /// </summary>
    public List<ComparisonTimePoint> TimeSeries { get; set; } = [];
}

public class ComparisonEntryResult
{
    public required string Name { get; set; }
    public decimal TotalInvested { get; set; }
    public decimal FinalCorpusValue { get; set; }
    public decimal AbsoluteReturnPercent { get; set; }
    public decimal? Cagr { get; set; }
    public decimal? Xirr { get; set; }
    public decimal? MaxDrawdownPercent { get; set; }
    public decimal? Volatility { get; set; }

    /// <summary>
    /// CAGR difference vs the first entry (treated as benchmark). Null for the first entry.
    /// </summary>
    public decimal? AlphaVsFirst { get; set; }
}

public class ComparisonTimePoint
{
    /// <summary>Date in dd-MM-yyyy format.</summary>
    public required string Date { get; set; }
    public List<ComparisonSeriesValue> Values { get; set; } = [];
}

public class ComparisonSeriesValue
{
    public required string Name { get; set; }
    public decimal PortfolioValue { get; set; }

    /// <summary>
    /// Portfolio value per ₹1 invested — enables fair visual comparison
    /// across entries with different contribution amounts.
    /// </summary>
    public decimal NormalizedValue { get; set; }
}
