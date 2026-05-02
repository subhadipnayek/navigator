namespace NAVigator.Models.Dto;

public class AnomalyAutoRecordResultDto
{
    public int DetectedAnomalies { get; set; }
    public int EligibleAnomalies { get; set; }
    public int RecordedActions { get; set; }
    public List<CorporateActionDto> Actions { get; set; } = [];
}
