package grpc

import (
	"context"
	"fmt"
	"strconv"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/notification-service/internal/worker"
	"go.uber.org/zap"
)

type Server struct {
	pb.UnimplementedNotificationServiceServer
	tokenRepo     domain.DeviceTokenRepository
	notifier      domain.PushNotifier
	templateRepo  domain.NotificationTemplateRepository
	templateCache *worker.TemplateCache
	sendPushUC    *usecase.SendPushUseCase
	prefUC        *usecase.NotificationPreferenceUseCase
	inboxUC       *usecase.NotificationInboxUseCase
	registerUC    *usecase.RegisterTokenUseCase
	removeUC      *usecase.RemoveTokenUseCase
	// tsid mints the per-broadcast id that scopes BroadcastAnnouncement's
	// reference_id. See server_broadcast.go for why the operator id alone
	// is not enough.
	tsid   func() int64
	logger *zap.Logger
}

// NewServer creates a new gRPC server instance.
func NewServer(
	tokenRepo domain.DeviceTokenRepository,
	notifier domain.PushNotifier,
	templateRepo domain.NotificationTemplateRepository,
	templateCache *worker.TemplateCache,
	sendPushUC *usecase.SendPushUseCase,
	prefUC *usecase.NotificationPreferenceUseCase,
	inboxUC *usecase.NotificationInboxUseCase,
	registerUC *usecase.RegisterTokenUseCase,
	removeUC *usecase.RemoveTokenUseCase,
	tsid func() int64,
	logger *zap.Logger,
) *Server {
	return &Server{
		tokenRepo:     tokenRepo,
		notifier:      notifier,
		templateRepo:  templateRepo,
		templateCache: templateCache,
		sendPushUC:    sendPushUC,
		prefUC:        prefUC,
		inboxUC:       inboxUC,
		registerUC:    registerUC,
		removeUC:      removeUC,
		tsid:          tsid,
		logger:        logger,
	}
}

// GetNotificationPreferences returns all notification preference categories for a user.
func (s *Server) GetNotificationPreferences(ctx context.Context, req *pb.GetNotificationPreferencesRequest) (*pb.GetNotificationPreferencesResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.GetNotificationPreferencesResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}

	prefs, err := s.prefUC.GetPreferences(ctx, userID, req.UserType)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetNotificationPreferencesResponse{Code: int32(code)}, nil
		}
		return nil, err
	}

	items := make([]*pb.NotificationPreferenceItem, len(prefs))
	for i, p := range prefs {
		items[i] = &pb.NotificationPreferenceItem{
			Category:    p.Category,
			Enabled:     p.Enabled,
			Title:       p.Title,
			Description: p.Description,
			// 議題 6：forced 由 domain 層判斷（賣家 shipment_anomaly = true）。
			// 前端據此 disable toggle，避免 PUT 後才收 400 的 UX 缺口。
			Forced: domain.IsForcedCategory(req.UserType, p.Category),
		}
	}

	return &pb.GetNotificationPreferencesResponse{
		Code:        0,
		Preferences: items,
	}, nil
}

// UpdateNotificationPreference updates a single notification preference category.
func (s *Server) UpdateNotificationPreference(ctx context.Context, req *pb.UpdateNotificationPreferenceRequest) (*pb.UpdateNotificationPreferenceResponse, error) {
	if !domain.IsValidCategory(req.Category) {
		return &pb.UpdateNotificationPreferenceResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid category"}, nil
	}

	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.UpdateNotificationPreferenceResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid user_id"}, nil
	}

	if err := s.prefUC.UpdatePreference(ctx, userID, req.UserType, req.Category, req.Enabled); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.UpdateNotificationPreferenceResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.UpdateNotificationPreferenceResponse{Code: 0, Message: "ok"}, nil
}

// RegisterDeviceToken registers or refreshes an FCM device token.
func (s *Server) RegisterDeviceToken(ctx context.Context, req *pb.RegisterDeviceTokenRequest) (*pb.RegisterDeviceTokenResponse, error) {
	if req.Platform != domain.PlatformIOS {
		return &pb.RegisterDeviceTokenResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid platform"}, nil
	}

	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.RegisterDeviceTokenResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid user_id"}, nil
	}

	if err := s.registerUC.Execute(ctx, &dto.RegisterDeviceTokenRequest{
		UserID:   userID,
		UserType: req.UserType,
		Token:    req.Token,
		Platform: req.Platform,
	}); err != nil {
		s.logger.Error("RegisterDeviceToken failed", zap.Error(err))
		return nil, fmt.Errorf("register device token: %w", err)
	}

	return &pb.RegisterDeviceTokenResponse{Code: 0, Message: "ok"}, nil
}

// RemoveDeviceToken deactivates an FCM device token (logout).
func (s *Server) RemoveDeviceToken(ctx context.Context, req *pb.RemoveDeviceTokenRequest) (*pb.RemoveDeviceTokenResponse, error) {
	userID, err := strconv.ParseInt(req.UserId, 10, 64)
	if err != nil {
		return &pb.RemoveDeviceTokenResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid user_id"}, nil
	}

	if err := s.removeUC.Execute(ctx, &dto.RemoveDeviceTokenRequest{
		UserID:   userID,
		UserType: req.UserType,
		Token:    req.Token,
	}); err != nil {
		s.logger.Error("RemoveDeviceToken failed", zap.Error(err))
		return nil, fmt.Errorf("remove device token: %w", err)
	}

	return &pb.RemoveDeviceTokenResponse{Code: 0, Message: "ok"}, nil
}
