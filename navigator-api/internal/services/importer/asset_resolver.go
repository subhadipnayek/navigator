package importer

import (
	"context"
	"strconv"
	"strings"

	"navigator-api/internal/models"

	"gorm.io/gorm"
)

type AssetResolver struct {
	db *gorm.DB
}

func NewAssetResolver(db *gorm.DB) *AssetResolver {
	return &AssetResolver{db: db}
}

// Resolve identifies AMFI SchemeCode and canonical Name for a draft trade.
func (r *AssetResolver) Resolve(ctx context.Context, draft *RawTradeDraft) {
	// 1. If ISIN is present, try exact lookup
	if draft.ISIN != "" {
		var fund models.Fund
		err := r.db.WithContext(ctx).
			Where("isin_growth = ? OR isin_div_reinvestment = ?", draft.ISIN, draft.ISIN).
			First(&fund).Error
		if err == nil {
			draft.SchemeCode = fund.SchemeCode
			if draft.SchemeName == "" || strings.HasPrefix(draft.SchemeName, "Fund ") {
				draft.SchemeName = fund.SchemeName
			}
			draft.AssetType = AssetMutualFund
			return
		}
	}

	// 2. If SchemeCode is already a valid integer
	if draft.SchemeCode > 0 {
		var fund models.Fund
		if err := r.db.WithContext(ctx).Where("scheme_code = ?", draft.SchemeCode).First(&fund).Error; err == nil {
			draft.SchemeName = fund.SchemeName
			draft.AssetType = AssetMutualFund
			return
		}
	}

	// 3. If Symbol looks like a numeric SchemeCode
	if code, err := strconv.Atoi(strings.TrimSpace(draft.Symbol)); err == nil && code > 0 {
		var fund models.Fund
		if err := r.db.WithContext(ctx).Where("scheme_code = ?", code).First(&fund).Error; err == nil {
			draft.SchemeCode = fund.SchemeCode
			draft.SchemeName = fund.SchemeName
			draft.AssetType = AssetMutualFund
			return
		}
	}

	// 4. Try fuzzy lookup by scheme name for mutual funds
	if draft.SchemeName != "" && draft.AssetType == AssetMutualFund {
		var fund models.Fund
		cleanName := strings.TrimSpace(draft.SchemeName)
		if err := r.db.WithContext(ctx).Where("scheme_name ILIKE ?", "%"+cleanName+"%").First(&fund).Error; err == nil {
			draft.SchemeCode = fund.SchemeCode
			draft.SchemeName = fund.SchemeName
			return
		}
	}

	// 5. If it's a stock / ETF with ISIN starting with "INE" or stock symbol
	if strings.HasPrefix(draft.ISIN, "INE") || draft.AssetType == AssetStock || draft.AssetType == AssetETF {
		// If SchemeCode is 0, we assign a deterministic synthetic code or symbol hash for stock tracking
		if draft.SchemeCode <= 0 {
			draft.SchemeCode = generateSyntheticEquityCode(draft.Symbol, draft.ISIN)
		}
	}
}

func generateSyntheticEquityCode(symbol, isin string) int {
	key := symbol
	if key == "" {
		key = isin
	}
	var hash uint32
	for i := 0; i < len(key); i++ {
		hash = hash*31 + uint32(key[i])
	}
	// Reserve 900,000+ range for synthetic equity assets
	return 900000 + int(hash%99999)
}
