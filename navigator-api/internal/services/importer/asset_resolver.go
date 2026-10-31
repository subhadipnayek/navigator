package importer

import (
	"context"
	"math"
	"regexp"
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

var (
	formerlyRegex    = regexp.MustCompile(`(?i)[-\s]*[\(\[]?formerly\s+[^\]\)]+[\)\]]?`)
	nonAlphaNumRegex = regexp.MustCompile(`[^a-zA-Z0-9\s]`)
	multipleSpaces   = regexp.MustCompile(`\s+`)
)

// Resolve identifies AMFI SchemeCode and canonical Name for a draft trade.
func (r *AssetResolver) Resolve(ctx context.Context, draft *RawTradeDraft) {
	if r.db == nil {
		return
	}

	// 1. If ISIN is present, try exact lookup
	if isin := strings.TrimSpace(draft.ISIN); isin != "" {
		var fund models.Fund
		err := r.db.WithContext(ctx).
			Where("isin_growth = ? OR isin_div_reinvestment = ? OR LOWER(isin_growth) LIKE LOWER(?) OR LOWER(isin_div_reinvestment) LIKE LOWER(?)", isin, isin, isin, isin).
			First(&fund).Error
		if err == nil && fund.SchemeCode > 0 {
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
		if err := r.db.WithContext(ctx).Where("scheme_code = ?", draft.SchemeCode).First(&fund).Error; err == nil && fund.SchemeCode > 0 {
			draft.SchemeName = fund.SchemeName
			draft.AssetType = AssetMutualFund
			return
		}
	}

	// 3. If Symbol looks like a numeric SchemeCode
	if code, err := strconv.Atoi(strings.TrimSpace(draft.Symbol)); err == nil && code > 0 {
		var fund models.Fund
		if err := r.db.WithContext(ctx).Where("scheme_code = ?", code).First(&fund).Error; err == nil && fund.SchemeCode > 0 {
			draft.SchemeCode = fund.SchemeCode
			draft.SchemeName = fund.SchemeName
			draft.AssetType = AssetMutualFund
			return
		}
	}

	// 4. Multi-strategy Mutual Fund Scheme Name resolution
	if draft.SchemeName != "" && draft.AssetType == AssetMutualFund {
		if resolved := r.resolveFundByName(ctx, draft.SchemeName); resolved != nil {
			draft.SchemeCode = resolved.SchemeCode
			draft.SchemeName = resolved.SchemeName
			return
		}
	}

	// 5. If it's a stock / ETF with ISIN starting with "INE" or stock symbol
	if strings.HasPrefix(draft.ISIN, "INE") || draft.AssetType == AssetStock || draft.AssetType == AssetETF {
		if draft.SchemeCode <= 0 {
			draft.SchemeCode = generateSyntheticEquityCode(draft.Symbol, draft.ISIN)
		}
		return
	}

	// 6. Mutual Fund synthetic fallback if name is present
	if draft.AssetType == AssetMutualFund && draft.SchemeName != "" && draft.SchemeCode <= 0 {
		draft.SchemeCode = generateSyntheticMutualFundCode(draft.SchemeName, draft.FolioNumber)
	}
}

func (r *AssetResolver) resolveFundByName(ctx context.Context, rawName string) *models.Fund {
	clean := cleanFundName(rawName)
	if clean == "" {
		return nil
	}

	// Strategy A: Direct exact or substring query
	var directFund models.Fund
	if err := r.db.WithContext(ctx).Where("LOWER(scheme_name) LIKE LOWER(?)", clean).First(&directFund).Error; err == nil && directFund.SchemeCode > 0 {
		return &directFund
	}
	if err := r.db.WithContext(ctx).Where("LOWER(scheme_name) LIKE LOWER(?)", "%"+clean+"%").First(&directFund).Error; err == nil && directFund.SchemeCode > 0 {
		return &directFund
	}

	// Strategy B: Token-Intersection and heuristic compatibility scoring
	plan, option, tokens := extractQueryFeatures(rawName)
	if len(tokens) == 0 {
		return nil
	}

	// Select the most distinctive keyword tokens (non-numeric, >= 3 chars)
	var searchKeywords []string
	for _, tok := range tokens {
		if len(tok) >= 3 && !isAllDigits(tok) {
			searchKeywords = append(searchKeywords, tok)
		}
	}
	if len(searchKeywords) == 0 {
		searchKeywords = tokens
	}

	// Query candidate funds from database using keyword conjunctions
	var candidates []models.Fund
	query := r.db.WithContext(ctx).Model(&models.Fund{})
	for _, kw := range searchKeywords {
		query = query.Where("LOWER(scheme_name) LIKE LOWER(?) OR LOWER(fund_house) LIKE LOWER(?)", "%"+kw+"%", "%"+kw+"%")
	}
	if err := query.Limit(40).Find(&candidates).Error; err != nil || len(candidates) == 0 {
		// Fallback to top 2 keywords if all keywords together yielded 0 records
		if len(searchKeywords) > 2 {
			q2 := r.db.WithContext(ctx).Model(&models.Fund{})
			for _, kw := range searchKeywords[:2] {
				q2 = q2.Where("LOWER(scheme_name) LIKE LOWER(?) OR LOWER(fund_house) LIKE LOWER(?)", "%"+kw+"%", "%"+kw+"%")
			}
			_ = q2.Limit(40).Find(&candidates).Error
		} else if len(searchKeywords) > 1 {
			qSingle := r.db.WithContext(ctx).Model(&models.Fund{})
			qSingle = qSingle.Where("LOWER(scheme_name) LIKE LOWER(?) OR LOWER(fund_house) LIKE LOWER(?)", "%"+searchKeywords[0]+"%", "%"+searchKeywords[0]+"%")
			_ = qSingle.Limit(40).Find(&candidates).Error
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Score candidates based on token overlap, plan compatibility, and option compatibility
	var bestFund *models.Fund
	var maxScore float64 = -100.0

	for i := range candidates {
		c := &candidates[i]
		candNameLower := strings.ToLower(c.SchemeName)
		candHouseLower := ""
		if c.FundHouse != nil {
			candHouseLower = strings.ToLower(*c.FundHouse)
		}

		score := 0.0

		// 1. Token matches
		for _, tok := range tokens {
			if strings.Contains(candNameLower, tok) || strings.Contains(candHouseLower, tok) {
				score += 15.0
			}
		}

		// 2. Plan compatibility (Direct vs Regular)
		isCandDirect := strings.Contains(candNameLower, "direct")
		isCandRegular := strings.Contains(candNameLower, "regular")
		if plan == "Direct" {
			if isCandDirect {
				score += 30.0
			} else if isCandRegular {
				score -= 40.0
			}
		} else if plan == "Regular" {
			if isCandRegular {
				score += 30.0
			} else if isCandDirect {
				score -= 40.0
			}
		}

		// 3. Option compatibility (Growth vs IDCW/Dividend)
		isCandGrowth := strings.Contains(candNameLower, "growth")
		isCandIDCW := strings.Contains(candNameLower, "idcw") || strings.Contains(candNameLower, "dividend")
		if option == "Growth" {
			if isCandGrowth {
				score += 25.0
			} else if isCandIDCW {
				score -= 35.0
			}
		} else if option == "IDCW" {
			if isCandIDCW {
				score += 25.0
			} else if isCandGrowth {
				score -= 35.0
			}
		}

		// 4. Proximity penalty
		diffLen := math.Abs(float64(len(candNameLower) - len(rawName)))
		score -= diffLen * 0.04

		if score > maxScore {
			maxScore = score
			bestFund = c
		}
	}

	if maxScore >= 30.0 && bestFund != nil {
		return bestFund
	}

	return nil
}

func cleanFundName(s string) string {
	cleaned := formerlyRegex.ReplaceAllString(s, "")
	cleaned = strings.ReplaceAll(cleaned, "-", " ")
	cleaned = strings.ReplaceAll(cleaned, "/", " ")
	cleaned = strings.TrimSpace(cleaned)
	return multipleSpaces.ReplaceAllString(cleaned, " ")
}

var commonMfStopwords = map[string]bool{
	"mutual": true, "fund": true, "funds": true, "plan": true, "option": true,
	"the": true, "and": true, "of": true, "for": true, "in": true, "dp": true,
	"dg": true, "dir": true, "direct": true, "reg": true, "regular": true,
	"growth": true, "gr": true, "g": true, "idcw": true, "div": true,
	"dividend": true, "passive": true, "scheme": true, "trust": true, "india": true,
}

func extractQueryFeatures(raw string) (plan string, option string, tokens []string) {
	lower := strings.ToLower(raw)
	lower = formerlyRegex.ReplaceAllString(lower, "")

	// 1. Detect Plan
	switch {
	case strings.Contains(lower, "direct") || strings.Contains(lower, "dir") || strings.Contains(lower, "dp") || strings.Contains(lower, "dg"):
		plan = "Direct"
	case strings.Contains(lower, "regular") || strings.Contains(lower, "reg") || strings.Contains(lower, "rp") || strings.Contains(lower, "rg"):
		plan = "Regular"
	}

	// 2. Detect Option
	switch {
	case strings.Contains(lower, "growth") || strings.Contains(lower, "gr") || strings.Contains(lower, " g") || strings.HasSuffix(lower, "-g") || strings.Contains(lower, "gro"):
		option = "Growth"
	case strings.Contains(lower, "idcw") || strings.Contains(lower, "dividend") || strings.Contains(lower, "div") || strings.Contains(lower, "reinvest") || strings.Contains(lower, "payout"):
		option = "IDCW"
	}

	// 3. Domain alias expansion
	expanded := lower
	expanded = strings.ReplaceAll(expanded, "q50", " quality 50 ")
	expanded = strings.ReplaceAll(expanded, "fof", " fund of fund ")
	expanded = strings.ReplaceAll(expanded, "nasdaq100", " nasdaq 100 ")
	expanded = strings.ReplaceAll(expanded, "midcap150", " midcap 150 ")
	expanded = strings.ReplaceAll(expanded, "nifty50", " nifty 50 ")
	expanded = strings.ReplaceAll(expanded, "niftynext50", " nifty next 50 ")
	expanded = strings.ReplaceAll(expanded, "ppfas", " parag parikh ")

	// 4. Tokenization & stopword removal
	cleaned := nonAlphaNumRegex.ReplaceAllString(expanded, " ")
	parts := strings.Fields(cleaned)
	seen := make(map[string]bool)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) < 2 {
			continue
		}
		if commonMfStopwords[p] && !isAllDigits(p) {
			continue
		}
		if !seen[p] {
			seen[p] = true
			tokens = append(tokens, p)
		}
	}

	return plan, option, tokens
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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

func generateSyntheticMutualFundCode(schemeName, folio string) int {
	key := strings.ToLower(strings.TrimSpace(schemeName))
	var hash uint32
	for i := 0; i < len(key); i++ {
		hash = hash*31 + uint32(key[i])
	}
	// Reserve 800,000+ range for synthetic mutual fund assets
	return 800000 + int(hash%99999)
}
