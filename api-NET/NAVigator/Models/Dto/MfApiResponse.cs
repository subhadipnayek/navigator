using System.Text.Json.Serialization;

namespace NAVigator.Models.Dto;

/// <summary>
/// Normalized NAV history response used by the AMFI history sync flow.
/// </summary>
public class MfApiResponse
{
    [JsonPropertyName("data")]
    public List<MfApiNavEntry> Data { get; set; } = [];
}

public class MfApiNavEntry
{
    [JsonPropertyName("date")]
    public string Date { get; set; } = string.Empty;

    [JsonPropertyName("nav")]
    public decimal Nav { get; set; }
}

public class AmfiNavHistoryResponse
{
    [JsonPropertyName("message")]
    public string? Message { get; set; }

    [JsonPropertyName("data")]
    public AmfiNavHistoryData? Data { get; set; }
}

public class AmfiNavHistoryData
{
    [JsonPropertyName("mf_name")]
    public string? MfName { get; set; }

    [JsonPropertyName("scheme_name")]
    public string? SchemeName { get; set; }

    [JsonPropertyName("date_range")]
    public string? DateRange { get; set; }

    [JsonPropertyName("nav_groups")]
    public List<AmfiNavGroup> NavGroups { get; set; } = [];
}

public class AmfiNavGroup
{
    [JsonPropertyName("nav_name")]
    public string? NavName { get; set; }

    [JsonPropertyName("historical_records")]
    public List<AmfiHistoricalRecord> HistoricalRecords { get; set; } = [];
}

public class AmfiHistoricalRecord
{
    [JsonPropertyName("date")]
    public string Date { get; set; } = string.Empty;

    [JsonPropertyName("nav")]
    public decimal Nav { get; set; }

    [JsonPropertyName("upload_time")]
    public DateTime? UploadTime { get; set; }

    [JsonPropertyName("upload_time_display")]
    public string? UploadTimeDisplay { get; set; }

    [JsonPropertyName("repurchase")]
    public string? Repurchase { get; set; }

    [JsonPropertyName("reissue")]
    public string? Reissue { get; set; }
}
