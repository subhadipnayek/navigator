namespace NAVigator.Models.Dto.Portfolio;

/// <summary>
/// Compares multiple baskets and/or individual funds over the same time period.
/// Supports basket-vs-basket, basket-vs-fund, and fund-vs-fund comparisons.
/// </summary>
public class ComparisonRequest
{
    /// <summary>
    /// 2–10 entries to compare. Each entry is either a basket or a single fund.
    /// </summary>
    public required List<ComparisonEntry> Entries { get; set; }

    /// <summary>Start date for all comparisons (dd-MM-yyyy).</summary>
    public required string StartDate { get; set; }

    /// <summary>End date for all comparisons (dd-MM-yyyy).</summary>
    public required string EndDate { get; set; }

    /// <summary>
    /// When true, results are normalized to show returns per ₹1 invested,
    /// enabling fair comparison even when SIP amounts differ across entries.
    /// </summary>
    public bool NormalizeSipAmounts { get; set; }
}

/// <summary>
/// A single entry in a comparison — either a basket or a single fund with its own SIP/lump-sum config.
/// </summary>
public class ComparisonEntry
{
    /// <summary>Label for this entry in comparison charts and tables.</summary>
    public required string Name { get; set; }

    public decimal SipAmount { get; set; }
    public int SipDay { get; set; } = 1;
    public decimal LumpSumAmount { get; set; }

    /// <summary>
    /// Set this for a basket (portfolio) entry. Mutually exclusive with SingleFundSchemeCode.
    /// </summary>
    public BasketDefinition? Basket { get; set; }

    /// <summary>
    /// Set this for a single-fund entry. Mutually exclusive with Basket.
    /// When set, the engine creates an implicit 1-fund basket with 100% allocation.
    /// </summary>
    public int? SingleFundSchemeCode { get; set; }
}
