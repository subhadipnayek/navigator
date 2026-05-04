namespace NAVigator.Models.Dto.Portfolio;

public class SimulationRequest
{
    /// <summary>
    /// The basket (portfolio) to simulate.
    /// </summary>
    public required BasketDefinition Basket { get; set; }

    /// <summary>
    /// Monthly SIP amount in ₹. Set to 0 for lump-sum-only simulation.
    /// </summary>
    public decimal SipAmount { get; set; }

    /// <summary>
    /// Day of month on which SIP is processed (1–28). If the date falls on a
    /// non-trading day, the next available NAV date is used.
    /// </summary>
    public int SipDay { get; set; } = 1;

    /// <summary>
    /// One-time lump sum invested on the start date. Set to 0 for SIP-only simulation.
    /// </summary>
    public decimal LumpSumAmount { get; set; }

    /// <summary>
    /// Simulation start date (dd-MM-yyyy).
    /// </summary>
    public required string StartDate { get; set; }

    /// <summary>
    /// Simulation end date (dd-MM-yyyy).
    /// </summary>
    public required string EndDate { get; set; }
}
