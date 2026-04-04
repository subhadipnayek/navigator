namespace NAVigator.Models.Dto;

public class FundSyncResultDto
{
    public int TotalParsed { get; set; }
    public int Added { get; set; }
    public int Updated { get; set; }
    public int Unchanged { get; set; }
    public TimeSpan Duration { get; set; }
}
