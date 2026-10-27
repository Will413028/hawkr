package usecase

import "context"

// ProductBasicInfo holds the product fields needed by social-service.
type ProductBasicInfo struct {
	ID       string
	Name     string
	ImageURL string
	Price    string
}

// ProductClient defines the interface for product-service operations.
type ProductClient interface {
	GetProductsByIDs(ctx context.Context, productIDs []string) (map[string]*ProductBasicInfo, error)
}
