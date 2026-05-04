namespace NAVigator.Models.Dto.Portfolio;

public class BasketDefinition
{
    /// <summary>
    /// Human-readable label for this basket (e.g., "Large Cap Mix", "Aggressive Growth").
    /// </summary>
    public required string Name { get; set; }

    /// <summary>
    /// Constituent funds and their target allocation weights.
    /// Allocation percentages must sum to exactly 100.
    /// </summary>
    public required List<BasketFundAllocation> Funds { get; set; }

    /// <summary>
    /// How often to rebalance the portfolio back to target allocations.
    /// Supported values: None, Monthly, Quarterly, Yearly.
    /// </summary>
    public string RebalanceFrequency { get; set; } = "None";
}

public class BasketFundAllocation
{
    public int SchemeCode { get; set; }

    /// <summary>
    /// Target allocation as a percentage (0–100). All allocations in a basket must sum to 100.
    /// </summary>
    public decimal AllocationPercent { get; set; }
}
