package util

import (
	"testing"
	"time"

	"github.com/blueship581/cyfreshfood/internal/constants"
)

func TestFoodCalculator_RemainingDays(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	yesterday := time.Now().AddDate(0, 0, -1)
	calc := NewFoodCalculator()
	tests := []struct {
		name       string
		expiry     *time.Time
		wantRemain int
	}{
		{name: "tomorrow", expiry: &tomorrow, wantRemain: 1},
		{name: "yesterday", expiry: &yesterday, wantRemain: -1},
		{name: "nil", expiry: nil, wantRemain: 3650},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calc.RemainingDays(tt.expiry); got != tt.wantRemain {
				t.Fatalf("RemainingDays() = %d, want %d", got, tt.wantRemain)
			}
		})
	}
}

func TestFoodCalculator_ComputeFreshness(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	tenDays := time.Now().AddDate(0, 0, 10)
	yesterday := time.Now().AddDate(0, 0, -1)
	calc := NewFoodCalculator()
	tests := []struct {
		name   string
		status string
		expiry *time.Time
		want   string
	}{
		{name: "consumed wins", status: "consumed", expiry: &tenDays, want: "consumed"},
		{name: "expiring within 3 days", status: "fresh", expiry: &tomorrow, want: "expiring"},
		{name: "fresh far away", status: "fresh", expiry: &tenDays, want: "fresh"},
		{name: "expired", status: "fresh", expiry: &yesterday, want: "expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calc.ComputeFreshness(tt.status, tt.expiry); got != tt.want {
				t.Fatalf("ComputeFreshness() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFoodCalculator_CalculateExpiryDate(t *testing.T) {
	calc := NewFoodCalculator()
	produced := time.Now().AddDate(0, 0, -2)
	opened := time.Now()
	intPtr := func(v int) *int { return &v }
	tests := []struct {
		name           string
		productionDate *time.Time
		shelfLifeDays  int
		openedAt       *time.Time
		openedDays     *int
		wantRemainDays int // 到期日相对今天的剩余天数（按自然日容差 1 天）
		wantNil        bool
	}{
		{name: "sealed uses shelf life", productionDate: &produced, shelfLifeDays: 10, wantRemainDays: 8},
		{name: "opened defaults 7 days", openedAt: &opened, wantRemainDays: 7},
		{name: "opened zero value defaults 7 days", openedAt: &opened, openedDays: intPtr(0), wantRemainDays: 7},
		{name: "opened custom days", openedAt: &opened, openedDays: intPtr(3), wantRemainDays: 3},
		{name: "opened limit earlier than shelf life", productionDate: &produced, shelfLifeDays: 30, openedAt: &opened, openedDays: intPtr(2), wantRemainDays: 2},
		{name: "shelf life earlier than opened limit", productionDate: &produced, shelfLifeDays: 3, openedAt: &opened, openedDays: intPtr(20), wantRemainDays: 1},
		{name: "nothing known is nil", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.CalculateExpiryDate(tt.productionDate, tt.shelfLifeDays, tt.openedAt, tt.openedDays)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expiry = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expiry is nil, want a date")
			}
			remain := calc.RemainingDays(got)
			if remain != tt.wantRemainDays {
				t.Fatalf("remaining days = %d, want %d", remain, tt.wantRemainDays)
			}
		})
	}
}

func TestEffectiveOpenedShelfLifeDays(t *testing.T) {
	v := 12
	zero := 0
	tests := []struct {
		name string
		in   *int
		want int
	}{
		{name: "nil defaults 7", in: nil, want: constants.DefaultOpenedShelfLifeDays},
		{name: "zero defaults 7", in: &zero, want: constants.DefaultOpenedShelfLifeDays},
		{name: "custom value", in: &v, want: 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveOpenedShelfLifeDays(tt.in); got != tt.want {
				t.Fatalf("EffectiveOpenedShelfLifeDays() = %d, want %d", got, tt.want)
			}
		})
	}
}
