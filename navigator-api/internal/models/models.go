package models

import (
	"time"
)

// Fund represents a mutual fund scheme master record.
type Fund struct {
	SchemeCode          int               `gorm:"column:scheme_code;primaryKey;autoIncrement:false" json:"schemeCode"`
	SchemeName          string            `gorm:"column:scheme_name;size:500;not null;index:idx_funds_scheme_name" json:"schemeName"`
	FundHouse           *string           `gorm:"column:fund_house;size:300;index:idx_funds_fund_house" json:"fundHouse"`
	SchemeType          *string           `gorm:"column:scheme_type;size:200" json:"schemeType"`
	SchemeCategory      *string           `gorm:"column:scheme_category;size:300" json:"schemeCategory"`
	IsinGrowth          *string           `gorm:"column:isin_growth;size:20" json:"isinGrowth"`
	IsinDivReinvestment *string           `gorm:"column:isin_div_reinvestment;size:20" json:"isinDivReinvestment"`
	CreatedAtUtc        time.Time         `gorm:"column:created_at;not null;autoCreateTime" json:"createdAt"`
	UpdatedAtUtc        time.Time         `gorm:"column:updated_at;not null;autoUpdateTime" json:"updatedAt"`
	NavSyncTracker      *NavSyncTracker   `gorm:"foreignKey:SchemeCode;references:SchemeCode" json:"navSyncTracker,omitempty"`
	NavHistories        []NavHistory      `gorm:"foreignKey:SchemeCode;references:SchemeCode" json:"-"`
	CorporateActions    []CorporateAction `gorm:"foreignKey:SchemeCode;references:SchemeCode" json:"-"`
}

func (Fund) TableName() string {
	return "Funds"
}

// NavHistory stores historical daily Net Asset Values for a mutual fund.
type NavHistory struct {
	Id         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemeCode int       `gorm:"column:scheme_code;not null;uniqueIndex:idx_nav_histories_scheme_date,priority:1" json:"schemeCode"`
	Date       time.Time `gorm:"column:date;type:date;not null;uniqueIndex:idx_nav_histories_scheme_date,priority:2" json:"date"`
	Nav        float64   `gorm:"column:nav;type:numeric(18,6);not null" json:"nav"`
}

func (NavHistory) TableName() string {
	return "NavHistories"
}

// NavSyncTracker tracks metadata and synchronization state for each fund's NAV history.
type NavSyncTracker struct {
	SchemeCode              int        `gorm:"column:scheme_code;primaryKey;autoIncrement:false" json:"schemeCode"`
	LatestNav               *float64   `gorm:"column:latest_nav;type:numeric(18,6)" json:"latestNav"`
	LatestNavDate           *time.Time `gorm:"column:latest_nav_date;type:date" json:"latestNavDate"`
	NavStartDate            *time.Time `gorm:"column:nav_start_date;type:date" json:"navStartDate"`
	NavEndDate              *time.Time `gorm:"column:nav_end_date;type:date" json:"navEndDate"`
	LastLatestNavCheckAtIst *time.Time `gorm:"column:last_latest_nav_check_at_ist" json:"lastLatestNavCheckAtIst"`
	LastHistoryCheckAtIst   *time.Time `gorm:"column:last_history_check_at_ist" json:"lastHistoryCheckAtIst"`
	NavHistoryCount         int        `gorm:"column:nav_history_count;not null;default:0" json:"navHistoryCount"`
	CorporateActionCount    int        `gorm:"column:corporate_action_count;not null;default:0" json:"corporateActionCount"`
	UpdatedAtUtc            time.Time  `gorm:"column:updated_at;not null;autoUpdateTime" json:"updatedAt"`
}

func (NavSyncTracker) TableName() string {
	return "NavSyncTrackers"
}

// CorporateAction tracks fund splits, mergers, bonuses, and data anomalies.
type CorporateAction struct {
	Id               int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemeCode       int       `gorm:"column:scheme_code;not null;index:idx_corporate_actions_scheme_date,priority:1" json:"schemeCode"`
	ActionDate       time.Time `gorm:"column:action_date;type:date;not null;index:idx_corporate_actions_scheme_date,priority:2" json:"actionDate"`
	ActionType       string    `gorm:"column:action_type;size:50;not null" json:"actionType"` // Split, Merger, Bonus, Discontinuity, MissingData, Other
	AdjustmentFactor float64   `gorm:"column:adjustment_factor;type:numeric(18,6);not null;default:1.0" json:"adjustmentFactor"`
	OldUnits         *float64  `gorm:"column:old_units;type:numeric(18,6)" json:"oldUnits"`
	NewUnits         *float64  `gorm:"column:new_units;type:numeric(18,6)" json:"newUnits"`
	Ratio            *string   `gorm:"column:ratio;size:50" json:"ratio"`
	ConfidenceScore  *float64  `gorm:"column:confidence_score;type:numeric(5,2)" json:"confidenceScore"`
	Notes            *string   `gorm:"column:notes;size:1000" json:"notes"`
	IsAutoDetected   bool      `gorm:"column:is_auto_detected;not null;default:false" json:"isAutoDetected"`
	IsVerified       bool      `gorm:"column:is_verified;not null;default:false" json:"isVerified"`
	CreatedAtUtc     time.Time `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
	UpdatedAtUtc     time.Time `gorm:"column:updated_at_utc;not null;autoUpdateTime" json:"updatedAtUtc"`
}

func (CorporateAction) TableName() string {
	return "CorporateActions"
}

// AppUser represents a user account with RBAC support.
type AppUser struct {
	Id                 int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username           string    `gorm:"column:username;size:80;not null" json:"username"`
	NormalizedUsername string    `gorm:"column:normalized_username;size:80;not null;uniqueIndex:idx_app_users_normalized_username" json:"normalizedUsername"`
	Email              string    `gorm:"column:email;size:256;not null" json:"email"`
	NormalizedEmail    string    `gorm:"column:normalized_email;size:256;not null;uniqueIndex:idx_app_users_normalized_email" json:"normalizedEmail"`
	PasswordHash       string    `gorm:"column:password_hash;size:500;not null" json:"-"`
	Role               string    `gorm:"column:role;size:30;not null;default:'User'" json:"role"` // "Admin" or "User"
	CreatedAtUtc       time.Time `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
}

func (AppUser) TableName() string {
	return "AppUsers"
}

// UserPortfolio represents a personal portfolio container.
type UserPortfolio struct {
	Id           int64                  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId       int                    `gorm:"column:user_id;not null;uniqueIndex:idx_user_portfolios_user_name,priority:1" json:"userId"`
	Name         string                 `gorm:"column:name;size:200;not null;uniqueIndex:idx_user_portfolios_user_name,priority:2" json:"name"`
	CreatedAtUtc time.Time              `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
	UpdatedAtUtc time.Time              `gorm:"column:updated_at_utc;not null;autoUpdateTime" json:"updatedAtUtc"`
	Transactions []PortfolioTransaction `gorm:"foreignKey:PortfolioId;references:Id" json:"transactions,omitempty"`
}

func (UserPortfolio) TableName() string {
	return "UserPortfolios"
}

// PortfolioTransaction represents an individual buy, sell, SIP, or dividend reinvestment transaction.
type PortfolioTransaction struct {
	Id              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	PortfolioId     int64     `gorm:"column:portfolio_id;not null;index:idx_portfolio_transactions_port_date,priority:1;index:idx_portfolio_transactions_port_scheme_date,priority:1" json:"portfolioId"`
	SchemeCode      int       `gorm:"column:scheme_code;not null;index:idx_portfolio_transactions_port_scheme_date,priority:2" json:"schemeCode"`
	TransactionDate time.Time `gorm:"column:transaction_date;type:date;not null;index:idx_portfolio_transactions_port_date,priority:2;index:idx_portfolio_transactions_port_scheme_date,priority:3" json:"transactionDate"`
	TransactionType string    `gorm:"column:transaction_type;size:50;not null" json:"transactionType"` // Purchase, Sell, SIP, SwitchIn, SwitchOut, DividendReinvestment
	Units           float64   `gorm:"column:units;type:numeric(18,6);not null" json:"units"`
	Nav             float64   `gorm:"column:nav;type:numeric(18,6);not null" json:"nav"`
	Amount          float64   `gorm:"column:amount;type:numeric(18,2);not null" json:"amount"`
	Notes           *string   `gorm:"column:notes;size:500" json:"notes"`
	Isin            *string   `gorm:"column:isin;size:20" json:"isin"`
	TradeId         *string   `gorm:"column:trade_id;size:100" json:"tradeId,omitempty"`
	Fingerprint     string    `gorm:"column:fingerprint;size:64;not null;index:idx_portfolio_transactions_port_fp,priority:2;default:''" json:"fingerprint"`
	CreatedAtUtc    time.Time `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
	UpdatedAtUtc    time.Time `gorm:"column:updated_at_utc;not null;autoUpdateTime" json:"updatedAtUtc"`

	Fund *Fund `gorm:"foreignKey:SchemeCode;references:SchemeCode" json:"fund,omitempty"`
}

func (PortfolioTransaction) TableName() string {
	return "PortfolioTransactions"
}

// UserGridPreference persists UI grid layout and filter settings per user.
type UserGridPreference struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId       int       `gorm:"column:user_id;not null;uniqueIndex:idx_user_grid_preferences_user_key,priority:1" json:"userId"`
	GridKey      string    `gorm:"column:grid_key;size:100;not null;uniqueIndex:idx_user_grid_preferences_user_key,priority:2" json:"gridKey"`
	StateJson    string    `gorm:"column:state_json;type:text;not null" json:"stateJson"`
	CreatedAtUtc time.Time `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
	UpdatedAtUtc time.Time `gorm:"column:updated_at_utc;not null;autoUpdateTime" json:"updatedAtUtc"`
}

func (UserGridPreference) TableName() string {
	return "UserGridPreferences"
}

// UserPortfolioDashboardCache caches heavy dashboard aggregates for quick access.
type UserPortfolioDashboardCache struct {
	Id                      int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserId                  int       `gorm:"column:user_id;not null;uniqueIndex:idx_user_port_dash_cache,priority:1" json:"userId"`
	PortfolioId             int64     `gorm:"column:portfolio_id;not null;uniqueIndex:idx_user_port_dash_cache,priority:2" json:"portfolioId"`
	IncludeFullyExitedFunds bool      `gorm:"column:include_fully_exited_funds;not null;uniqueIndex:idx_user_port_dash_cache,priority:3" json:"includeFullyExitedFunds"`
	DependencyKey           string    `gorm:"column:dependency_key;size:200;not null" json:"dependencyKey"`
	DashboardJson           string    `gorm:"column:dashboard_json;type:text;not null" json:"dashboardJson"`
	CreatedAtUtc            time.Time `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
	UpdatedAtUtc            time.Time `gorm:"column:updated_at_utc;not null;autoUpdateTime" json:"updatedAtUtc"`
}

func (UserPortfolioDashboardCache) TableName() string {
	return "UserPortfolioDashboardCaches"
}

// ApiEndpointConfig persists configurable URLs, methods, headers, and health status for upstream third-party APIs.
type ApiEndpointConfig struct {
	Key                  string     `gorm:"column:key;primaryKey;size:100" json:"key"`
	Name                 string     `gorm:"column:name;size:200;not null" json:"name"`
	Description          string     `gorm:"column:description;size:500;not null" json:"description"`
	Url                  string     `gorm:"column:url;size:1000;not null" json:"url"`
	DefaultUrl           string     `gorm:"column:default_url;size:1000;not null" json:"defaultUrl"`
	Method               string     `gorm:"column:method;size:10;not null;default:'GET'" json:"method"`
	HeadersJson          string     `gorm:"column:headers_json;type:text;not null;default:'{}'" json:"headersJson"`
	PayloadTemplate      string     `gorm:"column:payload_template;type:text;not null;default:''" json:"payloadTemplate"`
	IsActive             bool       `gorm:"column:is_active;not null;default:true" json:"isActive"`
	LastHealthCheckAtUtc *time.Time `gorm:"column:last_health_check_at_utc" json:"lastHealthCheckAtUtc"`
	LastHealthStatus     string     `gorm:"column:last_health_status;size:50;not null;default:'Unknown'" json:"lastHealthStatus"` // Healthy, Degraded, Down, Unknown
	LastStatusCode       int        `gorm:"column:last_status_code;not null;default:0" json:"lastStatusCode"`
	LastLatencyMs        int64      `gorm:"column:last_latency_ms;not null;default:0" json:"lastLatencyMs"`
	LastErrorMessage     *string    `gorm:"column:last_error_message;size:1000" json:"lastErrorMessage"`
	CreatedAtUtc         time.Time  `gorm:"column:created_at_utc;not null;autoCreateTime" json:"createdAtUtc"`
	UpdatedAtUtc         time.Time  `gorm:"column:updated_at_utc;not null;autoUpdateTime" json:"updatedAtUtc"`
}

func (ApiEndpointConfig) TableName() string {
	return "ApiEndpointConfigs"
}
