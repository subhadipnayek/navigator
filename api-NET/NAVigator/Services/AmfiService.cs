using System.Globalization;
using System.Text.Json.Serialization;
using System.Text.Json;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public class AmfiService(IHttpClientFactory httpClientFactory, ILogger<AmfiService> logger) : IAmfiService
{
    private const string AmfiMasterUrl = "https://portal.amfiindia.com/spages/NAVAll.txt";
    private const string AmfiNavHistoryUrl = "https://www.amfiindia.com/api/nav-history";
    private const int NavHistoryChunkYears = 4;
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        NumberHandling = JsonNumberHandling.AllowReadingFromString
    };

    public async Task<List<Fund>> FetchFundMasterListAsync(CancellationToken ct = default)
    {
        var client = httpClientFactory.CreateClient("Amfi");
        var response = await client.GetStringAsync(AmfiMasterUrl, ct);
        return ParseAmfiMasterData(response);
    }

    public async Task<MfApiResponse?> FetchNavHistoryAsync(
        int schemeCode,
        DateOnly from,
        DateOnly to,
        CancellationToken ct = default,
        bool stopAtFirstOlderEmptyChunk = true)
    {
        if (from > to)
            return new MfApiResponse();

        var client = httpClientFactory.CreateClient("AmfiHistory");
        var entriesByDate = new Dictionary<DateOnly, MfApiNavEntry>();
        var foundAnyRecords = false;

        foreach (var (chunkFrom, chunkTo) in BuildHistoryChunks(from, to))
        {
            var url = BuildNavHistoryUrl(schemeCode, chunkFrom, chunkTo);

            try
            {
                var response = await client.GetStringAsync(url, ct);

                var amfiResponse = JsonSerializer.Deserialize<AmfiNavHistoryResponse>(response, JsonOptions);
                var chunkHadRecords = false;
                if (amfiResponse?.Data?.NavGroups is null || amfiResponse.Data.NavGroups.Count == 0)
                {
                    logger.LogDebug(
                        "AMFI returned no NAV records for scheme {SchemeCode} from {From} to {To}: {Message}",
                        schemeCode,
                        chunkFrom,
                        chunkTo,
                        amfiResponse?.Message ?? "empty response");

                    if (foundAnyRecords && stopAtFirstOlderEmptyChunk)
                        break;

                    continue;
                }

                foreach (var record in amfiResponse.Data.NavGroups.SelectMany(group => group.HistoricalRecords))
                {
                    if (!DateOnly.TryParseExact(record.Date, "yyyy-MM-dd", CultureInfo.InvariantCulture,
                            DateTimeStyles.None, out var date))
                        continue;

                    if (record.Nav <= 0)
                        continue;

                    chunkHadRecords = true;
                    entriesByDate[date] = new MfApiNavEntry
                    {
                        Date = date.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture),
                        Nav = record.Nav
                    };
                }

                if (chunkHadRecords)
                    foundAnyRecords = true;
                else if (foundAnyRecords && stopAtFirstOlderEmptyChunk)
                    break;
            }
            catch (HttpRequestException ex)
            {
                logger.LogWarning(
                    ex,
                    "Failed to fetch AMFI NAV history for scheme {SchemeCode} from {From} to {To}",
                    schemeCode,
                    chunkFrom,
                    chunkTo);
                return null;
            }
            catch (JsonException ex)
            {
                logger.LogWarning(
                    ex,
                    "Failed to parse AMFI NAV history for scheme {SchemeCode} from {From} to {To}",
                    schemeCode,
                    chunkFrom,
                    chunkTo);
                return null;
            }
        }

        return new MfApiResponse
        {
            Data = entriesByDate
                .OrderBy(entry => entry.Key)
                .Select(entry => entry.Value)
                .ToList()
        };
    }

    private static IEnumerable<(DateOnly From, DateOnly To)> BuildHistoryChunks(DateOnly from, DateOnly to)
    {
        var chunkTo = to;
        while (chunkTo >= from)
        {
            var chunkFrom = chunkTo.AddYears(-NavHistoryChunkYears).AddDays(1);
            if (chunkFrom < from)
                chunkFrom = from;

            yield return (chunkFrom, chunkTo);
            chunkTo = chunkFrom.AddDays(-1);
        }
    }

    private static string BuildNavHistoryUrl(int schemeCode, DateOnly from, DateOnly to)
    {
        var fromDate = from.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture);
        var toDate = to.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture);
        return $"{AmfiNavHistoryUrl}?query_type=historical_period&from_date={fromDate}&to_date={toDate}&sd_id={schemeCode}";
    }

    private List<Fund> ParseAmfiMasterData(string rawData)
    {
        var funds = new List<Fund>();
        var lines = rawData.Split('\n', StringSplitOptions.None);

        string? currentSchemeType = null;
        string? currentSchemeCategory = null;
        string? currentFundHouse = null;
        bool expectFundHouse = false;

        foreach (var rawLine in lines)
        {
            var line = rawLine.Trim();

            // Blank line — next non-blank, non-header line is a Fund House name
            if (string.IsNullOrEmpty(line))
            {
                expectFundHouse = true;
                continue;
            }

            // Section header: "Open Ended Schemes(Debt Scheme - Banking and PSU Fund)"
            if (line.StartsWith("Open Ended Schemes", StringComparison.OrdinalIgnoreCase)
                || line.StartsWith("Close Ended Schemes", StringComparison.OrdinalIgnoreCase)
                || line.StartsWith("Interval Fund Schemes", StringComparison.OrdinalIgnoreCase))
            {
                var parenStart = line.IndexOf('(');
                var parenEnd = line.LastIndexOf(')');

                if (parenStart >= 0 && parenEnd > parenStart)
                {
                    currentSchemeType = line[..parenStart].Trim();
                    currentSchemeCategory = line[(parenStart + 1)..parenEnd].Trim();
                }
                else
                {
                    currentSchemeType = line;
                    currentSchemeCategory = null;
                }

                expectFundHouse = false;
                continue;
            }

            // Skip the column header line
            if (line.StartsWith("Scheme Code", StringComparison.OrdinalIgnoreCase))
                continue;

            // Data row — contains semicolons
            if (line.Contains(';'))
            {
                var parts = line.Split(';');
                if (parts.Length < 6) continue;

                if (!int.TryParse(parts[0].Trim(), out var schemeCode)) continue;

                var isinGrowth = NullIfDash(parts[1].Trim());
                var isinDivReinvestment = NullIfDash(parts[2].Trim());
                var schemeName = parts[3].Trim();
                var navStr = NullIfDash(parts[4].Trim());
                var dateStr = NullIfDash(parts[5].Trim());

                decimal? nav = null;
                if (navStr is not null && decimal.TryParse(navStr, CultureInfo.InvariantCulture, out var parsedNav))
                    nav = parsedNav;

                DateOnly? navDate = null;
                if (dateStr is not null && DateOnly.TryParseExact(dateStr, "dd-MMM-yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None, out var parsedDate))
                    navDate = parsedDate;

                funds.Add(new Fund
                {
                    SchemeCode = schemeCode,
                    IsinGrowth = isinGrowth,
                    IsinDivReinvestment = isinDivReinvestment,
                    SchemeName = schemeName,
                    LatestNav = nav,
                    LatestNavDate = navDate,
                    FundHouse = currentFundHouse,
                    SchemeType = currentSchemeType,
                    SchemeCategory = currentSchemeCategory
                });

                expectFundHouse = false;
                continue;
            }

            // Standalone text line after a blank line = Fund House name
            if (expectFundHouse)
            {
                currentFundHouse = line;
                expectFundHouse = false;
            }
        }

        logger.LogInformation("Parsed {Count} funds from AMFI master data", funds.Count);
        return funds;
    }

    private static string? NullIfDash(string value)
        => value is "-" or "" ? null : value;
}
