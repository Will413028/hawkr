package usecase

import "context"

// ProductInfo holds the minimal fields needed by seller-service.
type ProductInfo struct {
	ID       int64
	SellerID int64
}

// ProductClient defines the interface for fetching product information from product-service.
type ProductClient interface {
	GetProductsByIDs(ctx context.Context, productIDs []int64) ([]*ProductInfo, error)
	// ListStoreIDsWithPublicProducts returns every store that has at least one
	// publicly listed product. An empty (non-nil) slice with a nil error means
	// "nobody has one" — a real answer; an error means "could not ask", and
	// callers must fail open rather than treat it as the empty set.
	ListStoreIDsWithPublicProducts(ctx context.Context) ([]int64, error)
}
