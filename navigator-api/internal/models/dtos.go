package models

import "time"

// ─── Auth DTOs ───

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	UsernameOrEmail string `json:"usernameOrEmail" binding:"required"`
	Password        string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token        string      `json:"token"`
	ExpiresAtUtc time.Time   `json:"expiresAtUtc"`
	User         AuthUserDto `json:"user"`
}

type AuthUserDto struct {
	Id       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ─── Fund DTOs ───

type FundDto struct {
	SchemeCode          int      `json:"schemeCode"`
	SchemeName          string   `json:"schemeName"`
	FundHouse           *string  `json:"fundHouse"`
	SchemeType          *string  `json:"schemeType"`
	SchemeCategory      *string  `json:"schemeCategory"`
	IsinGrowth          *string  `json:"isinGrowth"`
	IsinDivReinvestment *string  `json:"isinDivReinvestment"`
	LatestNav           *float64 `json:"latestNav"`
	LatestNavDate       *string  `json:"latestNavDate"`
	NavStartDate        *string  `json:"navStartDate"`
	NavEndDate          *string  `json:"navEndDate"`
	NavHistoryCount     int      `json:"navHistoryCount"`
}

type FundSyncResultDto struct {
	TotalParsed int    `json:"totalParsed"`
	Added       int    `json:"added"`
	Inserted    int    `json:"inserted"`
	Updated     int    `json:"updated"`
	Unchanged   int    `json:"unchanged"`
	Total       int    `json:"total"`
	Duration    string `json:"duration"`
}

// ─── NAV DTOs ───

type NavDataPointDto struct {
	Date time.Time `json:"date"`
	Nav  float64   `json:"nav"`
}

type NavDataResponse struct {
	SchemeCode int               `json:"schemeCode"`
	SchemeName string            `json:"schemeName"`
	FundHouse  *string           `json:"fundHouse"`
	Category   *string           `json:"category"`
	Data       []NavDataPointDto `json:"data"`
}

type ReturnsResponse struct {
	SchemeCode            int     `json:"schemeCode"`
	SchemeName            string  `json:"schemeName"`
	StartDate             string  `json:"startDate"`
	EndDate               string  `json:"endDate"`
	StartNav              float64 `json:"startNav"`
	EndNav                float64 `json:"endNav"`
	AbsoluteReturnPercent float64 `json:"absoluteReturnPercent"`
	Cagr                  float64 `json:"cagr"`
	Days                  int     `json:"days"`
}

type RollingReturnDto struct {
	Date             string  `json:"date"`
	ReturnPercentage float64 `json:"returnPercentage"`
}

type RollingReturnResponse struct {
	SchemeCode    int                `json:"schemeCode"`
	PeriodInYears int                `json:"periodInYears"`
	DataPoints    int                `json:"dataPoints"`
	MinReturn     *float64           `json:"minReturn"`
	MaxReturn     *float64           `json:"maxReturn"`
	AverageReturn *float64           `json:"averageReturn"`
	MedianReturn  *float64           `json:"medianReturn"`
	Data          []RollingReturnDto `json:"data"`
}

type NavGapDto struct {
	SchemeCode int    `json:"schemeCode"`
	SchemeName string `json:"schemeName"`
	FromDate   string `json:"fromDate"`
	ToDate     string `json:"toDate"`
	GapDays    int    `json:"gapDays"`
}

// ─── Corporate Actions DTOs ───

type CorporateActionDto struct {
	Id               int64    `json:"id"`
	SchemeCode       int      `json:"schemeCode"`
	ActionDate       string   `json:"actionDate"` // dd-MM-yyyy
	ActionType       string   `json:"actionType"`
	AdjustmentFactor float64  `json:"adjustmentFactor"`
	OldUnits         *float64 `json:"oldUnits"`
	NewUnits         *float64 `json:"newUnits"`
	Ratio            *string  `json:"ratio"`
	ConfidenceScore  *float64 `json:"confidenceScore"`
	Notes            *string  `json:"notes"`
	IsAutoDetected   bool     `json:"isAutoDetected"`
	IsVerified       bool     `json:"isVerified"`
}

type CreateCorporateActionRequest struct {
	SchemeCode       int      `json:"schemeCode" binding:"required"`
	ActionDate       string   `json:"actionDate" binding:"required"` // dd-MM-yyyy
	ActionType       string   `json:"actionType" binding:"required"`
	AdjustmentFactor float64  `json:"adjustmentFactor" binding:"required"`
	OldUnits         *float64 `json:"oldUnits"`
	NewUnits         *float64 `json:"newUnits"`
	Ratio            *string  `json:"ratio"`
	Notes            *string  `json:"notes"`
	IsVerified       bool     `json:"isVerified"`
}

type UpdateCorporateActionRequest struct {
	ActionDate       string   `json:"actionDate" binding:"required"`
	ActionType       string   `json:"actionType" binding:"required"`
	AdjustmentFactor float64  `json:"adjustmentFactor" binding:"required"`
	OldUnits         *float64 `json:"oldUnits"`
	NewUnits         *float64 `json:"newUnits"`
	Ratio            *string  `json:"ratio"`
	Notes            *string  `json:"notes"`
	IsVerified       bool     `json:"isVerified"`
}

type NavAnomalyDto struct {
	SchemeCode                 int      `json:"schemeCode"`
	SchemeName                 string   `json:"schemeName"`
	PreviousDate               string   `json:"previousDate"`
	PreviousNav                float64  `json:"previousNav"`
	AnomalyDate                string   `json:"anomalyDate"`
	AnomalyNav                 float64  `json:"anomalyNav"`
	PercentChange              float64  `json:"percentChange"`
	DaysSincePrevious          int      `json:"daysSincePrevious"`
	SuggestedAction            string   `json:"suggestedAction"`
	SuggestedFactor            float64  `json:"suggestedFactor"`
	Confidence                 float64  `json:"confidence"`
	Explanation                string   `json:"explanation"`
	HasExistingCorporateAction bool     `json:"hasExistingCorporateAction"`
	ExistingActionId           *int64   `json:"existingActionId"`
	ExistingActionType         *string  `json:"existingActionType"`
	ExistingAdjustmentFactor   *float64 `json:"existingAdjustmentFactor"`
}

type CorporateActionReviewQueueDto struct {
	MovementThreshold     float64         `json:"movementThreshold"`
	MinGapDays            int             `json:"minGapDays"`
	Filter                string          `json:"filter"`
	Priority              string          `json:"priority"`
	TotalItems            int             `json:"totalItems"`
	PendingReviewCount    int             `json:"pendingReviewCount"`
	RecordedCount         int             `json:"recordedCount"`
	EligibleForAutoRecord int             `json:"eligibleForAutoRecord"`
	Items                 []NavAnomalyDto `json:"items"`
}

type AnomalyAutoRecordResultDto struct {
	TotalProcessed int `json:"totalProcessed"`
	CreatedCount   int `json:"createdCount"`
	SkippedCount   int `json:"skippedCount"`
}

// ─── Portfolio Simulation DTOs ───

type FundAllocation struct {
	SchemeCode        int     `json:"schemeCode"`
	AllocationPercent float64 `json:"allocationPercent"`
}

type BasketDefinition struct {
	Name               string           `json:"name"`
	RebalanceFrequency string           `json:"rebalanceFrequency"` // None, Monthly, Quarterly, Yearly
	Funds              []FundAllocation `json:"funds"`
}

type SimulationRequest struct {
	Basket        BasketDefinition `json:"basket"`
	StartDate     string           `json:"startDate"` // dd-MM-yyyy
	EndDate       string           `json:"endDate"`   // dd-MM-yyyy
	SipAmount     float64          `json:"sipAmount"`
	SipDay        int              `json:"sipDay"`
	LumpSumAmount float64          `json:"lumpSumAmount"`
}

type FundSnapshot struct {
	SchemeCode int     `json:"schemeCode"`
	Value      float64 `json:"value"`
	Units      float64 `json:"units"`
}

type PortfolioSnapshot struct {
	Date           string         `json:"date"`
	PortfolioValue float64        `json:"portfolioValue"`
	TotalInvested  float64        `json:"totalInvested"`
	GainPercent    float64        `json:"gainPercent"`
	Funds          []FundSnapshot `json:"funds"`
}

type FundSimulationDetail struct {
	SchemeCode            int     `json:"schemeCode"`
	SchemeName            string  `json:"schemeName"`
	AllocationPercent     float64 `json:"allocationPercent"`
	TotalUnits            float64 `json:"totalUnits"`
	TotalInvested         float64 `json:"totalInvested"`
	FinalValue            float64 `json:"finalValue"`
	AbsoluteReturnPercent float64 `json:"absoluteReturnPercent"`
}

type SimulationResponse struct {
	BasketName            string                 `json:"basketName"`
	TotalInvested         float64                `json:"totalInvested"`
	FinalCorpusValue      float64                `json:"finalCorpusValue"`
	AbsoluteReturnPercent float64                `json:"absoluteReturnPercent"`
	Cagr                  *float64               `json:"cagr"`
	Xirr                  *float64               `json:"xirr"`
	MaxDrawdownPercent    *float64               `json:"maxDrawdownPercent"`
	Volatility            *float64               `json:"volatility"`
	StartDate             string                 `json:"startDate"`
	EndDate               string                 `json:"endDate"`
	EffectiveStartDate    string                 `json:"effectiveStartDate"`
	EffectiveEndDate      string                 `json:"effectiveEndDate"`
	WasDateRangeClamped   bool                   `json:"wasDateRangeClamped"`
	DateRangeNote         *string                `json:"dateRangeNote"`
	SipInstallments       int                    `json:"sipInstallments"`
	SipAmount             float64                `json:"sipAmount"`
	LumpSumAmount         float64                `json:"lumpSumAmount"`
	FundDetails           []FundSimulationDetail `json:"fundDetails"`
	TimeSeries            []PortfolioSnapshot    `json:"timeSeries"`
}

type ComparisonEntry struct {
	Name                 string            `json:"name"`
	Basket               *BasketDefinition `json:"basket,omitempty"`
	SingleFundSchemeCode *int              `json:"singleFundSchemeCode,omitempty"`
	SipAmount            float64           `json:"sipAmount"`
	SipDay               int               `json:"sipDay"`
	LumpSumAmount        float64           `json:"lumpSumAmount"`
}

type ComparisonRequest struct {
	Entries              []ComparisonEntry `json:"entries"`
	StartDate            string            `json:"startDate"`
	EndDate              string            `json:"endDate"`
	NormalizeSipAmounts  bool              `json:"normalizeSipAmounts"`
}

type ComparisonEntryResult struct {
	Name                  string   `json:"name"`
	TotalInvested         float64  `json:"totalInvested"`
	FinalCorpusValue      float64  `json:"finalCorpusValue"`
	AbsoluteReturnPercent float64  `json:"absoluteReturnPercent"`
	Cagr                  *float64 `json:"cagr"`
	Xirr                  *float64 `json:"xirr"`
	MaxDrawdownPercent    *float64 `json:"maxDrawdownPercent"`
	Volatility            *float64 `json:"volatility"`
	AlphaVsFirst          *float64 `json:"alphaVsFirst"`
}

type AlignedSeriesValue struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type AlignedPortfolioSnapshot struct {
	Date   string               `json:"date"`
	Values []AlignedSeriesValue `json:"values"`
}

type ComparisonResponse struct {
	StartDate           string                     `json:"startDate"`
	EndDate             string                     `json:"endDate"`
	EffectiveStartDate  string                     `json:"effectiveStartDate"`
	EffectiveEndDate    string                     `json:"effectiveEndDate"`
	WasDateRangeClamped bool                       `json:"wasDateRangeClamped"`
	DateRangeNote       *string                    `json:"dateRangeNote"`
	IsNormalized        bool                       `json:"isNormalized"`
	Results             []ComparisonEntryResult    `json:"results"`
	TimeSeries          []AlignedPortfolioSnapshot `json:"timeSeries"`
}

type WhatIfScenario struct {
	Name          string           `json:"name"`
	Basket        BasketDefinition `json:"basket"`
	SipAmount     float64          `json:"sipAmount"`
	SipDay        int              `json:"sipDay"`
	LumpSumAmount float64          `json:"lumpSumAmount"`
}

type WhatIfRequest struct {
	Scenarios []WhatIfScenario `json:"scenarios"`
	StartDate string           `json:"startDate"`
	EndDate   string           `json:"endDate"`
}

type WhatIfScenarioResult struct {
	Name                   string   `json:"name"`
	SipAmount              float64  `json:"sipAmount"`
	LumpSumAmount          float64  `json:"lumpSumAmount"`
	TotalInvested          float64  `json:"totalInvested"`
	FinalCorpusValue       float64  `json:"finalCorpusValue"`
	AbsoluteReturnPercent  float64  `json:"absoluteReturnPercent"`
	Cagr                   *float64 `json:"cagr"`
	Xirr                   *float64 `json:"xirr"`
	MaxDrawdownPercent     *float64 `json:"maxDrawdownPercent"`
	Volatility             *float64 `json:"volatility"`
	CorpusPerRupeeInvested float64  `json:"corpusPerRupeeInvested"`
}

type WhatIfResponse struct {
	StartDate           string                 `json:"startDate"`
	EndDate             string                 `json:"endDate"`
	EffectiveStartDate  string                 `json:"effectiveStartDate"`
	EffectiveEndDate    string                 `json:"effectiveEndDate"`
	WasDateRangeClamped bool                   `json:"wasDateRangeClamped"`
	DateRangeNote       *string                `json:"dateRangeNote"`
	Scenarios           []WhatIfScenarioResult `json:"scenarios"`
	Insights            []string               `json:"insights"`
}

type RollingComparisonRequest struct {
	Entries       []ComparisonEntry `json:"entries"`
	StartDate     string            `json:"startDate"`
	EndDate       string            `json:"endDate"`
	PeriodInYears int               `json:"periodInYears"`
	Interval      *string           `json:"interval"`
}

type RollingComparisonEntryResult struct {
	Name          string   `json:"name"`
	DataPoints    int      `json:"dataPoints"`
	MinReturn     *float64 `json:"minReturn"`
	MaxReturn     *float64 `json:"maxReturn"`
	AverageReturn *float64 `json:"averageReturn"`
	MedianReturn  *float64 `json:"medianReturn"`
}

type RollingComparisonSeriesValue struct {
	Name          string   `json:"name"`
	ReturnPercent *float64 `json:"returnPercent"`
}

type RollingComparisonTimePoint struct {
	Date   string                         `json:"date"`
	Values []RollingComparisonSeriesValue `json:"values"`
}

type RollingComparisonResponse struct {
	StartDate     string                         `json:"startDate"`
	EndDate       string                         `json:"endDate"`
	PeriodInYears int                            `json:"periodInYears"`
	Results       []RollingComparisonEntryResult `json:"results"`
	TimeSeries    []RollingComparisonTimePoint   `json:"timeSeries"`
}

// ─── User Portfolio DTOs ───

type UserPortfolioSummaryDto struct {
	Id           int64     `json:"id"`
	Name         string    `json:"name"`
	FundCount    int       `json:"fundCount"`
	UpdatedAtUtc time.Time `json:"updatedAtUtc"`
}

type CreateUserPortfolioRequest struct {
	Name string `json:"name"`
}

type PortfolioMetricsDto struct {
	InvestedAmount        float64  `json:"investedAmount"`
	CurrentValue          float64  `json:"currentValue"`
	TotalGain             float64  `json:"totalGain"`
	GainPercentage        float64  `json:"gainPercentage"`
	RealizedGain          float64  `json:"realizedGain"`
	UnrealizedGain        float64  `json:"unrealizedGain"`
	Xirr                  *float64 `json:"xirr"`
	ActiveFundsCount      int      `json:"activeFundsCount"`
	FullyExitedFundsCount int      `json:"fullyExitedFundsCount"`
}

type FundInsightDto struct {
	SchemeCode     int      `json:"schemeCode"`
	SchemeName     string   `json:"schemeName"`
	FundHouse      string   `json:"fundHouse"`
	AssetClass     string   `json:"assetClass"`
	Category       string   `json:"category"`
	UnitsHeld      float64  `json:"unitsHeld"`
	InvestedAmount float64  `json:"investedAmount"`
	CurrentValue   float64  `json:"currentValue"`
	TotalGain      float64  `json:"totalGain"`
	GainPercentage float64  `json:"gainPercentage"`
	RealizedGain   float64  `json:"realizedGain"`
	UnrealizedGain float64  `json:"unrealizedGain"`
	Xirr           *float64 `json:"xirr"`
	AverageNav     float64  `json:"averageNav"`
	LatestNav      float64  `json:"latestNav"`
	LatestNavDate  string   `json:"latestNavDate"`
	IsFullyExited  bool     `json:"isFullyExited"`
}

type AllocationSegmentDto struct {
	Label      string  `json:"label"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type PeriodContributionDto struct {
	PeriodLabel string  `json:"periodLabel"`
	TotalInflow float64 `json:"totalInflow"`
	SipInflow   float64 `json:"sipInflow"`
	LumpInflow  float64 `json:"lumpInflow"`
	Outflow     float64 `json:"outflow"`
	NetInflow   float64 `json:"netInflow"`
}

type SipConsistencyDto struct {
	ExpectedMonthlySips int      `json:"expectedMonthlySips"`
	CompletedSips       int      `json:"completedSips"`
	MissedSips          int      `json:"missedSips"`
	ConsistencyScore    float64  `json:"consistencyScore"`
	MonthlyBreakdown    []string `json:"monthlyBreakdown"`
}

type PortfolioDashboardDto struct {
	Id                      int64                   `json:"id"`
	Name                    string                  `json:"name"`
	Metrics                 PortfolioMetricsDto     `json:"metrics"`
	Funds                   []FundInsightDto        `json:"funds"`
	AssetAllocation         []AllocationSegmentDto  `json:"assetAllocation"`
	FundAllocation          []AllocationSegmentDto  `json:"fundAllocation"`
	FinancialYearInflows    []PeriodContributionDto `json:"financialYearInflows"`
	CalendarYearInflows     []PeriodContributionDto `json:"calendarYearInflows"`
	SipConsistency          SipConsistencyDto       `json:"sipConsistency"`
	EmptyStates             []string                `json:"emptyStates"`
}

type PortfolioTransactionDto struct {
	Id              int64   `json:"id"`
	SchemeCode      int     `json:"schemeCode"`
	SchemeName      string  `json:"schemeName"`
	TransactionDate string  `json:"transactionDate"` // dd-MM-yyyy
	TransactionType string  `json:"transactionType"`
	Units           float64 `json:"units"`
	Nav             float64 `json:"nav"`
	Amount          float64 `json:"amount"`
	Notes           *string `json:"notes"`
	Isin            *string `json:"isin"`
}

type PortfolioTransactionsSummaryDto struct {
	TotalPurchases    float64 `json:"totalPurchases"`
	TotalRedemptions  float64 `json:"totalRedemptions"`
	NetInvestedAmount float64 `json:"netInvestedAmount"`
	TransactionCount  int     `json:"transactionCount"`
}

type SavePortfolioTransactionsRequest struct {
	FinancialYear string                    `json:"financialYear"`
	Transactions  []PortfolioTransactionDto `json:"transactions"`
}

type TransactionValidationIssueDto struct {
	RowNumber int    `json:"rowNumber"`
	Field     string `json:"field"`
	Message   string `json:"message"`
}

type SavePortfolioTransactionsResponse struct {
	IsValid      bool                            `json:"isValid"`
	Summary      PortfolioTransactionsSummaryDto `json:"summary"`
	Transactions []PortfolioTransactionDto       `json:"transactions"`
	Errors       []TransactionValidationIssueDto `json:"errors"`
}

type ImportDeletePreviewResponse struct {
	MatchingTransactionsToDelete int `json:"matchingTransactionsToDelete"`
}

type PortfolioBenchmarkRequest struct {
	BenchmarkSchemeCodes []int   `json:"benchmarkSchemeCodes"`
	StartDate            *string `json:"startDate"`
	EndDate              *string `json:"endDate"`
	RollingPeriodYears   *int    `json:"rollingPeriodYears"`
}

type PortfolioBenchmarkComparisonDto struct {
	PortfolioName       string                     `json:"portfolioName"`
	PortfolioXirr       *float64                   `json:"portfolioXirr"`
	PortfolioFinalValue float64                    `json:"portfolioFinalValue"`
	TotalInvested       float64                    `json:"totalInvested"`
	EffectiveStartDate  string                     `json:"effectiveStartDate"`
	EffectiveEndDate    string                     `json:"effectiveEndDate"`
	Benchmarks          []BenchmarkPerformanceDto  `json:"benchmarks"`
	TimeSeries          []AlignedPortfolioSnapshot `json:"timeSeries"`
}

type BenchmarkPerformanceDto struct {
	SchemeCode    int      `json:"schemeCode"`
	SchemeName    string   `json:"schemeName"`
	BenchmarkXirr *float64 `json:"benchmarkXirr"`
	FinalValue    float64  `json:"finalValue"`
	AlphaVsXirr   *float64 `json:"alphaVsXirr"`
}

type UserGridPreferenceDto struct {
	GridKey   string  `json:"gridKey"`
	StateJson *string `json:"stateJson"`
}

type SaveUserGridPreferenceRequest struct {
	StateJson string `json:"stateJson" binding:"required"`
}

type MatchPortfolioFundRow struct {
	RowNumber  int     `json:"rowNumber"`
	SchemeCode *string `json:"schemeCode"`
	Isin       *string `json:"isin"`
	SchemeName *string `json:"schemeName"`
}

type MatchPortfolioFundRequest struct {
	Rows []MatchPortfolioFundRow `json:"rows"`
}

type MatchedPortfolioFundDto struct {
	RowNumber  int     `json:"rowNumber"`
	SchemeCode *int    `json:"schemeCode"`
	SchemeName *string `json:"schemeName"`
	Error      *string `json:"error"`
}

type MatchPortfolioFundResponse struct {
	Matches []MatchedPortfolioFundDto `json:"matches"`
}

// ─── Admin DTOs ───

type AdminDatabaseSummaryDto struct {
	TotalFunds              int     `json:"totalFunds"`
	FundsWithNavHistory     int     `json:"fundsWithNavHistory"`
	FundsWithoutNavHistory  int     `json:"fundsWithoutNavHistory"`
	NavHistoryRecords       int64   `json:"navHistoryRecords"`
	CorporateActions        int     `json:"corporateActions"`
	OldestNavDate           *string `json:"oldestNavDate"`
	LatestNavDate           *string `json:"latestNavDate"`
}

type AdminFundStorageDto struct {
	SchemeCode               int      `json:"schemeCode"`
	SchemeName               string   `json:"schemeName"`
	FundHouse                *string  `json:"fundHouse"`
	SchemeType               *string  `json:"schemeType"`
	SchemeCategory           *string  `json:"schemeCategory"`
	LatestNav                *float64 `json:"latestNav"`
	LatestNavDate            *string  `json:"latestNavDate"`
	NavStartDate             *string  `json:"navStartDate"`
	NavEndDate               *string  `json:"navEndDate"`
	LastSuccessfulNavCheckUtc *string `json:"lastSuccessfulNavCheckUtc"`
	NavStatus                string   `json:"navStatus"`
	IsLikelyDiscontinued     bool     `json:"isLikelyDiscontinued"`
	NavHistoryCount          int      `json:"navHistoryCount"`
	FirstStoredNavDate       *string  `json:"firstStoredNavDate"`
	LastStoredNavDate        *string  `json:"lastStoredNavDate"`
	CorporateActionCount     int      `json:"corporateActionCount"`
}

type PagedResponseDto[T any] struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
	Items      []T `json:"items"`
}
