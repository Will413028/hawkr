package handler

import (
	"context"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"google.golang.org/grpc"
)

type delegationStub struct {
	accountpb.DelegationServiceClient
	intents  []*accountpb.RecordAuditIntentRequest
	confirms []*accountpb.ConfirmAuditResultRequest
}

func (s *delegationStub) RecordAuditIntent(ctx context.Context, in *accountpb.RecordAuditIntentRequest, opts ...grpc.CallOption) (*accountpb.RecordAuditIntentResponse, error) {
	s.intents = append(s.intents, in)
	return &accountpb.RecordAuditIntentResponse{
		Code: 0,
		Data: &accountpb.RecordAuditIntentData{AuditLogId: "2701445588392017"},
	}, nil
}

func (s *delegationStub) ConfirmAuditResult(ctx context.Context, in *accountpb.ConfirmAuditResultRequest, opts ...grpc.CallOption) (*accountpb.ConfirmAuditResultResponse, error) {
	s.confirms = append(s.confirms, in)
	return &accountpb.ConfirmAuditResultResponse{Code: 0}, nil
}
