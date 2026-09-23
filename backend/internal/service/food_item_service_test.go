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

func intPtr(v int) *int { return &v }

func TestFoodItemService_OpenedShelfLifeDays(t *testing.T) {
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

	// 创建时填写 31 天应被拒绝，且不产生任何记录。
	badInput := CreateFoodInput{
		FamilyID: group.ID, Name: "超范围牛奶", Category: constants.FoodCategoryDairy,
		Quantity: 2, Unit: "盒", ShelfLifeDays: 10, OpenedShelfLifeDays: intPtr(31),
	}
	if _, err := svc.Create(ctx, 1, badInput); err == nil {
		t.Fatal("expected error for opened_shelf_life_days=31 on create")
	}
	if count, _ := foodRepo.CountByFamily(group.ID); count != 0 {
		t.Fatalf("food count = %d, want 0 after rejected create", count)
	}

	// 合法创建：开封 3 天，到期日应为开封日+3。
	openedAt := time.Now()
	input := CreateFoodInput{
		FamilyID: group.ID, Name: "鲜牛奶", Category: constants.FoodCategoryDairy,
		Quantity: 2, Unit: "盒", ShelfLifeDays: 10, StorageLocation: constants.StorageFridge,
		OpenedAt: &openedAt, OpenedShelfLifeDays: intPtr(3),
	}
	created, err := svc.Create(ctx, 1, input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.OpenedShelfLifeDays == nil || *created.OpenedShelfLifeDays != 3 {
		t.Fatalf("opened shelf life days = %v, want 3", created.OpenedShelfLifeDays)
	}
	if remain := util.NewFoodCalculator().RemainingDays(created.ExpiryDate); remain != 3 {
		t.Fatalf("remaining days = %d, want 3", remain)
	}

	// 产生一条消耗记录，随后越界编辑必须保留数量与历史。
	if _, err := svc.Consume(ctx, 1, created.ID, 0.5, time.Now()); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	before, _ := foodRepo.FindByID(created.ID)
	beforeExpiry := before.ExpiryDate
	badUpdate := input
	badUpdate.OpenedShelfLifeDays = intPtr(0) // 0 同样超出 1~30 范围
	if _, err := svc.Update(ctx, 1, created.ID, badUpdate); err == nil {
		t.Fatal("expected error for opened_shelf_life_days=0 on update")
	}
	after, _ := foodRepo.FindByID(created.ID)
	if after.Quantity != before.Quantity {
		t.Fatalf("quantity changed %v -> %v, record must be kept", before.Quantity, after.Quantity)
	}
	if after.OpenedShelfLifeDays == nil || *after.OpenedShelfLifeDays != 3 {
		t.Fatalf("opened days changed to %v, original record must be kept", after.OpenedShelfLifeDays)
	}
	if after.ExpiryDate == nil || !after.ExpiryDate.Equal(*beforeExpiry) {
		t.Fatal("expiry date changed after rejected update, original record must be kept")
	}
	records, err := consumeRepo.ListByFood(created.ID)
	if err != nil {
		t.Fatalf("ListByFood() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("consumption history count = %d, want 1 (must not change)", len(records))
	}

	// 留空（nil）编辑：开封时间仍存在时按默认 7 天重算到期日。
	blankDays := input
	blankDays.OpenedShelfLifeDays = nil
	updated, err := svc.Update(ctx, 1, created.ID, blankDays)
	if err != nil {
		t.Fatalf("Update() blank days error = %v", err)
	}
	if updated.OpenedShelfLifeDays != nil {
		t.Fatalf("opened days = %v, want nil (default 7 at calc time)", updated.OpenedShelfLifeDays)
	}
	if remain := util.NewFoodCalculator().RemainingDays(updated.ExpiryDate); remain != constants.DefaultOpenedShelfLifeDays {
		t.Fatalf("remaining days = %d, want default %d", remain, constants.DefaultOpenedShelfLifeDays)
	}
}
