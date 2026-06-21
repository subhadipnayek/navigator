using System.Globalization;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public static class IndiaTime
{
    public static DateTime Now()
        => TimeZoneInfo.ConvertTimeFromUtc(DateTime.UtcNow, TimeZoneInfo.FindSystemTimeZoneById("India Standard Time"));

    public static DateOnly Today()
        => DateOnly.FromDateTime(Now());
}

public static class CorporateActionRules
{
    public static readonly string[] ValidActionTypes = ["Split", "Merger", "Bonus", "Discontinuity", "MissingData", "Other"];
    public static readonly decimal[] CommonAdjustmentFactors = [2m, 3m, 4m, 5m, 10m, 100m];

    public static bool IsSeriesBreakAction(string actionType)
        => actionType.Equals("Discontinuity", StringComparison.OrdinalIgnoreCase)
           || actionType.Equals("MissingData", StringComparison.OrdinalIgnoreCase);

    public static decimal ComputeAdjustmentFactor(IEnumerable<CorporateActionDto> actions, DateOnly fromDate, DateOnly toDate)
    {
        var factor = 1m;
        foreach (var action in actions)
        {
            if (!TryParseActionDate(action.ActionDate, out var actionDate))
                continue;

            if (IsSeriesBreakAction(action.ActionType))
                continue;

            if (actionDate > fromDate && actionDate <= toDate)
                factor *= action.AdjustmentFactor;
        }

        return factor;
    }

    public static List<DateOnly> GetSeriesBreakDates(IEnumerable<CorporateActionDto> actions)
        => actions
            .Where(a => IsSeriesBreakAction(a.ActionType))
            .Select(a => TryParseActionDate(a.ActionDate, out var date) ? date : (DateOnly?)null)
            .Where(d => d.HasValue)
            .Select(d => d!.Value)
            .ToList();

    public static bool TryParseActionDate(string value, out DateOnly date)
        => DateOnly.TryParseExact(value, "dd-MM-yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None, out date);
}

public static class NavCalculationRules
{
    public const int DefaultNavToleranceDays = 7;

    public static NavHistory? FindNavOnOrBefore(List<NavHistory> sortedAsc, DateOnly target)
    {
        int lo = 0, hi = sortedAsc.Count - 1;
        NavHistory? best = null;
        while (lo <= hi)
        {
            int mid = (lo + hi) / 2;
            if (sortedAsc[mid].Date <= target)
            {
                best = sortedAsc[mid];
                lo = mid + 1;
            }
            else
            {
                hi = mid - 1;
            }
        }

        return best;
    }

    public static NavHistory? FindNavOnOrAfter(
        List<NavHistory> sortedAsc,
        DateOnly target,
        int toleranceDays = DefaultNavToleranceDays,
        bool allowPreviousAtEnd = true)
    {
        int lo = 0, hi = sortedAsc.Count - 1;
        NavHistory? best = null;
        while (lo <= hi)
        {
            int mid = (lo + hi) / 2;
            if (sortedAsc[mid].Date >= target)
            {
                best = sortedAsc[mid];
                hi = mid - 1;
            }
            else
            {
                lo = mid + 1;
            }
        }

        if (best is null && allowPreviousAtEnd && sortedAsc.Count > 0)
        {
            var last = sortedAsc[^1];
            if (target.DayNumber - last.Date.DayNumber <= toleranceDays)
                return last;
        }

        if (best is not null && best.Date.DayNumber - target.DayNumber > toleranceDays)
            return null;

        return best;
    }

    public static NavHistory? FindClosestNav(
        List<NavHistory> sortedAsc,
        DateOnly target,
        int toleranceDays = DefaultNavToleranceDays)
    {
        var before = FindNavOnOrBefore(sortedAsc, target);
        if (before is not null && target.DayNumber - before.Date.DayNumber <= toleranceDays)
            return before;

        var after = FindNavOnOrAfter(sortedAsc, target, toleranceDays, allowPreviousAtEnd: false);
        if (after is not null && after.Date.DayNumber - target.DayNumber <= toleranceDays)
            return after;

        return null;
    }

    public static (DateOnly Date, decimal Nav)? FindClosestNavValue(
        List<NavHistory> sortedAsc,
        DateOnly target,
        int toleranceDays = DefaultNavToleranceDays)
    {
        var entry = FindClosestNav(sortedAsc, target, toleranceDays);
        return entry is null ? null : (entry.Date, entry.Nav);
    }

    public static List<T> SampleByInterval<T>(List<T> sortedAsc, string? interval, Func<T, DateOnly> dateSelector)
    {
        if (string.IsNullOrEmpty(interval) || interval.Equals("daily", StringComparison.OrdinalIgnoreCase))
            return sortedAsc;

        if (interval.Equals("weekly", StringComparison.OrdinalIgnoreCase))
        {
            return sortedAsc
                .GroupBy(item =>
                {
                    var dt = dateSelector(item).ToDateTime(TimeOnly.MinValue);
                    return (ISOWeek.GetYear(dt), ISOWeek.GetWeekOfYear(dt));
                })
                .Select(g => g.Last())
                .ToList();
        }

        if (interval.Equals("monthly", StringComparison.OrdinalIgnoreCase))
        {
            return sortedAsc
                .GroupBy(item =>
                {
                    var date = dateSelector(item);
                    return (date.Year, date.Month);
                })
                .Select(g => g.Last())
                .ToList();
        }

        return sortedAsc;
    }

    public static decimal? CalculateCagr(decimal startValue, decimal endValue, DateOnly startDate, DateOnly endDate)
    {
        if (startValue <= 0 || endValue <= 0 || endDate <= startDate)
            return null;

        var years = (endDate.DayNumber - startDate.DayNumber) / 365.25d;
        if (years <= 0)
            return null;

        return (decimal)(Math.Pow((double)(endValue / startValue), 1 / years) - 1) * 100m;
    }

    public static decimal? GetMedian(List<decimal> sortedValues)
        => sortedValues.Count switch
        {
            0 => null,
            _ when sortedValues.Count % 2 == 0 => Math.Round((sortedValues[sortedValues.Count / 2 - 1] + sortedValues[sortedValues.Count / 2]) / 2m, 2),
            _ => sortedValues[sortedValues.Count / 2]
        };
}
