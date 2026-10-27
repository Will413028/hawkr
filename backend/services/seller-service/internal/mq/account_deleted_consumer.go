package mq

import (
	"context"
	"fmt"

	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/zap"
)

// accountDeletedHandler releases the deleted account's seller-side media
// references (#1489): logo (SELLER_LOGO) and cover (SELLER_COVER) are both
// linked under ref_id=accountID, so once the account is gone those assets must
// be released to become GC-eligible. account.deleted fires only when an
// account's LAST role is removed (full teardown), so this is the correct point
// to drop seller media.
//
// A processing error is returned so the ResilientConsumer retries and
// eventually dead-letters rather than silently leaking references.
func accountDeletedHandler(mediaClient usecase.MediaClient) func(ctx context.Context, routingKey string, body []byte) error {
	return pkgmq.BindTypedHandler(zap.NewNop(), func(ctx context.Context, _ string, payload *pkgmq.AccountDeletedEvent) error {
		if payload.AccountID == 0 {
			return fmt.Errorf("account.deleted event missing accountId")
		}

		refIDs := []string{fmt.Sprintf("%d", payload.AccountID)}
		if err := mediaClient.ReleaseReferences(ctx, pkgmedia.RefTypeSellerLogo, refIDs, nil); err != nil {
			return fmt.Errorf("release seller logo references: %w", err)
		}
		if err := mediaClient.ReleaseReferences(ctx, pkgmedia.RefTypeSellerCover, refIDs, nil); err != nil {
			return fmt.Errorf("release seller cover references: %w", err)
		}
		return nil
	})
}
