package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Will413028/hawkr/backend/internal/legacy/sellerserver"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

func readKYCHashKeys(e *config.Env) *seal.MAC {
	mac, err := parseKYCHashKeys(e.String("KYC_HASH_KEYS"))
	if err != nil {
		e.Fail("KYC 金鑰：%v", err)
	}
	return mac
}

func parseKYCHashKeys(mac string) (*seal.MAC, error) {
	if mac == "" {
		return nil, errors.New("KYC_HASH_KEYS 必須設定")
	}
	macs, err := seal.ParseMAC(mac)
	if err != nil {
		return nil, fmt.Errorf("KYC_HASH_KEYS：%w", err)
	}
	return macs, nil
}

func readGCIS(e *config.Env) *gcis.Client {
	client, err := gcis.New(gcis.Config{BaseURL: e.StringOr("GCIS_BASE_URL", ""), Timeout: e.Duration("GCIS_TIMEOUT", gcis.DefaultTimeout)})
	if err != nil {
		e.Fail("GCIS：%v", err)
	}
	return client
}

func newSeller(d *db.DB, bus *events.Bus, jc jobs.ReportingEnqueuer, id *identity.Module, med mediaPort, orders seller.Orders, products seller.ProductStores, kycMAC *seal.MAC, payments *payment.Module, registry *gcis.Client, log *slog.Logger) *seller.Module {
	return seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: jc, Identity: id, Media: med, Orders: orders, KYCMAC: kycMAC,
		ProductStores: products, Payment: payments, Registry: registry, Now: time.Now, Logger: log,
	})
}

type legacySellerRatings struct{ client storeRatings }

func (r legacySellerRatings) StoreRatingStats(ctx context.Context, storeIDs []int64) (map[int64]sellerserver.RatingStats, error) {
	return ratingStats[sellerserver.RatingStats](ctx, r.client, storeIDs)
}
