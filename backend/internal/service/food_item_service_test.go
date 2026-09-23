package service

import (
	"context"
	"testing"
	"time"

	"github.com/blueship581/cyfreshfood/internal/constants"
	"github.com/blueship581/cyfreshfood/internal/model"
	"github.com/blueship581/cyfreshfood/internal/repository"
	"github.com/blueship581/cyfreshfood/internal/util"
	"gorm.io/gorm"
)

func seedFood(t *testing.T, db *gorm.DB, familyID, creatorID uint, quantity float64) *model.FoodItem {
	t.Helper()
	expiry := time.Now().AddDate(0, 0, 5)
	item := &model.FoodItem{
		FamilyID: familyID, Name: "测试食品", Category: constants.FoodCategoryDairy,
		Quantity: quantity, Unit: "盒", ShelfLifeDays: 5, StorageLocation: constants.StorageFridge,
		Status: constants.FreshnessFresh, CreatorID: creatorID, ExpiryDate: &expiry,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("seed food: %v", err)
	}
	return item
}

func TestFoodItemService_Consume(t *testing.T) {
	db := newTestDB(t)
	groupRepo := repository.NewFamilyGroupRepository(db)
	memberRepo := repository.NewFamilyMemberRepository(db)
	foodRepo := repository.NewFoodItemRepository(db)
	consumeRepo := repository.NewConsumptionRecordRepository(db)
	familySvc := NewFamilyGroupService(groupRepo, memberRepo, testLogger())
	svc := NewFoodItemService(foodRepo, consumeRepo, familySvc, util.NewFoodCalculator(), testLogger())
	ctx := context.Background()

	group, err := familySvc.Create(ctx, 1, "测试家庭")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	item := seedFood(t, db, group.ID, 1, 3)

	record, err := svc.Consume(ctx, 1, item.ID, 1.5, time.Now())
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if record.Quantity != 1.5 {
		t.Fatalf("record quantity = %v, want 1.5", record.Quantity)
	}

	got, err := foodRepo.FindByID(item.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.Quantity != 1.5 {
		t.Fatalf("remaining quantity = %v, want 1.5", got.Quantity)
	}

	// 全部消耗后状态流转为 consumed。
	if _, err := svc.Consume(ctx, 1, item.ID, 1.5, time.Now()); err != nil {
		t.Fatalf("second Consume() error = %v", err)
	}
	got, _ = foodRepo.FindByID(item.ID)
	if got.Status != constants.FreshnessConsumed {
		t.Fatalf("status = %s, want %s", got.Status, constants.FreshnessConsumed)
	}

	// 超量消耗应失败。
	if _, err := svc.Consume(ctx, 1, item.ID, 0.1, time.Now()); err == nil {
		t.Fatal("expected error for consuming consumed/empty food")
	}
}

func TestFoodItemService_OpenedAfterDays(t *testing.T) {
	db := newTestDB(t)
	groupRepo := repository.NewFamilyGroupRepository(db)
	memberRepo := repository.NewFamilyMemberRepository(db)
	foodRepo := repository.NewFoodItemRepository(db)
	consumeRepo := repository.NewConsumptionRecordRepository(db)
	familySvc := NewFamilyGroupService(groupRepo, memberRepo, testLogger())
	svc := NewFoodItemService(foodRepo, consumeRepo, familySvc, util.NewFoodCalculator(), testLogger())
	ctx := context.Background()

	group, err := familySvc.Create(ctx, 1, "测试家庭")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// 未开封：保质期 30 天。
	item, err := svc.Create(ctx, 1, CreateFoodInput{
		FamilyID: group.ID, Name: "冷冻牛奶", Category: constants.FoodCategoryDairy,
		ShelfLifeDays: 30, Quantity: 2, Unit: "盒", StorageLocation: constants.StorageFreezer,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if r := util.NewFoodCalculator().RemainingDays(item.ExpiryDate); r != 30 {
		t.Fatalf("unopened remaining = %d, want 30", r)
	}

	// 开封但天数留空：所有类别统一按 7 天，到期日取较早者。
	openedAt := time.Now()
	item, err = svc.Update(ctx, 1, item.ID, CreateFoodInput{
		FamilyID: group.ID, Name: item.Name, Category: item.Category,
		ShelfLifeDays: 30, Quantity: 2, Unit: "盒", StorageLocation: constants.StorageFreezer,
		OpenedAt: &openedAt,
	})
	if err != nil {
		t.Fatalf("Update(open) error = %v", err)
	}
	if item.OpenedAfterDays != nil {
		t.Fatalf("OpenedAfterDays = %v, want nil（留空按 7 天）", *item.OpenedAfterDays)
	}
	if r := util.NewFoodCalculator().RemainingDays(item.ExpiryDate); r != 7 {
		t.Fatalf("opened default remaining = %d, want 7", r)
	}

	// 开封天数改为 3 天。
	days3 := 3
	item, err = svc.Update(ctx, 1, item.ID, CreateFoodInput{
		FamilyID: group.ID, Name: item.Name, Category: item.Category,
		ShelfLifeDays: 30, Quantity: 2, Unit: "盒", StorageLocation: constants.StorageFreezer,
		OpenedAt: &openedAt, OpenedAfterDays: &days3,
	})
	if err != nil {
		t.Fatalf("Update(3 days) error = %v", err)
	}
	if item.OpenedAfterDays == nil || *item.OpenedAfterDays != 3 {
		t.Fatalf("OpenedAfterDays = %v, want 3", item.OpenedAfterDays)
	}
	if r := util.NewFoodCalculator().RemainingDays(item.ExpiryDate); r != 3 {
		t.Fatalf("opened 3 days remaining = %d, want 3", r)
	}

	// 取消开封（时间与天数均留空）：回退为原保质期 30 天。
	item, err = svc.Update(ctx, 1, item.ID, CreateFoodInput{
		FamilyID: group.ID, Name: item.Name, Category: item.Category,
		ShelfLifeDays: 30, Quantity: 2, Unit: "盒", StorageLocation: constants.StorageFreezer,
	})
	if err != nil {
		t.Fatalf("Update(unopened) error = %v", err)
	}
	if item.OpenedAt != nil || item.OpenedAfterDays != nil {
		t.Fatalf("opened fields should be cleared, got %+v", item)
	}
	if r := util.NewFoodCalculator().RemainingDays(item.ExpiryDate); r != 30 {
		t.Fatalf("unopened remaining = %d, want 30", r)
	}

	// 先消耗一份，再用越界天数更新：必须保留原记录（余量/开封信息/到期日）与消耗历史不变。
	if _, err := svc.Consume(ctx, 1, item.ID, 1, time.Now()); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	before, _ := foodRepo.FindByID(item.ID)
	beforeExpiry := before.ExpiryDate
	beforeOpenedAt := before.OpenedAt
	beforeDays := before.OpenedAfterDays
	beforeQty := before.Quantity
	for _, bad := range []int{0, 31, -5} {
		v := bad
		_, err := svc.Update(ctx, 1, item.ID, CreateFoodInput{
			FamilyID: group.ID, Name: item.Name, Category: item.Category,
			ShelfLifeDays: 30, Quantity: 2, Unit: "盒", StorageLocation: constants.StorageFreezer,
			OpenedAt: &openedAt, OpenedAfterDays: &v,
		})
		if err == nil {
			t.Fatalf("Update(opened_after_days=%d) expected error", bad)
		}
	}
	after, _ := foodRepo.FindByID(item.ID)
	if after.Quantity != beforeQty {
		t.Fatalf("quantity changed after rejected update: %v -> %v", beforeQty, after.Quantity)
	}
	if !equalTimePtr(after.ExpiryDate, beforeExpiry) {
		t.Fatalf("expiry changed after rejected update: %v -> %v", beforeExpiry, after.ExpiryDate)
	}
	if !equalTimePtr(after.OpenedAt, beforeOpenedAt) {
		t.Fatalf("opened_at changed after rejected update")
	}
	if !equalIntPtr(after.OpenedAfterDays, beforeDays) {
		t.Fatalf("opened_after_days changed after rejected update")
	}
	records, err := consumeRepo.ListByFood(item.ID)
	if err != nil {
		t.Fatalf("ListByFood() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("consumption history changed: got %d records, want 1", len(records))
	}
}

func equalTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
