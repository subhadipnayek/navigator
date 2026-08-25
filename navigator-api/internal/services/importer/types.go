package importer

import (
	"time"
)

// ImportStrategy defines how new transactions should be reconciled with existing records.
type ImportStrategy string

const (
	StrategySmartMerge    ImportStrategy = "SmartMerge"    // Default: Skip duplicates, insert new trades
	StrategyAppend        ImportStrategy = "Append"        // Force insert all valid trades
	StrategyWindowReplace ImportStrategy = "WindowReplace" // Replace records within the file's min/max date range
	StrategyFullOverwrite ImportStrategy = "FullOverwrite" // Reset and replace all transactions in portfolio
)

// AssetType classifies the instrument (Mutual Fund, Stock, ETF).
type AssetType string

const (
	AssetMutualFund AssetType = "MutualFund"
	AssetStock      AssetType = "Stock"
	AssetETF        AssetType = "ETF"
)

// RawTradeDraft is the standardized intermediate representation output by all BrokerParsers.
type RawTradeDraft struct {
	SourceRowNumber int       `json:"sourceRowNumber"`
	BrokerName      string    `json:"brokerName"`
	AssetType       AssetType `json:"assetType"`
	Symbol          string    `json:"symbol"`     // e.g. "INFY", "NIFTYBEES", or Fund Name
	ISIN            string    `json:"isin"`       // e.g. "INF789F01XA0", "INE736A01011"
	SchemeCode      int       `json:"schemeCode"` // AMFI Code for Mutual Funds
	SchemeName      string    `json:"schemeName"`
	TransactionDate time.Time `json:"transactionDate"`
	TransactionType string    `json:"transactionType"` // "Buy", "Sell", "SIP", "Switch In", "Switch Out"
	Units           float64   `json:"units"`
	Price           float64   `json:"price"` // NAV for MF, Price for Equity
	Amount          float64   `json:"amount"`
	TradeID         string    `json:"tradeId,omitempty"` // Broker unique trade/order ID
	FolioNumber     string    `json:"folioNumber,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	IsIgnored       bool      `json:"isIgnored"` // True if non-financial event (e.g. KYC/Address update)
	IgnoreReason    string    `json:"ignoreReason,omitempty"`
}

// IngestionDiffStatus categorizes how a draft interacts with existing portfolio records.
type IngestionDiffStatus string

const (
	StatusNew        IngestionDiffStatus = "New"        // Will be inserted
	StatusDuplicate  IngestionDiffStatus = "Duplicate"  // Already exists (skipped in SmartMerge)
	StatusIgnored    IngestionDiffStatus = "Ignored"    // Non-financial row (filtered)
	StatusUnresolved IngestionDiffStatus = "Unresolved" // Scheme/Symbol could not be matched
	StatusInvalid    IngestionDiffStatus = "Invalid"    // Invalid date/units/price
)

// ProcessedTradeRecord is the fully resolved and diffed transaction item presented in the dry-run preview.
type ProcessedTradeRecord struct {
	RawTradeDraft
	Status        IngestionDiffStatus `json:"status"`
	StatusMessage string              `json:"statusMessage,omitempty"`
	Fingerprint   string              `json:"fingerprint"` // Deterministic SHA-256 hash
}

// ImportPreviewResult represents the complete dry-run analysis returned to the user before commit.
type ImportPreviewResult struct {
	DetectedFormat          string                 `json:"detectedFormat"`
	DetectedFormatLabel     string                 `json:"detectedFormatLabel"`
	TotalRowsParsed         int                    `json:"totalRowsParsed"`
	ValidFinancialTrades    int                    `json:"validFinancialTrades"`
	NewTradesCount          int                    `json:"newTradesCount"`
	DuplicateTradesCount    int                    `json:"duplicateTradesCount"`
	IgnoredEventsCount      int                    `json:"ignoredEventsCount"`
	UnresolvedCount         int                    `json:"unresolvedCount"`
	EarliestTransactionDate string                 `json:"earliestTransactionDate,omitempty"`
	LatestTransactionDate   string                 `json:"latestTransactionDate,omitempty"`
	TotalInflow             float64                `json:"totalInflow"`
	TotalOutflow            float64                `json:"totalOutflow"`
	RecommendedStrategy     ImportStrategy         `json:"recommendedStrategy"`
	Transactions            []ProcessedTradeRecord `json:"transactions"`
	AvailableStrategies     []ImportStrategyOption `json:"availableStrategies"`
}

// ImportStrategyOption describes each ingestion strategy for the UI selector.
type ImportStrategyOption struct {
	Key         ImportStrategy `json:"key"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Recommended bool           `json:"recommended"`
}

// ImportCommitRequest specifies the batch of verified transactions and chosen strategy to commit.
type ImportCommitRequest struct {
	Strategy     ImportStrategy         `json:"strategy"`
	Transactions []ProcessedTradeRecord `json:"transactions"`
}

// BrokerParser is the core open-for-extension interface that every institutional format implements.
type BrokerParser interface {
	// ID returns unique lowercase machine identifier (e.g. "cams_cas", "kfintech", "zerodha_mf")
	ID() string
	// DisplayName returns human-readable label for UI badges (e.g. "CAMS Consolidated Account Statement")
	DisplayName() string
	// Match determines if the provided raw 2D grid matches this parser's structural signature
	Match(rows [][]string) bool
	// Parse extracts standardized trade drafts from raw rows
	Parse(rows [][]string) ([]RawTradeDraft, error)
}
