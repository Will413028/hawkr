package grpc

import (
	"context"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
)

// ReloadTemplates triggers an immediate cache reload on notification-service.
func (c *NotificationClient) ReloadTemplates(ctx context.Context, req *pb.ReloadTemplatesRequest) (*pb.ReloadTemplatesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.client.ReloadTemplates(ctx, req)
}

// ListTemplates returns notification templates with optional filters.
func (c *NotificationClient) ListTemplates(ctx context.Context, req *pb.ListTemplatesRequest) (*pb.ListTemplatesResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.ListTemplates(ctx, req)
}

// GetTemplate returns a single template by ID.
func (c *NotificationClient) GetTemplate(ctx context.Context, req *pb.GetTemplateRequest) (*pb.GetTemplateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.GetTemplate(ctx, req)
}

// UpdateTemplateEnabled toggles the enabled flag of a template.
func (c *NotificationClient) UpdateTemplateEnabled(ctx context.Context, req *pb.UpdateTemplateEnabledRequest) (*pb.UpdateTemplateEnabledResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.UpdateTemplateEnabled(ctx, req)
}

// PreviewTemplate renders the template with a sample payload.
func (c *NotificationClient) PreviewTemplate(ctx context.Context, req *pb.PreviewTemplateRequest) (*pb.PreviewTemplateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.PreviewTemplate(ctx, req)
}

// TestSendTemplate renders and sends a push to the specified account.
func (c *NotificationClient) TestSendTemplate(ctx context.Context, req *pb.TestSendTemplateRequest) (*pb.TestSendTemplateResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.client.TestSendTemplate(ctx, req)
}
