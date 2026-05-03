namespace NAVigator.Models.Dto.Portfolio;

public class SimulationResponse
{
    public required string BasketName { get; set; }
    public decimal TotalInvested { get; set; }
    public decimal FinalCorpusValue { get; set; }
    public decimal AbsoluteReturnPercent { get; set; }
    public decimal? Cagr { get; set; }
    public decimal? Xirr { get; set; }
    public decimal? MaxDrawdownPercent { get; set; }

    /// <summary>
    /// Annualized standard deviation of monthly portfolio returns.
    /// </summary>
    public decimal? Volatility { get; set; }

    public required string StartDate { get; set; }
    public required string EndDate { get; set; }
    public int SipInstallments { get; set; }
    public decimal SipAmount { get; set; }
    public decimal LumpSumAmount { get; set; }

    /// <summary>
    /// Per-fund breakdown of the simulation results.
    /// </summary>
    public List<FundSimulationDetail> FundDetails { get; set; } = [];

    /// <summary>
    /// Time-series snapshots for charting portfolio growth over time.
    /// Sampled at monthly intervals to keep response sizes reasonable.
    /// </summary>
    public List<PortfolioSnapshot> TimeSeries { get; set; } = [];
}

public class FundSimulationDetail
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public decimal AllocationPercent { get; set; }
    public decimal TotalUnits { get; set; }
    public decimal TotalInvested { get; set; }
    public decimal FinalValue { get; set; }
    public decimal AbsoluteReturnPercent { get; set; }
}

public class PortfolioSnapshot
{
    /// <summary>Date in dd-MM-yyyy format.</summary>
    public required string Date { get; set; }
    public decimal PortfolioValue { get; set; }
    public decimal TotalInvested { get; set; }
    public decimal GainPercent { get; set; }

    /// <summary>Per-fund breakdown at this point in time.</summary>
    public List<FundSnapshot> Funds { get; set; } = [];
}

public class FundSnapshot
{
    public int SchemeCode { get; set; }
    public decimal Value { get; set; }
    public decimal Units { get; set; }
}
