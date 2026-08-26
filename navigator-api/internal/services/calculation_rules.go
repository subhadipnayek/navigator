package services

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"navigator-api/internal/models"
)

const (
	DateFormatDDMMYYYY      = "02-01-2006"
	DateFormatYYYYMMDD      = "2006-01-02"
	DefaultNavToleranceDays = 7
)

var ISTLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		loc = time.FixedZone("IST", int(5.5*3600))
	}
	ISTLocation = loc
}

func IndiaNow() time.Time {
	return time.Now().In(ISTLocation)
}

func IndiaToday() time.Time {
	now := IndiaNow()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(DateFormatDDMMYYYY, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	if t, err := time.Parse("02-Jan-2006", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	if t, err := time.Parse("02-Jan-06", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	if t, err := time.Parse(DateFormatYYYYMMDD, s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("invalid date format '%s'", s)
}

func FormatDateDDMMYYYY(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(DateFormatDDMMYYYY)
}

func FormatDatePtr(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	s := t.Format(DateFormatDDMMYYYY)
	return &s
}

func DaysBetween(a, b time.Time) int {
	return int(math.Round(b.Sub(a).Hours() / 24.0))
}

func FindNavOnOrBefore(sortedAsc []models.NavHistory, target time.Time) *models.NavHistory {
	lo := 0
	hi := len(sortedAsc) - 1
	var best *models.NavHistory

	for lo <= hi {
		mid := (lo + hi) / 2
		if !sortedAsc[mid].Date.After(target) {
			best = &sortedAsc[mid]
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func FindNavOnOrAfter(sortedAsc []models.NavHistory, target time.Time, toleranceDays int, allowPreviousAtEnd bool) *models.NavHistory {
	if toleranceDays <= 0 {
		toleranceDays = DefaultNavToleranceDays
	}

	lo := 0
	hi := len(sortedAsc) - 1
	var best *models.NavHistory

	for lo <= hi {
		mid := (lo + hi) / 2
		if !sortedAsc[mid].Date.Before(target) {
			best = &sortedAsc[mid]
			hi = mid - 1
		} else {
			lo = mid + 1
		}
	}

	if best == nil && allowPreviousAtEnd && len(sortedAsc) > 0 {
		last := &sortedAsc[len(sortedAsc)-1]
		if DaysBetween(last.Date, target) <= toleranceDays {
			return last
		}
	}

	if best != nil && DaysBetween(target, best.Date) > toleranceDays {
		return nil
	}

	return best
}

func FindClosestNav(sortedAsc []models.NavHistory, target time.Time, toleranceDays int) *models.NavHistory {
	if toleranceDays <= 0 {
		toleranceDays = DefaultNavToleranceDays
	}

	before := FindNavOnOrBefore(sortedAsc, target)
	if before != nil && DaysBetween(before.Date, target) <= toleranceDays {
		return before
	}

	after := FindNavOnOrAfter(sortedAsc, target, toleranceDays, false)
	if after != nil && DaysBetween(target, after.Date) <= toleranceDays {
		return after
	}

	return nil
}

func CalculateCagr(startValue, endValue float64, startDate, endDate time.Time) *float64 {
	if startValue <= 0 || endValue <= 0 || !endDate.After(startDate) {
		return nil
	}

	days := DaysBetween(startDate, endDate)
	years := float64(days) / DaysPerYear
	if years <= 0 {
		return nil
	}

	cagr := (math.Pow(endValue/startValue, 1.0/years) - 1.0) * 100.0
	res := roundFloat(cagr, 2)
	return &res
}

func GetMedian(sortedValues []float64) *float64 {
	n := len(sortedValues)
	if n == 0 {
		return nil
	}
	if n%2 == 0 {
		val := roundFloat((sortedValues[n/2-1]+sortedValues[n/2])/2.0, 2)
		return &val
	}
	val := sortedValues[n/2]
	return &val
}

func IsSeriesBreakAction(actionType string) bool {
	return strings.EqualFold(actionType, "Discontinuity") || strings.EqualFold(actionType, "MissingData")
}

func ComputeAdjustmentFactor(actions []models.CorporateActionDto, fromDate, toDate time.Time) float64 {
	factor := 1.0
	for _, a := range actions {
		actDate, err := ParseDate(a.ActionDate)
		if err != nil {
			continue
		}
		if IsSeriesBreakAction(a.ActionType) {
			continue
		}
		if actDate.After(fromDate) && !actDate.After(toDate) {
			factor *= a.AdjustmentFactor
		}
	}
	return factor
}

func SampleNavDataPoints(data []models.NavDataPointDto, interval string) []models.NavDataPointDto {
	if interval == "" || strings.EqualFold(interval, "daily") {
		return data
	}

	if strings.EqualFold(interval, "weekly") {
		var result []models.NavDataPointDto
		type key struct {
			Year int
			Week int
		}
		m := make(map[key]models.NavDataPointDto)
		var keys []key

		for _, item := range data {
			year, week := item.Date.ISOWeek()
			k := key{Year: year, Week: week}
			if _, exists := m[k]; !exists {
				keys = append(keys, k)
			}
			m[k] = item
		}
		for _, k := range keys {
			result = append(result, m[k])
		}
		return result
	}

	if strings.EqualFold(interval, "monthly") {
		var result []models.NavDataPointDto
		type key struct {
			Year  int
			Month time.Month
		}
		m := make(map[key]models.NavDataPointDto)
		var keys []key

		for _, item := range data {
			k := key{Year: item.Date.Year(), Month: item.Date.Month()}
			if _, exists := m[k]; !exists {
				keys = append(keys, k)
			}
			m[k] = item
		}
		for _, k := range keys {
			result = append(result, m[k])
		}
		return result
	}

	return data
}

func SampleRollingReturns(data []models.RollingReturnDto, interval string) []models.RollingReturnDto {
	if interval == "" || strings.EqualFold(interval, "daily") || len(data) == 0 {
		return data
	}

	if strings.EqualFold(interval, "weekly") {
		var result []models.RollingReturnDto
		type key struct {
			Year int
			Week int
		}
		m := make(map[key]models.RollingReturnDto)
		var keys []key

		for _, item := range data {
			t, err := ParseDate(item.Date)
			if err != nil {
				continue
			}
			year, week := t.ISOWeek()
			k := key{Year: year, Week: week}
			if _, exists := m[k]; !exists {
				keys = append(keys, k)
			}
			m[k] = item
		}
		for _, k := range keys {
			result = append(result, m[k])
		}
		return result
	}

	if strings.EqualFold(interval, "monthly") {
		var result []models.RollingReturnDto
		type key struct {
			Year  int
			Month time.Month
		}
		m := make(map[key]models.RollingReturnDto)
		var keys []key

		for _, item := range data {
			t, err := ParseDate(item.Date)
			if err != nil {
				continue
			}
			k := key{Year: t.Year(), Month: t.Month()}
			if _, exists := m[k]; !exists {
				keys = append(keys, k)
			}
			m[k] = item
		}
		for _, k := range keys {
			result = append(result, m[k])
		}
		return result
	}

	return data
}

func SampleDates(dates []time.Time, interval string) []time.Time {
	if interval == "" || strings.EqualFold(interval, "daily") || len(dates) == 0 {
		return dates
	}

	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })

	if strings.EqualFold(interval, "weekly") {
		var result []time.Time
		type key struct {
			Year int
			Week int
		}
		m := make(map[key]time.Time)
		var keys []key
		for _, d := range dates {
			year, week := d.ISOWeek()
			k := key{Year: year, Week: week}
			if _, exists := m[k]; !exists {
				keys = append(keys, k)
			}
			m[k] = d
		}
		for _, k := range keys {
			result = append(result, m[k])
		}
		return result
	}

	if strings.EqualFold(interval, "monthly") {
		var result []time.Time
		type key struct {
			Year  int
			Month time.Month
		}
		m := make(map[key]time.Time)
		var keys []key
		for _, d := range dates {
			k := key{Year: d.Year(), Month: d.Month()}
			if _, exists := m[k]; !exists {
				keys = append(keys, k)
			}
			m[k] = d
		}
		for _, k := range keys {
			result = append(result, m[k])
		}
		return result
	}

	return dates
}
