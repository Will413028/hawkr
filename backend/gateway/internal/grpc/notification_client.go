package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// NotificationClient is the gRPC client for notification service
type NotificationClient struct {
	conn   *grpc.ClientConn
	client pb.NotificationServiceClient
}

// NewNotificationClient creates a new notification gRPC client
func NewNotificationClient() (*NotificationClient, error) {
	host := getEnvOrDefault("NOTIFICATION_SERVICE_HOST", "127.0.0.1")
	port := getEnvOrDefault("NOTIFICATION_SERVICE_GRPC_PORT", "10037")
	target := fmt.Sprintf("%s:%s", host, port)

	conn, err := grpcclient.Dial(target, DefaultDialOpts()...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to notification service at %s: %w", target, err)
	}

	zap.S().Infof("Connected to notification gRPC service at %s", target)

	return &NotificationClient{
		conn:   conn,
		client: pb.NewNotificationServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection
func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetNotificationPreferences returns all notification preference categories for a user.
func (c *NotificationClient) GetNotificationPreferences(ctx context.Context, req *pb.GetNotificationPreferencesRequest) (*pb.GetNotificationPreferencesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetNotificationPreferences(ctx, req)
}

// UpdateNotificationPreference updates a single notification preference category.
func (c *NotificationClient) UpdateNotificationPreference(ctx context.Context, req *pb.UpdateNotificationPreferenceRequest) (*pb.UpdateNotificationPreferenceResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.UpdateNotificationPreference(ctx, req)
}

// RegisterDeviceToken registers or refreshes an FCM device token.
func (c *NotificationClient) RegisterDeviceToken(ctx context.Context, req *pb.RegisterDeviceTokenRequest) (*pb.RegisterDeviceTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.RegisterDeviceToken(ctx, req)
}

// RemoveDeviceToken deactivates an FCM device token (logout).
func (c *NotificationClient) RemoveDeviceToken(ctx context.Context, req *pb.RemoveDeviceTokenRequest) (*pb.RemoveDeviceTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.RemoveDeviceToken(ctx, req)
}
