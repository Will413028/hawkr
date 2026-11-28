//go:build integration

package sellerfixture

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

type enqueuer struct{}

func (enqueuer) Enqueue(context.Context, river.JobArgs, *river.InsertOpts) error { return nil }

func (enqueuer) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

type orders struct{}

func (orders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type products struct{}

func (products) StoreIDs(context.Context, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return map[uuid.UUID]int64{}, nil
}

type registry struct{}

var storeCodeSequence atomic.Int64

func (registry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

func NewModule(d *db.DB, now func() time.Time) (*seller.Module, error) {
	return NewModuleWithEvents(d, events.NewBus(), now)
}

func NewModuleWithEvents(d *db.DB, bus *events.Bus, now func() time.Time) (*seller.Module, error) {
	return newModule(d, bus, now, func(at time.Time) (int64, error) {
		return int64(at.UTC().Year()%100)*100_000_000 + 99_999_999 - storeCodeSequence.Add(1), nil
	})
}

func NewModuleWithStoreCode(d *db.DB, now func() time.Time, storeCode func(time.Time) (int64, error)) (*seller.Module, error) {
	return newModule(d, events.NewBus(), now, storeCode)
}

func newModule(d *db.DB, bus *events.Bus, now func() time.Time, storeCode func(time.Time) (int64, error)) (*seller.Module, error) {
	if d == nil || now == nil {
		return nil, fmt.Errorf("sellerfixture: database and clock are required")
	}
	if storeCode == nil {
		return nil, fmt.Errorf("sellerfixture: store-code generator is required")
	}
	if bus == nil {
		bus = events.NewBus()
	}
	queue := enqueuer{}
	identityModule := identity.New(identity.Deps{DB: d, Events: bus, Now: now})
	payments := payment.New(payment.Deps{DB: d, Events: bus, Jobs: queue, Audit: adminaudit.New(d, now), Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	mac, err := seal.ParseMAC("fixture:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("s", 32))))
	if err != nil {
		return nil, err
	}
	return seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: identityModule, Media: media.NewFake(), Orders: orders{},
		KYCMAC: mac, Payment: payments, ProductStores: products{}, Registry: registry{}, Now: now,
		StoreCode: storeCode,
	}), nil
}

func ExposableStore(ctx context.Context, d *db.DB, accountID int64, now time.Time) (int64, error) {
	return ExposableStoreNamed(ctx, d, accountID, now, "測試店")
}

func NewAccount(ctx context.Context, d *db.DB, now time.Time) (int64, error) {
	bus := events.NewBus()
	module := identity.New(identity.Deps{DB: d, Events: bus, Now: func() time.Time { return now }})
	var accountID int64
	for range 2 {
		subject := uuid.NewV7().String()
		login, err := module.Login(ctx, identity.LoginInput{
			Identity: identity.ProviderIdentity{Provider: identity.ProviderGoogle, Subject: subject, Name: subject},
			Client:   identity.ClientApp,
		})
		if err != nil {
			return 0, err
		}
		accountID = login.AccountID
	}
	return accountID, nil
}

func ExposableStoreNamed(ctx context.Context, d *db.DB, accountID int64, now time.Time, name string) (int64, error) {
	clock := func() time.Time { return now }
	module, err := NewModule(d, clock)
	if err != nil {
		return 0, err
	}
	identityModule := identity.New(identity.Deps{DB: d, Events: events.NewBus(), Now: clock})
	_, ok, err := identityModule.ContactEmail(ctx, accountID)
	if err != nil {
		return 0, err
	}
	if !ok {
		if _, err := identityModule.SetSelfEnteredEmail(ctx, accountID, fmt.Sprintf("sellerfixture-%d@example.test", accountID)); err != nil {
			return 0, err
		}
	}
	created, err := module.Apply(ctx, seller.ApplyInput{
		AccountID: accountID, StoreName: name, ContactPhone: "0912345678", TownCode: "63000010", AddressDetail: "測試路一號",
		AcceptedAgreements: seller.CurrentApplyAcceptances(),
	})
	if err != nil {
		return 0, err
	}
	if created.StoreID == accountID || created.StoreCode == accountID || created.StoreID == created.StoreCode {
		return 0, fmt.Errorf("sellerfixture: generated identifiers alias for store %d and account %d", created.StoreID, accountID)
	}
	supportEmail := fmt.Sprintf("sellerfixture-support-%d@example.test", accountID)
	if _, err := module.UpdateProfile(ctx, seller.UpdateProfileInput{
		StoreID: created.StoreID, ExpectedVersion: created.Version, LogoToken: fmt.Sprintf("sellerfixture-logo-%d", accountID), UploaderID: accountID, SupportEmail: &supportEmail,
	}); err != nil {
		return 0, err
	}
	sellerType := seller.KYCIndividual
	productType := "蔬果"
	draft, err := module.SaveKYCDraft(ctx, seller.KYCDraftInput{AccountID: accountID, SellerType: &sellerType, ProductType: &productType})
	if err != nil {
		return 0, err
	}
	submitted, err := module.SubmitKYC(ctx, seller.SubmitKYCInput{
		AcceptedAgreements: seller.CurrentKYCAcceptances(), AccountID: accountID, ExpectedVersion: draft.Version,
	})
	if err != nil {
		return 0, err
	}
	if _, err := module.ApproveKYC(ctx, seller.ApproveKYCInput{
		ApplicationID: submitted.ID, ExpectedVersion: submitted.Version, AdminID: 1,
	}); err != nil {
		return 0, err
	}
	if err := d.InTx(ctx, func(txCtx context.Context) error {
		return module.OnPayoutAccountChanged(txCtx, payment.PayoutAccountChanged{
			OccurredAt: now, StoreID: created.StoreID, Provider: "fixture", TransfersActive: true,
		})
	}); err != nil {
		return 0, err
	}
	exposed, err := module.ExposableStores(ctx, []int64{created.StoreID})
	if err != nil {
		return 0, err
	}
	if len(exposed) != 1 || exposed[0].StoreID != created.StoreID {
		return 0, fmt.Errorf("sellerfixture: store %d was not exposable after seller operations", created.StoreID)
	}
	return created.StoreID, nil
}

func SetPayoutCapability(ctx context.Context, d *db.DB, storeID int64, now time.Time, active bool) error {
	module, err := NewModule(d, func() time.Time { return now })
	if err != nil {
		return err
	}
	return d.InTx(ctx, func(txCtx context.Context) error {
		return module.OnPayoutAccountChanged(txCtx, payment.PayoutAccountChanged{
			OccurredAt: now, StoreID: storeID, Provider: "fixture", TransfersActive: active,
		})
	})
}

func BlockBuyer(ctx context.Context, d *db.DB, storeID, buyerID int64, now time.Time) error {
	module, err := NewModule(d, func() time.Time { return now })
	if err != nil {
		return err
	}
	_, err = module.BlockBuyer(ctx, storeID, buyerID)
	return err
}
