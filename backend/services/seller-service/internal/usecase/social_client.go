package usecase

import "context"

type SocialClient interface {
	HasPublishedPost(ctx context.Context, storeID int64) (bool, error)
}
