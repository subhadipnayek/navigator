namespace NAVigator.Models.Dto;

/// <summary>
/// Raw row returned by the SQL LAG() window function query for gap detection.
/// Used internally by DetectAllNavGapsAsync — not exposed through API.
/// </summary>
public class NavGapRawRow
{
    public int SchemeCode { get; set; }
    public DateOnly CurrentDate { get; set; }
    public decimal Nav { get; set; }
    public DateOnly? PreviousDate { get; set; }
}
