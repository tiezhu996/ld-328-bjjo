package util

import (
	"testing"
	"time"
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

func TestFoodCalculator_CalculateExpiryDate_Opened(t *testing.T) {
	calc := NewFoodCalculator()
	today := time.Now()
	produced := today.AddDate(0, 0, -1) // 昨天生产
	opened := today.AddDate(0, 0, -2)   // 前天开封

	tests := []struct {
		name            string
		shelfLifeDays   int
		productionDate  *time.Time
		openedAt        *time.Time
		openedAfterDays *int
		wantRemain      int // 相对今天的剩余天数
		wantNil         bool
	}{
		{name: "unopened uses shelf life", shelfLifeDays: 10, productionDate: &produced, wantRemain: 9},
		{name: "opened defaults to 7 days", shelfLifeDays: 30, productionDate: &produced, openedAt: &opened, wantRemain: 5},
		{name: "opened custom days earlier than shelf life", shelfLifeDays: 30, productionDate: &produced, openedAt: &opened, openedAfterDays: intPtr(3), wantRemain: 1},
		{name: "opened custom days later keeps shelf life", shelfLifeDays: 2, productionDate: &produced, openedAt: &opened, openedAfterDays: intPtr(10), wantRemain: 1},
		{name: "opened without shelf life uses opened limit", openedAt: &opened, openedAfterDays: intPtr(4), wantRemain: 2},
		{name: "nothing known yields nil", wantNil: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.CalculateExpiryDate(tt.productionDate, tt.shelfLifeDays, tt.openedAt, tt.openedAfterDays)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expiry = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expiry = nil, want date")
			}
			if remain := calc.RemainingDays(got); remain != tt.wantRemain {
				t.Fatalf("remaining = %d, want %d (expiry=%v)", remain, tt.wantRemain, got)
			}
		})
	}
}

func TestResolveOpenedAfterDays(t *testing.T) {
	if got := ResolveOpenedAfterDays(nil); got != 7 {
		t.Fatalf("nil opened days = %d, want default 7", got)
	}
	zero := 0
	if got := ResolveOpenedAfterDays(&zero); got != 7 {
		t.Fatalf("zero opened days = %d, want default 7", got)
	}
	five := 5
	if got := ResolveOpenedAfterDays(&five); got != 5 {
		t.Fatalf("opened days = %d, want 5", got)
	}
}

func TestValidOpenedAfterDays(t *testing.T) {
	for _, d := range []int{1, 7, 30} {
		if !ValidOpenedAfterDays(d) {
			t.Fatalf("%d should be valid", d)
		}
	}
	for _, d := range []int{0, -1, 31, 99} {
		if ValidOpenedAfterDays(d) {
			t.Fatalf("%d should be invalid", d)
		}
	}
}

func intPtr(v int) *int { return &v }
