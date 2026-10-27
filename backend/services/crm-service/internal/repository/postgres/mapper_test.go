package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
	"github.com/Will413028/hawkr/crm-service/internal/repository/postgres/gen"
)

func TestToDomainCustomer(t *testing.T) {
	now := time.Now()
	row := gen.CrmCustomer{
		ID:           1,
		StoreID:      100,
		BuyerID:      200,
		CustomerType: "blocklist",
		TotalSpent:   50000,
		OrderCount:   10,
		Nickname:     "小明",
		Phone:        "0912345678",
		CreatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	}

	c := toDomainCustomer(row)

	if c.ID != 1 {
		t.Errorf("ID: got %d", c.ID)
	}
	if c.StoreID != 100 {
		t.Errorf("StoreID: got %d", c.StoreID)
	}
	if c.BuyerID != 200 {
		t.Errorf("BuyerID: got %d", c.BuyerID)
	}
	if c.CustomerType != domain.CustomerTypeBlocklist {
		t.Errorf("CustomerType: got %v", c.CustomerType)
	}
	if c.TotalSpent != 50000 {
		t.Errorf("TotalSpent: got %d", c.TotalSpent)
	}
	if c.OrderCount != 10 {
		t.Errorf("OrderCount: got %d", c.OrderCount)
	}
	if c.Nickname != "小明" {
		t.Errorf("Nickname: got %s", c.Nickname)
	}
	if c.Phone != "0912345678" {
		t.Errorf("Phone: got %s", c.Phone)
	}
	if !c.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", c.CreatedAt, now)
	}
	if !c.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt: got %v, want %v", c.UpdatedAt, now)
	}
}

func TestToDomainTag(t *testing.T) {
	now := time.Now()
	row := gen.CrmTag{
		ID:        5,
		StoreID:   100,
		Name:      "VIP",
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}

	tag := toDomainTag(row)

	if tag.ID != 5 {
		t.Errorf("ID: got %d", tag.ID)
	}
	if tag.Name != "VIP" {
		t.Errorf("Name: got %s", tag.Name)
	}
	if !tag.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", tag.CreatedAt, now)
	}
	if !tag.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt: got %v, want %v", tag.UpdatedAt, now)
	}
}
