package grpc

import (
	"context"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
)

// ListNotifications returns paginated notifications for a user.
func (c *NotificationClient) ListNotifications(ctx context.Context, req *pb.ListNotificationsRequest) (*pb.ListNotificationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListNotifications(ctx, req)
}

// GetUnreadCount returns the count of unread notifications for a user.
func (c *NotificationClient) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.GetUnreadCountResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.GetUnreadCount(ctx, req)
}

// MarkAsRead marks a single notification as read.
func (c *NotificationClient) MarkAsRead(ctx context.Context, req *pb.MarkAsReadRequest) (*pb.MarkAsReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.MarkAsRead(ctx, req)
}

// MarkAllAsRead marks all unread notifications as read for a user.
func (c *NotificationClient) MarkAllAsRead(ctx context.Context, req *pb.MarkAllAsReadRequest) (*pb.MarkAllAsReadResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.MarkAllAsRead(ctx, req)
}

// DeleteNotifications soft-deletes notifications by IDs.
func (c *NotificationClient) DeleteNotifications(ctx context.Context, req *pb.DeleteNotificationsRequest) (*pb.DeleteNotificationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.DeleteNotifications(ctx, req)
}

// BroadcastAnnouncement publishes a system announcement to all users.
func (c *NotificationClient) BroadcastAnnouncement(ctx context.Context, req *pb.BroadcastAnnouncementRequest) (*pb.BroadcastAnnouncementResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.BroadcastAnnouncement(ctx, req)
}
