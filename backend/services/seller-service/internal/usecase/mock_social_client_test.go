package usecase

import "context"

type mockSocialClient struct {
	HasPublishedPostFn func(ctx context.Context, storeID int64) (bool, error)
}

func (m *mockSocialClient) HasPublishedPost(ctx context.Context, storeID int64) (bool, error) {
	return m.HasPublishedPostFn(ctx, storeID)
}
