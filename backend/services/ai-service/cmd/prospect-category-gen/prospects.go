package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

type prospect struct {
	ID             int64
	DisplayName    string
	Category       string
	Region         string
	ProductSummary string
	Positioning    string
}

func loadProspects(ctx context.Context, dsn string, limit int) ([]prospect, error) {
	if dsn == "" {
		dsn = fmt.Sprintf("postgres://%s:%s@%s:5432/hawkr_seller",
			os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("INFRA_VM_IP"))
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connecting to seller db: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	q := `SELECT id, display_name, category, region, product_summary, positioning
	      FROM seller_prospects WHERE status = 'ACTIVE' ORDER BY id`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := conn.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []prospect
	for rows.Next() {
		var p prospect
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.Category, &p.Region, &p.ProductSummary, &p.Positioning); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
