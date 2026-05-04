namespace NAVigator.Services;

/// <summary>
/// Newton-Raphson XIRR solver for cash-flow-aware annualized return calculation.
/// Finds the discount rate at which the net present value of all cash flows equals zero.
/// </summary>
public static class XirrCalculator
{
    private const int MaxIterations = 200;
    private const double Tolerance = 1e-10;
    private const double DaysPerYear = 365.25;

    /// <summary>
    /// Computes XIRR (extended internal rate of return) for a series of cash flows.
    /// </summary>
    /// <param name="cashFlows">
    /// List of (date, amount) tuples.
    /// Negative amounts = outflows (investments), positive amounts = inflows (redemptions/final value).
    /// Must contain at least one negative and one positive cash flow.
    /// </param>
    /// <returns>Annualized return as a decimal percentage (e.g., 14.87 for 14.87%), or null if non-convergent.</returns>
    public static decimal? Calculate(List<(DateTime Date, double Amount)> cashFlows)
    {
        if (cashFlows.Count < 2)
            return null;

        bool hasNegative = false, hasPositive = false;
        foreach (var cf in cashFlows)
        {
            if (cf.Amount < 0) hasNegative = true;
            if (cf.Amount > 0) hasPositive = true;
            if (hasNegative && hasPositive) break;
        }
        if (!hasNegative || !hasPositive)
            return null;

        var baseDate = cashFlows[0].Date;

        // Pre-compute day fractions for performance
        var dayFractions = new double[cashFlows.Count];
        var amounts = new double[cashFlows.Count];
        for (int i = 0; i < cashFlows.Count; i++)
        {
            dayFractions[i] = (cashFlows[i].Date - baseDate).TotalDays / DaysPerYear;
            amounts[i] = cashFlows[i].Amount;
        }

        // Initial guess: try 10% first
        double rate = 0.1;

        for (int iter = 0; iter < MaxIterations; iter++)
        {
            double npv = 0;
            double dnpv = 0; // derivative of NPV with respect to rate

            for (int i = 0; i < cashFlows.Count; i++)
            {
                double t = dayFractions[i];
                double denom = Math.Pow(1.0 + rate, t);

                if (denom == 0 || double.IsNaN(denom) || double.IsInfinity(denom))
                    break;

                npv += amounts[i] / denom;
                dnpv -= t * amounts[i] / (denom * (1.0 + rate));
            }

            if (double.IsNaN(npv) || double.IsInfinity(npv))
                break;

            if (Math.Abs(npv) < Tolerance)
                return Math.Round((decimal)(rate * 100), 4);

            if (Math.Abs(dnpv) < 1e-20)
                break; // Derivative too small — would cause huge step

            double newRate = rate - npv / dnpv;

            // Guard against divergence: clamp step size
            if (newRate < -0.999)
                newRate = (rate - 0.999) / 2;
            if (newRate > 100)
                newRate = (rate + 100) / 2;

            rate = newRate;
        }

        // If Newton-Raphson failed, try bisection as fallback
        return BisectionFallback(amounts, dayFractions);
    }

    /// <summary>
    /// Bisection fallback for cases where Newton-Raphson diverges (e.g., very volatile cash flows).
    /// </summary>
    private static decimal? BisectionFallback(double[] amounts, double[] dayFractions)
    {
        double lo = -0.99, hi = 10.0;

        double npvLo = ComputeNpv(amounts, dayFractions, lo);
        double npvHi = ComputeNpv(amounts, dayFractions, hi);

        // NPV must change sign between lo and hi for bisection to work
        if (npvLo * npvHi > 0)
            return null;

        for (int iter = 0; iter < MaxIterations * 2; iter++)
        {
            double mid = (lo + hi) / 2;
            double npvMid = ComputeNpv(amounts, dayFractions, mid);

            if (Math.Abs(npvMid) < Tolerance || (hi - lo) < Tolerance)
                return Math.Round((decimal)(mid * 100), 4);

            if (npvMid * npvLo < 0)
            {
                hi = mid;
                npvHi = npvMid;
            }
            else
            {
                lo = mid;
                npvLo = npvMid;
            }
        }

        return null;
    }

    private static double ComputeNpv(double[] amounts, double[] dayFractions, double rate)
    {
        double npv = 0;
        for (int i = 0; i < amounts.Length; i++)
        {
            double denom = Math.Pow(1.0 + rate, dayFractions[i]);
            if (denom == 0 || double.IsNaN(denom) || double.IsInfinity(denom))
                return double.NaN;
            npv += amounts[i] / denom;
        }
        return npv;
    }
}
