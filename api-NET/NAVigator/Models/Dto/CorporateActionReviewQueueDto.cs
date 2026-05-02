namespace NAVigator.Models.Dto;

public class CorporateActionReviewQueueDto
{
    public List<CorporateActionReviewRowDto> Rows { get; set; } = [];
    public CorporateActionReviewSummaryDto Summary { get; set; } = new();
    public List<NavAnomalyDto> Anomalies { get; set; } = [];
    public List<NavGapDto> Gaps { get; set; } = [];
}

public class CorporateActionReviewSummaryDto
{
    public int PendingVisible { get; set; }
    public int NavGaps { get; set; }
    public int AutoRecordable { get; set; }
    public int HighPriority { get; set; }
    public int PreReview { get; set; }
    public int RecordedItems { get; set; }
}

public class CorporateActionReviewRowDto
{
    public required string Kind { get; set; }
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public required string Date { get; set; }
    public required string PreviousDate { get; set; }
    public int DaysSincePrevious { get; set; }
    public required string Title { get; set; }
    public required string Detail { get; set; }
    public required string NavDetail { get; set; }
    public required string SuggestionTitle { get; set; }
    public required string SuggestionDetail { get; set; }
    public required string Priority { get; set; }
    public required string Confidence { get; set; }
    public bool HasCorporateAction { get; set; }
    public bool IsAutoRecordable { get; set; }
    public bool RequiresPreReview { get; set; }
    public decimal SortMagnitude { get; set; }
    public bool HasMovement { get; set; }
    public bool HasGap { get; set; }
    public bool CanRepairGap { get; set; }
    public bool CanSyncNav { get; set; }
    public NavAnomalyDto? Anomaly { get; set; }
    public NavGapDto? Gap { get; set; }
}
