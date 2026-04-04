using System.Globalization;
using System.Text.Json;
using NAVigator.Models.Database;
using NAVigator.Models.Dto;

namespace NAVigator.Services;

public class AmfiService(IHttpClientFactory httpClientFactory, ILogger<AmfiService> logger) : IAmfiService
{
    private const string AmfiMasterUrl = "https://portal.amfiindia.com/spages/NAVAll.txt";
    private const string MfApiBaseUrl = "https://api.mfapi.in/mf/";

    public async Task<List<Fund>> FetchFundMasterListAsync(CancellationToken ct = default)
    {
        var client = httpClientFactory.CreateClient("Amfi");
        var response = await client.GetStringAsync(AmfiMasterUrl, ct);
        return ParseAmfiMasterData(response);
    }

    public async Task<MfApiResponse?> FetchNavHistoryAsync(int schemeCode, CancellationToken ct = default)
    {
        var client = httpClientFactory.CreateClient("MfApi");
        var url = $"{MfApiBaseUrl}{schemeCode}";

        try
        {
            var response = await client.GetStringAsync(url, ct);
            return JsonSerializer.Deserialize<MfApiResponse>(response);
        }
        catch (HttpRequestException ex)
        {
            logger.LogWarning(ex, "Failed to fetch NAV history for scheme {SchemeCode}", schemeCode);
            return null;
        }
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
