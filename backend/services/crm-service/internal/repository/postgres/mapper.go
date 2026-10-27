package postgres

import (
	"github.com/Will413028/hawkr/crm-service/internal/domain"
	"github.com/Will413028/hawkr/crm-service/internal/repository/postgres/gen"
)

// toDomainCustomer converts a gen.CrmCustomer to domain.CrmCustomer
func toDomainCustomer(row gen.CrmCustomer) *domain.CrmCustomer {
	c := &domain.CrmCustomer{
		ID:           row.ID,
		StoreID:      row.StoreID,
		BuyerID:      row.BuyerID,
		CustomerType: domain.CustomerType(row.CustomerType),
		TotalSpent:   row.TotalSpent,
		OrderCount:   int(row.OrderCount),
		Nickname:     row.Nickname,
		Phone:        row.Phone,
	}
	if row.CreatedAt.Valid {
		c.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		c.UpdatedAt = row.UpdatedAt.Time
	}
	return c
}

// toDomainTag converts a gen.CrmTag to domain.CrmTag
func toDomainTag(row gen.CrmTag) *domain.CrmTag {
	t := &domain.CrmTag{
		ID:      row.ID,
		StoreID: row.StoreID,
		Name:    row.Name,
	}
	if row.CreatedAt.Valid {
		t.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		t.UpdatedAt = row.UpdatedAt.Time
	}
	return t
}
