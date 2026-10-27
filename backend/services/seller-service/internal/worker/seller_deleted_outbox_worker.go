// seller_deleted_outbox_worker.go: seller-service's seller-lifecycle outbox
// worker, built on github.com/Will413028/hawkr/go-pkg/outbox. This file owns
// everything specific to seller_deleted_outbox — dispatching seller.deleted /
// seller.onboarded / seller.suspended / seller.unsuspended /
// seller.membership_changed by event_type, and the account-service role sync
// (grant on onboard, removal on delete) — while claim preemption, retry
// backoff, and terminal-failure handling live in pkg/outbox.Worker[T].
//
// Publish and role sync are two sub-steps of ONE outbox row, and they have
// DELIBERATELY OPPOSITE TIME SEMANTICS (issue #3189):
//
//   - Publish is edge-triggered. The MQ event states what happened, so its
//     payload is the enqueue-time snapshot — re-resolving the surrogate at
//     dispatch would race a later re-onboard and rewrite history.
//   - Role sync is level-triggered on the way in and fenced on the way out.
//     The grant re-reads the account's CURRENT stint, so a retry that lands
//     minutes late grants for whichever onboarding is real now. The removal
//     names the stint it is closing, and account-service applies it only to a
//     grant that came from that same stint.
//
// Conflating the two is what this file used to get wrong: the removal was
// written in the publish's shape — an unconditional command derived from a past
// event — so a deletion queued at T and processed at T+51s stripped a role the
// account had legitimately regained in between, with no path back.
//
// Publish and removal are tracked by EventPublishedAt/RoleRemovedAt on the row
// itself — these are RESUME CHECKPOINTS, unrelated to pkg/outbox's own
// claim/attempt state machine. A retry after a partial completion (e.g. publish
// succeeded, then the process crashed before role removal) must not republish;
// Process checks these fields itself, exactly like the pre-migration processRow
// did. The grant has no checkpoint on purpose: it is idempotent and
// level-triggered, so re-running it costs one RPC and can never be wrong.
//
// Backoff schedule (capped), unchanged from before this file existed:
//
//	attempt=1 → +2m   attempt=2 → +4m
//	attempt=3 → +8m   attempt=4 → +16m
//	attempt=5 → terminal failure (MarkFailed, no further reschedule)
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/logger"
	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/go-pkg/outbox"
	"github.com/Will413028/hawkr/go-pkg/rolestate"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
	"go.uber.org/zap"
)

const (
	// sdoMaxAttempts is the cap; once attempt_count reaches this value the row
	// is terminally failed. The first call is attempt 1.
	sdoMaxAttempts = 5
	// sdoBaseBackoff is the unit interval; total delay = 2^attempt * base.
	sdoBaseBackoff = 1 * time.Minute
	// sdoMaxBackoff caps the per-retry sleep regardless of attempt count.
	sdoMaxBackoff = 1 * time.Hour
	// sdoFetchBatch limits how many outbox rows we pull per RunOnce pass.
	sdoFetchBatch = 50
	// sdoRoleRemovalTimeout bounds the account-service RemoveSellerRole call.
	// Without it the call would inherit Process's ctx unbounded — if
	// account-service hangs, this goroutine never returns and the whole
	// outbox drain stalls forever.
	sdoRoleRemovalTimeout = 10 * time.Second
	// sdoRoleGrantTimeout bounds the account-service BindSeller call, same
	// rationale as sdoRoleRemovalTimeout.
	sdoRoleGrantTimeout = 10 * time.Second
	// sdoClaimLease is how far into the future a successful claim pushes
	// next_attempt_at (issue #2294; docs/開發規範/多副本排程搶佔規範.md §三).
	// Worst case for one row is one MQ publish (ms) + HasActiveBuyer (3s) +
	// RemoveSellerRole (sdoRoleRemovalTimeout, 10s) ≈ 13s; 3m leaves an order
	// of magnitude of headroom.
	sdoClaimLease = 3 * time.Minute
)

// sellerLifecyclePublisher is the seam for publishing seller lifecycle events.
// *pkgmq.Publisher satisfies this interface.
type sellerLifecyclePublisher interface {
	PublishSellerDeleted(ctx context.Context, event *pkgmq.SellerDeletedEvent) error
	PublishSellerOnboarded(ctx context.Context, event *pkgmq.SellerOnboardedEvent) error
	PublishSellerSuspended(ctx context.Context, event *pkgmq.SellerSuspendedEvent) error
	PublishSellerUnsuspended(ctx context.Context, event *pkgmq.SellerUnsuspendedEvent) error
	PublishSellerMembershipChanged(ctx context.Context, event *pkgmq.SellerMembershipChangedEvent) error
}

// sellerRoleRemover is the seam for retrying account-service role removal.
// usecase.AccountRoleRemover (via *account.Client) satisfies this structurally.
type sellerRoleRemover interface {
	RemoveSellerRole(ctx context.Context, accountID int64, other rolestate.OtherRoleState, storeID int64) error
}

// sellerRoleGranter is the seam for the onboarding grant.
// usecase.AccountRoleGranter (via *account.Client) satisfies this structurally.
type sellerRoleGranter interface {
	BindSellerRole(ctx context.Context, accountID, storeID int64) error
}

// currentStintLookup resolves the account's current live stint at APPLY time.
// domain.SellerRepository satisfies this structurally.
//
// It is what makes the grant level-triggered: the outbox row says "this account
// onboarded", and this lookup answers "and here is the onboarding that is real
// right now". A row whose account has since been deleted resolves to NotFound
// and grants nothing.
type currentStintLookup interface {
	GetByAccountID(ctx context.Context, accountID int64) (*domain.Seller, error)
}

// SellerDeletedOutboxWorker drains pending seller lifecycle outbox rows and
// runs their side effects. RunOnce is the single entry point called by the
// scheduler on every tick.
type SellerDeletedOutboxWorker struct {
	inner *outbox.Worker[*domain.SellerDeletedOutbox]
	// publisherMissing is set once at construction and never changes
	// afterward. pkg/outbox always claims before invoking ProcessFunc, so
	// gating here avoids claiming a row we already know we can't publish for.
	publisherMissing bool
}

// NewSellerDeletedOutboxWorker constructs a worker. If pub is nil (MQ
// unavailable at startup), RunOnce is a no-op until the process restarts with
// a healthy MQ connection. metrics must not be nil.
func NewSellerDeletedOutboxWorker(
	repo domain.SellerDeletedOutboxRepository,
	pub sellerLifecyclePublisher,
	remover sellerRoleRemover,
	granter sellerRoleGranter,
	sellers currentStintLookup,
	buyers usecase.BuyerClient,
	metrics *outbox.Metrics,
) *SellerDeletedOutboxWorker {
	if metrics == nil {
		panic("worker.NewSellerDeletedOutboxWorker: metrics must not be nil")
	}

	process := func(ctx context.Context, row *domain.SellerDeletedOutbox) (any, error) {
		if row.EventPublishedAt == nil {
			if err := publishSellerLifecycleRow(ctx, pub, row); err != nil {
				return nil, err
			}
			now := time.Now()
			if err := repo.MarkEventPublished(ctx, row.ID, now); err != nil {
				return nil, fmt.Errorf("mark_event_published: %w", err)
			}
			row.EventPublishedAt = &now
		}

		// Role sync. The event_type allowlists are explicit and mirror
		// publishSellerLifecycleRow's dispatch below: suspension and membership
		// rows touch no role at all, and anything unrecognised falls through
		// without touching one either — a seller's role is never stripped or
		// handed out as a side effect of an event this file does not model.
		switch {
		case (row.EventType == "" || row.EventType == domain.SellerLifecycleEventDeleted) && row.RoleRemovedAt == nil:
			// RemoveSellerRole is normally idempotent here: the primary
			// synchronous call in DeleteSellerAccountUseCase.Execute already
			// ran and already made the real cascade decision. But this retry
			// can also be the FIRST successful call, if the process crashed
			// between the outbox commit and that primary call succeeding — so
			// we query buyer state ourselves rather than assume UNKNOWN; a
			// query failure still fail-closes to UNKNOWN (rolestate.Resolve).
			hasBuyer, buyerErr := buyers.HasActiveBuyer(ctx, row.AccountID)
			if buyerErr != nil {
				logger.Warn(ctx, "seller-deleted-outbox: HasActiveBuyer lookup failed; passing UNKNOWN (fail-closed)",
					zap.Int64("accountID", row.AccountID), zap.Error(buyerErr))
			}
			other := rolestate.Resolve(hasBuyer, buyerErr)
			// The stint this row closed, snapshotted at enqueue time — and here
			// the snapshot is exactly right, unlike for the grant below: this
			// removal was authorised by the closure of THAT stint and must not
			// silently widen into "remove the seller role, whatever justifies
			// it now". Legacy rows carry no payload and resolve to 0, i.e. no
			// attribution, which account-service treats as it did before the
			// fence existed.
			storeID := lifecycleStoreID(row)
			roleCtx, cancel := context.WithTimeout(ctx, sdoRoleRemovalTimeout)
			err := remover.RemoveSellerRole(roleCtx, row.AccountID, other, storeID)
			cancel()
			if err != nil {
				return nil, fmt.Errorf("remove_seller_role: %w", err)
			}
			now := time.Now()
			if err := repo.MarkRoleRemoved(ctx, row.ID, now); err != nil {
				return nil, fmt.Errorf("mark_role_removed: %w", err)
			}
			row.RoleRemovedAt = &now

		case row.EventType == domain.SellerLifecycleEventOnboarded:
			// The grant is the reason "store but no role" is no longer a state
			// this system can rest in: ApplySeller's synchronous BindSeller is
			// an optimisation for latency, and THIS is the guarantee.
			//
			// Level-triggered, so the row's payload surrogate is deliberately
			// NOT used: a retry arriving after the account deleted and
			// re-onboarded must grant for the stint that exists now, not
			// re-point the grant at a retired one — which would re-arm the very
			// revocation the fence rejects. No current stint means the account
			// has no live storefront, and granting would manufacture a role
			// nothing justifies.
			seller, err := sellers.GetByAccountID(ctx, row.AccountID)
			if err != nil {
				if pkgerrors.IsNotFound(err) {
					logger.Info(ctx, "seller-onboarded-outbox: no current stint at apply time; skipping role grant",
						zap.Int64("accountID", row.AccountID), zap.Int64("id", row.ID))
					return nil, nil
				}
				return nil, fmt.Errorf("lookup_current_stint: %w", err)
			}
			roleCtx, cancel := context.WithTimeout(ctx, sdoRoleGrantTimeout)
			err = granter.BindSellerRole(roleCtx, row.AccountID, seller.StoreID)
			cancel()
			if err != nil {
				return nil, fmt.Errorf("bind_seller_role: %w", err)
			}
		}

		return nil, nil
	}

	hooks := outbox.Hooks[*domain.SellerDeletedOutbox]{
		OnAttemptFailed: func(ctx context.Context, row *domain.SellerDeletedOutbox, cause string, err error) {
			logger.Warn(ctx, "seller-deleted-outbox: attempt failed",
				zap.Int64("id", row.ID), zap.Int64("accountID", row.AccountID),
				zap.String("cause", cause), zap.Error(err))
			metrics.AttemptFailedTotal(cause).Inc()
		},
		OnTerminalFailed: func(ctx context.Context, row *domain.SellerDeletedOutbox, newAttempt int, lastErr string, markFailedErr error) {
			if markFailedErr != nil {
				logger.Warn(ctx, "seller-deleted-outbox: MarkFailed error (degraded)",
					zap.Int64("id", row.ID), zap.Int64("accountID", row.AccountID),
					zap.Int("attempt", newAttempt), zap.String("lastErr", lastErr), zap.Error(markFailedErr))
				return
			}
			logger.Warn(ctx, "seller-deleted-outbox: row exhausted all retries → failed",
				zap.Int64("id", row.ID), zap.Int64("accountID", row.AccountID),
				zap.Int("attempt", newAttempt), zap.String("lastErr", lastErr))
			metrics.FailedTotal().Inc()
		},
	}

	cfg := outbox.Config{
		MaxAttempts: sdoMaxAttempts,
		BaseBackoff: sdoBaseBackoff,
		MaxBackoff:  sdoMaxBackoff,
		FetchBatch:  sdoFetchBatch,
		ClaimLease:  sdoClaimLease,
	}

	return &SellerDeletedOutboxWorker{
		inner:            outbox.New[*domain.SellerDeletedOutbox](repo, process, cfg, hooks),
		publisherMissing: pub == nil,
	}
}

// RunOnce processes one batch of pending+due outbox rows and returns. The
// scheduler invokes RunOnce on every tick.
func (w *SellerDeletedOutboxWorker) RunOnce(ctx context.Context) error {
	if w.publisherMissing {
		logger.Warn(ctx, "seller-deleted-outbox: publisher unavailable, skipping tick")
		return nil
	}
	return w.inner.RunOnce(ctx)
}

// lifecycleStoreID returns the stint surrogate snapshotted in the row's
// payload, or 0 when the row predates the payload (legacy seller.deleted rows,
// which carry an empty event_type or a NULL payload).
//
// It deliberately does NOT fall back to row.AccountID the way
// publishSellerLifecycleRow does for the wire event. That fallback exists to
// keep a legacy event's store field populated with the pre-surrogate identity;
// reusing it as an attribution would assert that an account id is a stint id
// and could match — or fail to match — a real grant by coincidence. 0 means
// "unattributed", which is the honest answer for a row that never recorded one.
func lifecycleStoreID(row *domain.SellerDeletedOutbox) int64 {
	if len(row.Payload) == 0 {
		return 0
	}
	var p domain.SellerLifecyclePayload
	if err := json.Unmarshal(row.Payload, &p); err != nil {
		return 0
	}
	return p.StoreID
}

func publishSellerLifecycleRow(ctx context.Context, pub sellerLifecyclePublisher, row *domain.SellerDeletedOutbox) error {
	switch row.EventType {
	case "", domain.SellerLifecycleEventDeleted, domain.SellerLifecycleEventOnboarded:
		var p domain.SellerLifecyclePayload
		if len(row.Payload) > 0 {
			if err := json.Unmarshal(row.Payload, &p); err != nil {
				return fmt.Errorf("unmarshal payload: %w", err)
			}
		}
		if p.StoreID == 0 {
			p.StoreID = row.AccountID
		}
		if row.EventType == domain.SellerLifecycleEventOnboarded {
			return pub.PublishSellerOnboarded(ctx, &pkgmq.SellerOnboardedEvent{
				BaseEvent: pkgmq.NewBaseEvent(pkgmq.EventTypeSellerOnboarded, "seller-service"),
				AccountID: row.AccountID,
				StoreID:   p.StoreID,
			})
		}
		return pub.PublishSellerDeleted(ctx, &pkgmq.SellerDeletedEvent{
			BaseEvent: pkgmq.NewBaseEvent(pkgmq.EventTypeSellerDeleted, "seller-service"),
			AccountID: row.AccountID,
			StoreID:   p.StoreID,
		})
	case domain.SellerLifecycleEventSuspended:
		var p domain.SellerSuspensionOutboxPayload
		if err := json.Unmarshal(row.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}
		return pub.PublishSellerSuspended(ctx, &pkgmq.SellerSuspendedEvent{
			BaseEvent:    pkgmq.NewBaseEvent(pkgmq.EventTypeSellerSuspended, "seller-service"),
			SuspensionID: p.SuspensionID,
			AccountID:    p.AccountID,
			ReasonCode:   p.ReasonCode,
			ReasonLabel:  domain.SellerSuspensionReasonLabel(p.ReasonCode),
			StoreID:      p.StoreID,
		})
	case domain.SellerLifecycleEventUnsuspended:
		var p domain.SellerSuspensionOutboxPayload
		if err := json.Unmarshal(row.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}
		return pub.PublishSellerUnsuspended(ctx, &pkgmq.SellerUnsuspendedEvent{
			BaseEvent:    pkgmq.NewBaseEvent(pkgmq.EventTypeSellerUnsuspended, "seller-service"),
			SuspensionID: p.SuspensionID,
			AccountID:    p.AccountID,
			StoreID:      p.StoreID,
		})
	case domain.SellerLifecycleEventMembershipChanged:
		var p domain.SellerMembershipOutboxPayload
		if err := json.Unmarshal(row.Payload, &p); err != nil {
			return fmt.Errorf("unmarshal payload: %w", err)
		}
		return pub.PublishSellerMembershipChanged(ctx, &pkgmq.SellerMembershipChangedEvent{
			BaseEvent:       pkgmq.NewBaseEvent(pkgmq.EventTypeSellerMembershipChanged, "seller-service"),
			StoreID:         strconv.FormatInt(p.StoreID, 10),
			PlanCode:        p.PlanCode,
			TierRank:        p.TierRank,
			MonthlyOrderCap: p.MonthlyOrderCap,
			EffectiveAt:     p.EffectiveAt.UTC().Format(time.RFC3339),
		})
	default:
		return fmt.Errorf("unknown outbox event_type %q", row.EventType)
	}
}
