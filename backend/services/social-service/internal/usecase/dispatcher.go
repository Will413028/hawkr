package usecase

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/social-service/internal/domain"
)

// FanoutHideDispatcher fulfils HIDE_TARGET across all 6 targetType values.
// In-process: POST / POST_COMMENT / SELLER_FORUM (social-service owned tables).
// Cross-service gRPC: REVIEW (review-service) / CHAT_MESSAGE (im-service).
// USER target is rejected — banning users uses BAN_USER action, not HIDE.
type FanoutHideDispatcher struct {
	postRepo     domain.PostRepository
	commentRepo  domain.CommentRepository
	forumRepo    domain.ForumRepository
	reviewClient ReviewModerationClient
	imClient     IMModerationClient
	postHider    *PostHider
}

func NewFanoutHideDispatcher(
	postRepo domain.PostRepository,
	commentRepo domain.CommentRepository,
	forumRepo domain.ForumRepository,
	reviewClient ReviewModerationClient,
	imClient IMModerationClient,
	postHider *PostHider,
) *FanoutHideDispatcher {
	return &FanoutHideDispatcher{
		postRepo:     postRepo,
		commentRepo:  commentRepo,
		forumRepo:    forumRepo,
		reviewClient: reviewClient,
		imClient:     imClient,
		postHider:    postHider,
	}
}

func (d *FanoutHideDispatcher) HideTarget(ctx context.Context, targetType string, targetID int64, reason string) error {
	switch targetType {
	case codemeta.ReportTargetTypePost.Value:
		return d.postHider.HidePost(ctx, targetID, reason)
	case codemeta.ReportTargetTypePostComment.Value:
		return d.commentRepo.HideComment(ctx, targetID, reason)
	case codemeta.ReportTargetTypeSellerForum.Value:
		return d.forumRepo.HideEntry(ctx, targetID, reason)
	case codemeta.ReportTargetTypeReview.Value:
		return d.reviewClient.HideReview(ctx, targetID, reason)
	case codemeta.ReportTargetTypeChatMessage.Value:
		return d.imClient.HideMessage(ctx, targetID, reason)
	case codemeta.ReportTargetTypeUser.Value:
		return fmt.Errorf("HIDE_TARGET not applicable to USER (use BAN_USER instead)")
	default:
		return fmt.Errorf("unsupported target_type: %s", targetType)
	}
}

// ResolveOwner implements TargetOwnerResolver for in-process target types.
// REVIEW/CHAT_MESSAGE return (0, nil) — cross-service owner lookup RPC not
// built yet (no client submits reports for those types currently).
func (d *FanoutHideDispatcher) ResolveOwner(ctx context.Context, targetType string, targetID int64) (int64, error) {
	switch targetType {
	case codemeta.ReportTargetTypePost.Value:
		post, err := d.postRepo.GetByID(ctx, targetID)
		if err != nil {
			return 0, err
		}
		return post.SellerID, nil
	case codemeta.ReportTargetTypePostComment.Value:
		c, err := d.commentRepo.GetByID(ctx, targetID)
		if err != nil {
			return 0, err
		}
		return c.UserID, nil
	case codemeta.ReportTargetTypeSellerForum.Value:
		f, err := d.forumRepo.GetByID(ctx, targetID)
		if err != nil {
			return 0, err
		}
		return f.UserID, nil
	case codemeta.ReportTargetTypeUser.Value:
		return targetID, nil
	default:
		return 0, nil
	}
}

// AccountBanDispatcher fulfils BAN_USER by resolving the report target to its
// login principal and calling account-service SuspendAccount.
//
// It deliberately does NOT write the account:blocked Redis key any more (#2053).
// That key is a derived cache of accounts.status, and it is now written by the
// same use case that writes the status — inside account-service, in one place,
// for every caller. Writing it from here made "does a suspension actually reach
// mobile clients?" a per-caller obligation that nothing enforced: the admin
// suspension entry point added in #2053 would have had to know to do it too, and
// this dispatcher's own write could fail AFTER the RPC had committed, leaving
// the status saying suspended while the cache said clean — which is the answer
// the gateway believes in steady state.
//
// The TTL that used to live here (one year) went with it. It was covering
// nothing: a suspended account cannot log in or refresh (both gate on
// Account.IsActive), so the only credential a block still has to stop is one
// issued before the suspension, bounded by the access token's own lifetime.
type AccountBanDispatcher struct {
	accountClient AccountClient
	sellerClient  SellerClient
	buyerClient   BuyerClient
}

func NewAccountBanDispatcher(accountClient AccountClient, sellerClient SellerClient, buyerClient BuyerClient) *AccountBanDispatcher {
	return &AccountBanDispatcher{accountClient: accountClient, sellerClient: sellerClient, buyerClient: buyerClient}
}

func (d *AccountBanDispatcher) BanUser(ctx context.Context, ownerID int64, reason string) error {
	if ownerID == 0 {
		// Defence in depth: report.go's banTargetOwnerID already refuses this,
		// but a second caller of the interface must not be able to turn "no
		// owner" into SuspendAccount(0).
		return ErrUnknownTargetOwner
	}
	principal := ownerID
	if d.sellerClient != nil {
		res, err := d.sellerClient.ResolvePrincipalStrict(ctx, principal)
		if err != nil {
			return fmt.Errorf("resolve seller principal for ban: %w", err)
		}
		if res.Found {
			principal = res.AccountID
		}
	}
	if d.buyerClient != nil {
		res, err := d.buyerClient.ResolvePrincipalStrict(ctx, principal)
		if err != nil {
			return fmt.Errorf("resolve buyer principal for ban: %w", err)
		}
		if res.Found {
			principal = res.AccountID
		}
	}
	if d.accountClient == nil {
		return nil
	}
	return d.accountClient.SuspendAccount(ctx, principal, reason)
}
