package util

import (
	"time"

	"github.com/blueship581/cyfreshfood/internal/constants"
)

// FoodCalculator 食品保质期计算：剩余天数、到期日、新鲜度状态。
type FoodCalculator struct{}

// NewFoodCalculator 构造计算器。
func NewFoodCalculator() *FoodCalculator { return &FoodCalculator{} }

// EffectiveOpenedShelfLifeDays 返回生效的开封后食用天数：未填写（nil 或 0）按默认 7 天。
func EffectiveOpenedShelfLifeDays(days *int) int {
	if days == nil || *days <= 0 {
		return constants.DefaultOpenedShelfLifeDays
	}
	return *days
}

// CalculateExpiryDate 计算到期日：
// 未开封时按生产日期+保质期；已开封时取「原保质期到期日」与「开封时间+开封后食用天数」中较早者。
// openedShelfLifeDays 为空时按默认 7 天；开封时间为空时只按原保质期计算。
func (c *FoodCalculator) CalculateExpiryDate(productionDate *time.Time, shelfLifeDays int, openedAt *time.Time, openedShelfLifeDays *int) *time.Time {
	var expiry *time.Time
	if shelfLifeDays > 0 {
		base := time.Now()
		if productionDate != nil {
			base = *productionDate
		}
		e := base.AddDate(0, 0, shelfLifeDays)
		expiry = &e
	}
	if openedAt != nil {
		opened := openedAt.AddDate(0, 0, EffectiveOpenedShelfLifeDays(openedShelfLifeDays))
		if expiry == nil || opened.Before(*expiry) {
			expiry = &opened
		}
	}
	return expiry
}

// RemainingDays 计算剩余自然日（今天到期=0，明天=1，昨天=-1）。
func (c *FoodCalculator) RemainingDays(expiryDate *time.Time) int {
	if expiryDate == nil {
		return 365 * 10
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	expiry := time.Date(expiryDate.Year(), expiryDate.Month(), expiryDate.Day(), 0, 0, 0, 0, expiryDate.Location())
	return int(expiry.Sub(today).Hours() / 24)
}

// ComputeFreshness 计算新鲜度状态（已消耗优先返回 consumed）。
func (c *FoodCalculator) ComputeFreshness(status string, expiryDate *time.Time) string {
	if status == constants.FreshnessConsumed {
		return constants.FreshnessConsumed
	}
	days := c.RemainingDays(expiryDate)
	switch {
	case days < 0:
		return constants.FreshnessExpired
	case days <= constants.ExpiringThresholdDays:
		return constants.FreshnessExpiring
	default:
		return constants.FreshnessFresh
	}
}
