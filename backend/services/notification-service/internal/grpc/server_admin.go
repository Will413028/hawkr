package grpc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/notification-service/internal/worker"
	"go.uber.org/zap"
)

func (s *Server) ReloadTemplates(ctx context.Context, _ *pb.ReloadTemplatesRequest) (*pb.ReloadTemplatesResponse, error) {
	n, err := s.templateCache.LoadFromDB(ctx)
	if err != nil {
		s.logger.Error("ReloadTemplates failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ReloadTemplatesResponse{Code: int32(code)}, nil
		}
		return nil, err
	}
	s.logger.Info("ReloadTemplates completed", zap.Int("count", n))
	return &pb.ReloadTemplatesResponse{Code: 0, ReloadedCount: strconv.Itoa(n)}, nil
}

func (s *Server) ListTemplates(ctx context.Context, req *pb.ListTemplatesRequest) (*pb.ListTemplatesResponse, error) {
	templates, err := s.templateRepo.FindAll(ctx)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.ListTemplatesResponse{Code: int32(code)}, nil
		}
		return nil, err
	}

	var filtered []*pb.NotificationTemplateProto
	for _, t := range templates {
		if req.EventType != "" && t.EventType != req.EventType {
			continue
		}
		if req.TargetUserType != "" && t.TargetUserType != req.TargetUserType {
			continue
		}
		switch req.Enabled {
		case "true":
			if !t.Enabled {
				continue
			}
		case "false":
			if t.Enabled {
				continue
			}
		}
		filtered = append(filtered, templateToProto(t))
	}

	return &pb.ListTemplatesResponse{Code: 0, Templates: filtered}, nil
}

func (s *Server) GetTemplate(ctx context.Context, req *pb.GetTemplateRequest) (*pb.GetTemplateResponse, error) {
	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.GetTemplateResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}
	t, err := s.templateRepo.FindByID(ctx, id)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.GetTemplateResponse{Code: int32(code)}, nil
		}
		return nil, err
	}
	if t == nil {
		return &pb.GetTemplateResponse{Code: int32(pkgerrors.CodeNotFound)}, nil
	}
	return &pb.GetTemplateResponse{Code: 0, Template: templateToProto(t)}, nil
}

// CreateTemplate / UpdateTemplate 已移除（issue #1176 SSOT 政策）：template 內容變更唯一入口 = Atlas migration（13 號設計書 §10.1 / §14.2）。

func (s *Server) UpdateTemplateEnabled(ctx context.Context, req *pb.UpdateTemplateEnabledRequest) (*pb.UpdateTemplateEnabledResponse, error) {
	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.UpdateTemplateEnabledResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid id"}, nil
	}
	if err := s.templateRepo.UpdateEnabled(ctx, id, req.Enabled); err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.UpdateTemplateEnabledResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	return &pb.UpdateTemplateEnabledResponse{Code: 0, Message: "ok"}, nil
}

func (s *Server) PreviewTemplate(ctx context.Context, req *pb.PreviewTemplateRequest) (*pb.PreviewTemplateResponse, error) {
	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.PreviewTemplateResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}
	t, err := s.templateRepo.FindByID(ctx, id)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.PreviewTemplateResponse{Code: int32(code)}, nil
		}
		return nil, err
	}
	if t == nil {
		return &pb.PreviewTemplateResponse{Code: int32(pkgerrors.CodeNotFound)}, nil
	}

	payload, err := worker.DecodePayload([]byte(req.SamplePayload))
	if err != nil {
		return &pb.PreviewTemplateResponse{Code: int32(pkgerrors.CodeInvalidInput)}, nil
	}

	renderedTitle, titleMissing := worker.RenderTemplate(t.TitleTemplate, payload)
	renderedBody, bodyMissing := worker.RenderTemplate(t.BodyTemplate, payload)

	dataMap := worker.BuildDataMap(t.DataFields, payload)
	dataMap["type"] = t.DataType

	var renderedSchemaURL string
	if t.SchemaURL != nil {
		renderedSchemaURL = worker.RenderBraces(*t.SchemaURL, dataMap)
		dataMap["url"] = renderedSchemaURL
	}

	var renderedRefID string
	if t.ReferenceIDTemplate != nil && *t.ReferenceIDTemplate != "" {
		renderedRefID, _ = worker.RenderTemplate(*t.ReferenceIDTemplate, payload)
	}

	missing := deduplicate(append(titleMissing, bodyMissing...))

	return &pb.PreviewTemplateResponse{
		Code:                0,
		RenderedTitle:       renderedTitle,
		RenderedBody:        renderedBody,
		RenderedData:        dataMap,
		RenderedSchemaUrl:   renderedSchemaURL,
		RenderedReferenceId: renderedRefID,
		MissingFields:       missing,
	}, nil
}

func (s *Server) TestSendTemplate(ctx context.Context, req *pb.TestSendTemplateRequest) (*pb.TestSendTemplateResponse, error) {
	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		return &pb.TestSendTemplateResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid id"}, nil
	}
	targetUserID, err := strconv.ParseInt(req.TargetUserId, 10, 64)
	if err != nil {
		return &pb.TestSendTemplateResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: "invalid target_user_id"}, nil
	}
	t, err := s.templateRepo.FindByID(ctx, id)
	if err != nil {
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.TestSendTemplateResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}
	if t == nil {
		return &pb.TestSendTemplateResponse{Code: int32(pkgerrors.CodeNotFound), Message: "template not found"}, nil
	}

	payload, err := worker.DecodePayload([]byte(req.SamplePayload))
	if err != nil {
		return &pb.TestSendTemplateResponse{Code: int32(pkgerrors.CodeInvalidInput), Message: fmt.Sprintf("invalid sample_payload JSON: %v", err)}, nil
	}

	title, _ := worker.RenderTemplate(t.TitleTemplate, payload)
	body, missingBody := worker.RenderTemplate(t.BodyTemplate, payload)
	if len(missingBody) > 0 && t.BodyFallback != nil {
		body = *t.BodyFallback
	}

	dataMap := worker.BuildDataMap(t.DataFields, payload)
	dataMap["type"] = t.DataType
	dataMap["category"] = t.Category
	if t.SchemaURL != nil {
		dataMap["url"] = worker.RenderBraces(*t.SchemaURL, dataMap)
	}

	userType := req.TargetUserType
	if userType == "" {
		userType = t.TargetUserType
	}

	referenceID := fmt.Sprintf("test_%d", time.Now().UnixMilli())

	if err := s.sendPushUC.Execute(ctx, &dto.SendPushRequest{
		UserID:      targetUserID,
		UserType:    userType,
		EventType:   t.EventType,
		Category:    t.Category,
		ReferenceID: referenceID,
		Title:       title,
		Body:        body,
		Data:        dataMap,
	}); err != nil {
		s.logger.Error("TestSendTemplate push failed", zap.Error(err))
		if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnknown {
			return &pb.TestSendTemplateResponse{Code: int32(code), Message: err.Error()}, nil
		}
		return nil, err
	}

	return &pb.TestSendTemplateResponse{Code: 0, Message: "ok"}, nil
}

func dataFieldsToProto(in map[string]domain.FieldSpec) map[string]*pb.FieldSpec {
	if in == nil {
		return nil
	}
	out := make(map[string]*pb.FieldSpec, len(in))
	for k, spec := range in {
		switch {
		case spec.Const != nil:
			out[k] = &pb.FieldSpec{Spec: &pb.FieldSpec_Const{Const: *spec.Const}}
		case spec.Field != nil:
			out[k] = &pb.FieldSpec{Spec: &pb.FieldSpec_Field{Field: *spec.Field}}
		default:
			out[k] = &pb.FieldSpec{}
		}
	}
	return out
}

func templateToProto(t *domain.NotificationTemplate) *pb.NotificationTemplateProto {
	p := &pb.NotificationTemplateProto{
		Id:              strconv.FormatInt(t.ID, 10),
		Exchange:        t.Exchange,
		RoutingKey:      t.RoutingKey,
		EventType:       t.EventType,
		TargetUserType:  t.TargetUserType,
		TargetUserField: t.TargetUserField,
		Category:        t.Category,
		TitleTemplate:   t.TitleTemplate,
		BodyTemplate:    t.BodyTemplate,
		DataType:        t.DataType,
		DataFields:      dataFieldsToProto(t.DataFields),
		Enabled:         t.Enabled,
	}
	if t.BodyFallback != nil {
		p.BodyFallback = *t.BodyFallback
	}
	if t.SchemaURL != nil {
		p.SchemaUrl = *t.SchemaURL
	}
	if t.ReferenceIDTemplate != nil {
		p.ReferenceIdTemplate = *t.ReferenceIDTemplate
	}
	if t.UpdatedBy != nil {
		p.UpdatedBy = strconv.FormatInt(*t.UpdatedBy, 10)
	}
	return p
}

func deduplicate(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
