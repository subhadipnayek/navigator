namespace NAVigator.Models.Dto.Portfolio;

public class WhatIfResponse
{
    public required string StartDate { get; set; }
    public required string EndDate { get; set; }
    public List<WhatIfScenarioResult> Scenarios { get; set; } = [];

    /// <summary>
    /// Auto-generated insights derived from comparing all scenarios.
    /// </summary>
    public WhatIfInsights Insights { get; set; } = new();
}

public class WhatIfScenarioResult
{
    public required string Name { get; set; }
    public decimal SipAmount { get; set; }
    public decimal LumpSumAmount { get; set; }
    public decimal TotalInvested { get; set; }
    public decimal FinalCorpusValue { get; set; }
    public decimal AbsoluteReturnPercent { get; set; }
    public decimal? Cagr { get; set; }
    public decimal? Xirr { get; set; }
    public decimal? MaxDrawdownPercent { get; set; }
    public decimal? Volatility { get; set; }

    /// <summary>
    /// Final corpus value divided by total capital deployed.
    /// Higher = better capital efficiency.
    /// </summary>
    public decimal CorpusPerRupeeInvested { get; set; }
}

public class WhatIfInsights
{
    /// <summary>Scenario with the highest absolute final corpus.</summary>
    public string? BestAbsoluteReturner { get; set; }

    /// <summary>Scenario with the highest corpus per ₹1 invested.</summary>
    public string? BestCapitalEfficiency { get; set; }

    /// <summary>Scenario with the shallowest max drawdown.</summary>
    public string? LowestDrawdown { get; set; }

    /// <summary>
    /// True if any scenario with a lower total investment outperformed
    /// a scenario with higher total investment in absolute corpus terms.
    /// </summary>
    public bool AllocationBeatCapital { get; set; }

    /// <summary>
    /// Human-readable description of which lower-capital scenario
    /// outperformed which higher-capital scenario, if any.
    /// </summary>
    public string? AllocationBeatCapitalDetails { get; set; }
}
