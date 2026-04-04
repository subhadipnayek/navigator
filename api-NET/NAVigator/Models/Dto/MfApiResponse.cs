using System.Text.Json.Serialization;

namespace NAVigator.Models.Dto;

/// <summary>
/// Maps the JSON response from https://api.mfapi.in/mf/{scheme_code}
/// </summary>
public class MfApiResponse
{
    [JsonPropertyName("meta")]
    public MfApiMeta Meta { get; set; } = null!;

    [JsonPropertyName("data")]
    public List<MfApiNavEntry> Data { get; set; } = [];
}

public class MfApiMeta
{
    [JsonPropertyName("fund_house")]
    public string? FundHouse { get; set; }

    [JsonPropertyName("scheme_type")]
    public string? SchemeType { get; set; }

    [JsonPropertyName("scheme_category")]
    public string? SchemeCategory { get; set; }

    [JsonPropertyName("scheme_code")]
    public int SchemeCode { get; set; }

    [JsonPropertyName("scheme_name")]
    public string? SchemeName { get; set; }

    [JsonPropertyName("isin_growth")]
    public string? IsinGrowth { get; set; }

    [JsonPropertyName("isin_div_reinvestment")]
    public string? IsinDivReinvestment { get; set; }
}

public class MfApiNavEntry
{
    [JsonPropertyName("date")]
    public string Date { get; set; } = string.Empty;

    [JsonPropertyName("nav")]
    public string Nav { get; set; } = string.Empty;
}
