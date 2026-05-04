using NAVigator.Models.Dto.Portfolio;

namespace NAVigator.Services;

public interface IPortfolioSimulationService
{
    /// <summary>
    /// Simulates SIP and/or lump-sum investment in a basket of mutual funds
    /// using real NAV data, producing detailed metrics and time-series output.
    /// </summary>
    Task<SimulationResponse> SimulateAsync(SimulationRequest request, CancellationToken ct = default);

    /// <summary>
    /// Compares multiple baskets/funds side-by-side with aligned timelines.
    /// Supports normalization for fair comparison across different SIP amounts.
    /// </summary>
    Task<ComparisonResponse> CompareAsync(ComparisonRequest request, CancellationToken ct = default);

    /// <summary>
    /// Runs what-if analysis across multiple allocation/contribution scenarios,
    /// generating insights about capital efficiency and allocation sensitivity.
    /// </summary>
    Task<WhatIfResponse> WhatIfAsync(WhatIfRequest request, CancellationToken ct = default);

    /// <summary>
    /// Computes rolling returns (annualized CAGR) for multiple baskets/funds,
    /// enabling "which consistently outperforms across time?" analysis.
    /// </summary>
    Task<RollingComparisonResponse> RollingComparisonAsync(RollingComparisonRequest request, CancellationToken ct = default);
}
