namespace NAVigator.Models.Dto;

public class NavGapDto
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public string? FundHouse { get; set; }
    public required string PreviousDate { get; set; }
    public required string NextDate { get; set; }
    public required string MissingFrom { get; set; }
    public required string MissingTo { get; set; }
    public int GapDays { get; set; }
    public int MissingCalendarDays { get; set; }
}

public class NavGapRepairResultDto
{
    public int SchemeCode { get; set; }
    public required string SchemeName { get; set; }
    public int DetectedGaps { get; set; }
    public int RepairedGaps { get; set; }
    public int VerifiedEmptyGaps { get; set; }
    public int InsertedRecords { get; set; }
    public List<NavGapRepairItemDto> Gaps { get; set; } = [];
}

public class NavGapRepairItemDto : NavGapDto
{
    public int InsertedRecords { get; set; }
    public required string Status { get; set; }
}
