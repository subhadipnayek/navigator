using System.Globalization;
using System.Text.Json.Serialization;
using System.Text.Json;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public class AmfiService(IHttpClientFactory httpClientFactory, ILogger<AmfiService> logger) : IAmfiService
{
    private const string AmfiMasterUrl = "https://portal.amfiindia.com/DownloadSchemeData_Po.aspx?mf=0";
    private const string AmfiLatestNavUrl = "https://portal.amfiindia.com/spages/NAVAll.txt";
    private const string AmfiNavHistoryUrl = "https://www.amfiindia.com/api/nav-history";
    private const int NavHistoryChunkYears = 4;
    private const int NavHistoryParallelChunks = 2;
    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        NumberHandling = JsonNumberHandling.AllowReadingFromString
    };

    public async Task<List<Fund>> FetchFundMasterListAsync(CancellationToken ct = default)
    {
        var client = httpClientFactory.CreateClient("Amfi");
        var response = await client.GetStringAsync(AmfiMasterUrl, ct);
        return await ParseAmfiMasterData(response);
    }

    public async Task<List<AmfiLatestNavSnapshot>> FetchLatestNavSnapshotsAsync(CancellationToken ct = default)
    {
        var client = httpClientFactory.CreateClient("Amfi");
        var response = await client.GetStringAsync(AmfiLatestNavUrl, ct);
        return await ParseLatestNavAllAsync(new StringReader(response));
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

        var chunks = BuildHistoryChunks(from, to).ToList();
        for (var i = 0; i < chunks.Count; i += NavHistoryParallelChunks)
        {
            var batch = chunks.Skip(i).Take(NavHistoryParallelChunks).ToList();
            var results = await Task.WhenAll(batch.Select(chunk =>
                FetchNavHistoryChunkAsync(client, schemeCode, chunk.From, chunk.To, ct)));

            foreach (var result in results)
            {
                if (!result.Succeeded)
                    return null;

                if (result.Entries.Count == 0)
                {
                    logger.LogDebug(
                        "AMFI returned no NAV records for scheme {SchemeCode} from {From} to {To}: {Message}",
                        schemeCode,
                        result.From,
                        result.To,
                        result.Message ?? "empty response");

                    if (foundAnyRecords && stopAtFirstOlderEmptyChunk)
                        return ToMfApiResponse(entriesByDate);

                    continue;
                }

                foreach (var (date, entry) in result.Entries)
                    entriesByDate[date] = entry;

                foundAnyRecords = true;
            }
        }

        return ToMfApiResponse(entriesByDate);
    }

    private async Task<NavHistoryChunkResult> FetchNavHistoryChunkAsync(
        HttpClient client,
        int schemeCode,
        DateOnly from,
        DateOnly to,
        CancellationToken ct)
    {
        var url = BuildNavHistoryUrl(schemeCode, from, to);

        try
        {
            var response = await client.GetStringAsync(url, ct);
            var amfiResponse = JsonSerializer.Deserialize<AmfiNavHistoryResponse>(response, JsonOptions);
            var entries = new Dictionary<DateOnly, MfApiNavEntry>();

            if (amfiResponse?.Data?.NavGroups is null || amfiResponse.Data.NavGroups.Count == 0)
                return NavHistoryChunkResult.Empty(from, to, amfiResponse?.Message);

            foreach (var record in amfiResponse.Data.NavGroups.SelectMany(group => group.HistoricalRecords))
            {
                if (!DateOnly.TryParseExact(record.Date, "yyyy-MM-dd", CultureInfo.InvariantCulture,
                        DateTimeStyles.None, out var date))
                    continue;

                if (record.Nav <= 0)
                    continue;

                entries[date] = new MfApiNavEntry
                {
                    Date = date.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture),
                    Nav = record.Nav
                };
            }

            return NavHistoryChunkResult.Success(from, to, entries);
        }
        catch (HttpRequestException ex)
        {
            logger.LogWarning(
                ex,
                "Failed to fetch AMFI NAV history for scheme {SchemeCode} from {From} to {To}",
                schemeCode,
                from,
                to);
            return NavHistoryChunkResult.Failed(from, to);
        }
        catch (JsonException ex)
        {
            logger.LogWarning(
                ex,
                "Failed to parse AMFI NAV history for scheme {SchemeCode} from {From} to {To}",
                schemeCode,
                from,
                to);
            return NavHistoryChunkResult.Failed(from, to);
        }
    }

    private static MfApiResponse ToMfApiResponse(Dictionary<DateOnly, MfApiNavEntry> entriesByDate)
    {
        return new MfApiResponse
        {
            Data = entriesByDate
                .OrderBy(entry => entry.Key)
                .Select(entry => entry.Value)
                .ToList()
        };
    }

    private sealed record NavHistoryChunkResult(
        DateOnly From,
        DateOnly To,
        bool Succeeded,
        Dictionary<DateOnly, MfApiNavEntry> Entries,
        string? Message)
    {
        public static NavHistoryChunkResult Success(DateOnly from, DateOnly to, Dictionary<DateOnly, MfApiNavEntry> entries)
            => new(from, to, true, entries, null);

        public static NavHistoryChunkResult Empty(DateOnly from, DateOnly to, string? message)
            => new(from, to, true, [], message);

        public static NavHistoryChunkResult Failed(DateOnly from, DateOnly to)
            => new(from, to, false, [], null);
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

    private async Task<List<Fund>> ParseAmfiMasterData(string rawData)
    {
        try
        {
            if (LooksLikeSchemeWorkbookExport(rawData))
            {
                return ParseSchemeWorkbookExport(rawData);
            }
            else
            {
                return await ParseLegacyNavAllAsync(new StringReader(rawData));

            }

        }
        catch (Exception ex)
        {
            logger.LogWarning(
                ex,
                "Failed to parse AMFI master data. This may be due to an unexpected format change in the source data. " +
                "Please verify the format of the data at {Url} and update the parsing logic if necessary.",
                AmfiMasterUrl);

            return new List<Fund>();
        }
    }

    private async Task<List<Fund>> ParseLegacyNavAllAsync(TextReader reader)
    {
        var funds = new List<Fund>();
        var seenSchemeCodes = new HashSet<int>();

        string? currentSchemeType = null;
        string? currentSchemeCategory = null;
        string? currentFundHouse = null;
        bool expectFundHouse = false;

        string? rawLine;

        while ((rawLine = await reader.ReadLineAsync()) != null)
        {
            var line = rawLine.Trim();

            if (string.IsNullOrEmpty(line))
            {
                expectFundHouse = true;
                continue;
            }

            if (line.StartsWith("Open Ended Schemes", StringComparison.OrdinalIgnoreCase)
                || line.StartsWith("Close Ended Schemes", StringComparison.OrdinalIgnoreCase)
                || line.StartsWith("Interval Fund Schemes", StringComparison.OrdinalIgnoreCase))
            {
                var parenStart = line.IndexOf('(');
                var parenEnd = line.LastIndexOf(')');

                currentSchemeType = parenStart >= 0
                    ? line[..parenStart].Trim()
                    : line;

                currentSchemeCategory = (parenStart >= 0 && parenEnd > parenStart)
                    ? line[(parenStart + 1)..parenEnd].Trim()
                    : null;

                currentFundHouse = null;
                expectFundHouse = false;
                continue;
            }

            if (line.StartsWith("Scheme Code", StringComparison.OrdinalIgnoreCase))
                continue;

            if (line.Contains(';'))
            {
                // Fix #5: cap split to 7 — exposes scheme names with semicolons
                // via the malformed row check instead of silently shifting columns
                var parts = line.Split(';', 7);

                if (parts.Length < 6)
                {
                    logger.LogWarning(
                        "Skipping malformed data line (expected 6 columns, got {Count}): {Line}",
                        parts.Length, line);
                    continue;
                }

                if (!int.TryParse(parts[0].Trim(), out var schemeCode))
                {
                    logger.LogWarning("Skipping line with non-numeric scheme code: {Line}", line);
                    continue;
                }

                // Fix #2: duplicate scheme code guard
                if (!seenSchemeCodes.Add(schemeCode))
                {
                    logger.LogWarning("Duplicate scheme code {Code} encountered, skipping", schemeCode);
                    continue;
                }

                var isinGrowth = NullIfDash(parts[1].Trim());
                var isinDivReinvestment = NullIfDash(parts[2].Trim());
                var rawSchemeName = NullIfDash(parts[3].Trim());
                var navStr = NullIfDash(parts[4].Trim());
                var dateStr = NullIfDash(parts[5].Trim());

                // Fix #7: warn on empty/dash scheme name
                if (rawSchemeName is null)
                {
                    logger.LogWarning("Scheme code {Code} has empty scheme name", schemeCode);
                }
                var schemeName = rawSchemeName ?? string.Empty;

                // Fix #3: treat 0.0000 as null — no valid fund has a zero NAV
                decimal? nav = navStr is not null
                    && decimal.TryParse(navStr, CultureInfo.InvariantCulture, out var parsedNav)
                    && parsedNav > 0
                        ? parsedNav
                        : null;

                DateOnly? navDate = dateStr is not null
                    && DateOnly.TryParseExact(dateStr, "dd-MMM-yyyy", CultureInfo.InvariantCulture,
                        DateTimeStyles.None, out var parsedDate)
                        ? parsedDate
                        : null;

                funds.Add(new Fund
                {
                    SchemeCode = schemeCode,
                    IsinGrowth = isinGrowth,
                    IsinDivReinvestment = isinDivReinvestment,
                    SchemeName = schemeName,
                    FundHouse = currentFundHouse,
                    SchemeType = currentSchemeType,
                    SchemeCategory = currentSchemeCategory
                });

                expectFundHouse = false;
                continue;
            }

            // Fix #4: tighter fund house heuristic — must be inside a valid section
            // and the line must look like a fund house name (contains "Fund" or "Asset")
            if (expectFundHouse
                && currentSchemeType is not null
                && LooksLikeFundHouse(line))
            {
                currentFundHouse = line;
                expectFundHouse = false;
            }
        }

        logger.LogInformation("Parsed {Count} funds from AMFI master data", funds.Count);
        return funds;
    }

    // Fix #4: heuristic guard — real fund house names always contain one of these
    private static bool LooksLikeFundHouse(string line) =>
        line.Contains("Mutual Fund", StringComparison.OrdinalIgnoreCase)
        || line.Contains("Asset Management", StringComparison.OrdinalIgnoreCase)
        || line.Contains("AMC", StringComparison.OrdinalIgnoreCase);

    private List<Fund> ParseSchemeWorkbookExport(string rawData)
    {
        var funds = new List<Fund>();
        var lines = rawData.Split('\n', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries);

        foreach (var line in lines)
        {
            if (line.StartsWith("AMC|", StringComparison.OrdinalIgnoreCase)
                || line.StartsWith("AMC,", StringComparison.OrdinalIgnoreCase)
                || line.StartsWith("AMC\t", StringComparison.OrdinalIgnoreCase))
                continue;

            var parts = ParseDelimitedLine(line);
            if (parts.Length < 10)
                continue;

            if (!int.TryParse(parts[1].Trim(), out var schemeCode))
                continue;

            var launchDate = TryParseAmfiDate(parts[7].Trim());
            // AMFI "Closure Date" here is NFO subscription close date;
            // it does not mean an open-ended scheme stopped accepting investments or NAV updates.
            var (isinGrowth, isinDivReinvestment) = ParseCombinedIsin(parts[9].Trim());
            var isOpenEnded = parts[3].Trim().Equals("Open Ended", StringComparison.OrdinalIgnoreCase);

            var isEtf = parts[5].Trim().IndexOf("etf", StringComparison.OrdinalIgnoreCase) >= 0;

            var isDirectGrowth = MatchesFilter(
                parts[5].Trim(),
                mustContain: new[] { "direct", "growth" },
                mustNotContain: new[] { "idcw", "dividend", "regular" },
                matchAll: true
            );

            if (!isOpenEnded) continue;

            // ETFs skip the direct/growth filter — they're always growth by nature
            if (!isEtf && !isDirectGrowth) continue;
            funds.Add(new Fund
            {
                SchemeCode = schemeCode,
                SchemeName = parts[5].Trim(),
                FundHouse = NullIfDash(parts[0].Trim()),
                SchemeType = NullIfDash(parts[3].Trim()),
                SchemeCategory = NullIfDash(parts[4].Trim()),
                IsinGrowth = isinGrowth,
                IsinDivReinvestment = isinDivReinvestment,
                LaunchDate = launchDate
            });
        }

        logger.LogInformation("Parsed {Count} funds from AMFI workbook export", funds.Count);
        return funds;
    }
    bool MatchesFilter(string line, string[] mustContain, string[] mustNotContain, bool matchAll = false)
    {
        var normalized = line.Replace("-", " ");

        bool hasMatch = matchAll
            ? mustContain.All(word => normalized.IndexOf(word, StringComparison.OrdinalIgnoreCase) >= 0)  // AND
            : mustContain.Any(word => normalized.IndexOf(word, StringComparison.OrdinalIgnoreCase) >= 0); // OR

        bool hasNone = mustNotContain.All(word => normalized.IndexOf(word, StringComparison.OrdinalIgnoreCase) < 0);

        return hasMatch && hasNone;
    }
    private async Task<List<AmfiLatestNavSnapshot>> ParseLatestNavAllAsync(TextReader reader)
    {
        var snapshots = new List<AmfiLatestNavSnapshot>();
        var seenSchemeCodes = new HashSet<int>();
        string? rawLine;

        while ((rawLine = await reader.ReadLineAsync()) != null)
        {
            var line = rawLine.Trim();
            if (string.IsNullOrEmpty(line) || !line.Contains(';'))
                continue;

            if (line.StartsWith("Scheme Code", StringComparison.OrdinalIgnoreCase))
                continue;

            var parts = line.Split(';', 7);
            if (parts.Length < 6)
                continue;

            if (!int.TryParse(parts[0].Trim(), out var schemeCode))
                continue;

            if (!seenSchemeCodes.Add(schemeCode))
                continue;

            var navStr = NullIfDash(parts[4].Trim());
            var dateStr = NullIfDash(parts[5].Trim());
            if (navStr is null || dateStr is null)
                continue;

            if (!decimal.TryParse(navStr, CultureInfo.InvariantCulture, out var nav) || nav <= 0)
                continue;

            if (!DateOnly.TryParseExact(dateStr, "dd-MMM-yyyy", CultureInfo.InvariantCulture,
                    DateTimeStyles.None, out var navDate))
                continue;

            snapshots.Add(new AmfiLatestNavSnapshot
            {
                SchemeCode = schemeCode,
                LatestNav = nav,
                LatestNavDate = navDate
            });
        }

        logger.LogInformation("Parsed {Count} latest NAV snapshots from AMFI NAVAll", snapshots.Count);
        return snapshots;
    }
    private static bool LooksLikeSchemeWorkbookExport(string rawData)
    {
        var firstNonEmpty = rawData
            .Split('\n', StringSplitOptions.RemoveEmptyEntries | StringSplitOptions.TrimEntries)
            .FirstOrDefault();

        return firstNonEmpty is not null
               && (firstNonEmpty.StartsWith("AMC|", StringComparison.OrdinalIgnoreCase)
                   || firstNonEmpty.StartsWith("AMC,", StringComparison.OrdinalIgnoreCase)
                   || firstNonEmpty.StartsWith("AMC\t", StringComparison.OrdinalIgnoreCase))
               && firstNonEmpty.Contains("Launch Date", StringComparison.OrdinalIgnoreCase);
    }
    private static string[] ParseDelimitedLine(string line)
    {
        var delimiter = line.Contains('|')
            ? '|'
            : line.Contains('\t')
                ? '\t'
                : ',';
        var values = new List<string>();
        var current = new System.Text.StringBuilder();
        var inQuotes = false;

        foreach (var ch in line)
        {
            if (ch == '"')
            {
                inQuotes = !inQuotes;
                continue;
            }

            if (ch == delimiter && !inQuotes)
            {
                values.Add(current.ToString().Trim());
                current.Clear();
                continue;
            }

            current.Append(ch);
        }

        values.Add(current.ToString().Trim());
        return values.ToArray();
    }
    private static DateOnly? TryParseAmfiDate(string value)
         => DateOnly.TryParseExact(value, "dd-MMM-yyyy", CultureInfo.InvariantCulture, DateTimeStyles.None,
             out var parsed)
             ? parsed
             : null;

    private static (string? IsinGrowth, string? IsinDivReinvestment) ParseCombinedIsin(string value)
    {
        var normalized = NullIfDash(value);
        if (normalized is null)
            return (null, null);

        if (normalized.Length <= 12)
            return (normalized, null);

        var growth = normalized[..12];
        var reinvestment = normalized.Length > 12 ? normalized[12..] : null;
        return (NullIfDash(growth), reinvestment is null ? null : NullIfDash(reinvestment));
    }
    private static string? NullIfDash(string value)
        => value is "-" or "" ? null : value;
}
