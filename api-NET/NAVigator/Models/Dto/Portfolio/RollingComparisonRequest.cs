namespace NAVigator.Models.Dto.Portfolio;

/// <summary>
/// Rolling returns comparison across multiple baskets/funds.
/// For each entry, computes rolling CAGR for every possible window of the given period,
/// enabling "which basket consistently outperforms across time?" analysis.
/// </summary>
public class RollingComparisonRequest
{
    /// <summary>2–10 entries (baskets or single funds) to compare.</summary>
    public required List<ComparisonEntry> Entries { get; set; }

    /// <summary>Rolling window length in years (1–30).</summary>
    public int PeriodInYears { get; set; } = 3;

    /// <summary>Start of the analysis window (dd-MM-yyyy).</summary>
    public required string StartDate { get; set; }

    /// <summary>End of the analysis window (dd-MM-yyyy).</summary>
    public required string EndDate { get; set; }

    /// <summary>Sampling interval: daily, weekly, or monthly. Null = daily.</summary>
    public string? Interval { get; set; }
}

public class RollingComparisonResponse
{
    public required string StartDate { get; set; }
    public required string EndDate { get; set; }
    public int PeriodInYears { get; set; }
    public List<RollingComparisonEntryResult> Results { get; set; } = [];

    /// <summary>Aligned time-series for overlay charting of rolling returns.</summary>
    public List<RollingComparisonTimePoint> TimeSeries { get; set; } = [];
}

public class RollingComparisonEntryResult
{
    public required string Name { get; set; }
    public int DataPoints { get; set; }
    public decimal? MinReturn { get; set; }
    public decimal? MaxReturn { get; set; }
    public decimal? AverageReturn { get; set; }
    public decimal? MedianReturn { get; set; }
}

public class RollingComparisonTimePoint
{
    /// <summary>Date in dd-MM-yyyy format.</summary>
    public required string Date { get; set; }
    public List<RollingComparisonSeriesValue> Values { get; set; } = [];
}

public class RollingComparisonSeriesValue
{
    public required string Name { get; set; }
    public decimal? ReturnPercent { get; set; }
}
