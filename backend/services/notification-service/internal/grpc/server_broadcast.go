package grpc

import (
	"context"
	"fmt"
	"strconv"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
	"go.uber.org/zap"
)

// announcementEventType tags both the inbox row and the push_notifications row.
// Kept as the historical value so existing rows and this path stay one series —
// the DB template that once carried the same event_type was a ghost (no
// publisher ever emitted `notification.announcement`) and is deleted in
// 20260822130000; this RPC is now the single writer for platform announcements.
const announcementEventType = "notification.announcement"

// BroadcastAnnouncement fans a platform announcement out to every user of the
// target type who has an active device token, one recipient at a time through
// the same SendPushUseCase every template-driven notification uses.
//
// Going through the usecase rather than writing push_notifications directly is
// what buys idempotency (ExistsProcessed), the recipient's category preference,
// an honest status/error_log, and deactivation of permanently dead FCM tokens.
// The previous hand-rolled loop had none of those: it hard-coded status=sent,
// dropped the repository error with `_ =`, and pushed to users who had switched
// the `system` category off — which 13 號設計書 §9 says it must respect.
//
// Recipient reach is unchanged on purpose: only users with an active device
// token are enumerable here. Reaching users who never installed the App needs a
// cross-service recipient source and is a feature, not part of this fix.
func (s *Server) BroadcastAnnouncement(ctx context.Context, req *pb.BroadcastAnnouncementRequest) (*pb.BroadcastAnnouncementResponse, error) {
	if req.Title == "" || req.Body == "" {
		return &pb.BroadcastAnnouncementResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "title and body are required"}, nil
	}
	if req.TargetUserType == "" {
		req.TargetUserType = "all"
	}

	targets := []string{req.TargetUserType}
	if req.TargetUserType == "all" {
		targets = []string{domain.UserTypeSeller, domain.UserTypeBuyer}
	}

	// One id per broadcast, not per operator. reference_id used to be
	// `broadcast_{operatorId}_{userId}`, which carries no announcement identity:
	// the same admin's second announcement to the same user collided with the
	// first, so the inbox INSERT was swallowed by ON CONFLICT DO NOTHING and the
	// push row hit the dedup index — while FCM had already delivered the banner.
	// The visible result was an announcement that flashed once and existed
	// nowhere afterwards.
	broadcastID := s.tsid()
	// category is not decoration: the inbox list API filters on data->>'category',
	// so an announcement without it falls outside every category-filtered query.
	data := map[string]string{
		"type":           "SYSTEM_ANNOUNCEMENT",
		"category":       domain.CategorySystem,
		"announcementId": strconv.FormatInt(broadcastID, 10),
	}

	var recipients int
	for _, userType := range targets {
		// operator_id no longer scopes reference_id, so it is logged instead —
		// otherwise dropping it from the key would also drop the only record of
		// who sent the announcement.
		s.logger.Info("broadcasting announcement",
			zap.Int64("broadcast_id", broadcastID),
			zap.String("operator_id", req.OperatorId),
			zap.String("target_user_type", userType),
			zap.String("title", req.Title),
		)

		const batchSize = int64(100)
		var offset int64
		// Deduped across pages, not per page: a user with several devices can
		// straddle a page boundary.
		seen := make(map[int64]bool)
		for {
			tokens, err := s.tokenRepo.GetActiveByUserType(ctx, userType, batchSize, offset)
			if err != nil {
				s.logger.Error("failed to get tokens for broadcast",
					zap.Error(err),
					zap.Int64("broadcast_id", broadcastID),
					zap.String("target_user_type", userType))
				break
			}
			if len(tokens) == 0 {
				break
			}

			for _, t := range tokens {
				if seen[t.UserID] {
					continue
				}
				seen[t.UserID] = true
				s.deliverAnnouncement(ctx, broadcastID, t.UserID, userType, req.Title, req.Body, data)
				recipients++
			}

			if int64(len(tokens)) < batchSize {
				break
			}
			offset += batchSize
		}
	}

	return &pb.BroadcastAnnouncementResponse{
		Code:           0,
		Message:        fmt.Sprintf("已推播給 %d 位使用者", recipients),
		BroadcastId:    strconv.FormatInt(broadcastID, 10),
		RecipientCount: strconv.Itoa(recipients),
	}, nil
}

// deliverAnnouncement writes one recipient's inbox row and then hands the push
// to SendPushUseCase. Same order as the template engine's handleTemplate: the
// inbox row is the durable copy and must not depend on the push succeeding, and
// the category preference gates the push only — a user who muted `system` still
// finds the announcement in their message center.
func (s *Server) deliverAnnouncement(ctx context.Context, broadcastID, userID int64, userType, title, body string, data map[string]string) {
	referenceID := fmt.Sprintf("announcement_%d_%d", broadcastID, userID)

	if err := s.inboxUC.CreateInboxItem(ctx, &domain.InboxItem{
		UserID:      userID,
		UserType:    userType,
		EventType:   announcementEventType,
		ReferenceID: referenceID,
		Title:       title,
		Body:        body,
		Data:        data,
	}); err != nil {
		s.logger.Error("failed to write broadcast inbox item",
			zap.Error(err),
			zap.Int64("broadcast_id", broadcastID),
			zap.Int64("user_id", userID))
	}

	if err := s.sendPushUC.Execute(ctx, &dto.SendPushRequest{
		UserID:      userID,
		UserType:    userType,
		EventType:   announcementEventType,
		ReferenceID: referenceID,
		Title:       title,
		Body:        body,
		Data:        data,
		Category:    domain.CategorySystem,
	}); err != nil {
		s.logger.Error("failed to send broadcast push",
			zap.Error(err),
			zap.Int64("broadcast_id", broadcastID),
			zap.Int64("user_id", userID))
	}
}
