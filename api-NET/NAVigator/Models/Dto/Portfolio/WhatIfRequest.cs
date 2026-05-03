namespace NAVigator.Models.Dto.Portfolio;

/// <summary>
/// What-if analysis request — tests multiple allocation/contribution strategies
/// to answer questions like "Can a better allocation beat higher capital?"
/// </summary>
public class WhatIfRequest
{
    /// <summary>
    /// 2–10 scenarios to evaluate. Each has its own basket, SIP, and lump-sum configuration.
    /// </summary>
    public required List<WhatIfScenario> Scenarios { get; set; }

    /// <summary>Simulation start date (dd-MM-yyyy).</summary>
    public required string StartDate { get; set; }

    /// <summary>Simulation end date (dd-MM-yyyy).</summary>
    public required string EndDate { get; set; }
}

public class WhatIfScenario
{
    /// <summary>Label for this scenario (e.g., "Low SIP + Quality Allocation").</summary>
    public required string Name { get; set; }

    public required BasketDefinition Basket { get; set; }
    public decimal SipAmount { get; set; }
    public int SipDay { get; set; } = 1;
    public decimal LumpSumAmount { get; set; }
}
