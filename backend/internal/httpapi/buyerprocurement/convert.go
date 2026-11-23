package buyerprocurement

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
)

// 模組的狀態、角色、run 類型是 proto enum 值名去掉前綴的代碼；UNSPECIFIED 與不認得的代碼是不變量被破壞，回沒有分類的錯誤。

func runStatusToProto(code string) (pb.RunStatus, error) {
	v, ok := pb.RunStatus_value["RUN_STATUS_"+code]
	if !ok || v == int32(pb.RunStatus_RUN_STATUS_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：run 狀態 %q 沒有對應的 RunStatus", code)
	}
	return pb.RunStatus(v), nil
}

func runTypeToProto(code string) (pb.RunType, error) {
	v, ok := pb.RunType_value["RUN_TYPE_"+code]
	if !ok || v == int32(pb.RunType_RUN_TYPE_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：run 類型 %q 沒有對應的 RunType", code)
	}
	return pb.RunType(v), nil
}

func requestStatusToProto(code string) (pb.RequestStatus, error) {
	v, ok := pb.RequestStatus_value["REQUEST_STATUS_"+code]
	if !ok || v == int32(pb.RequestStatus_REQUEST_STATUS_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：需求狀態 %q 沒有對應的 RequestStatus", code)
	}
	return pb.RequestStatus(v), nil
}

func roleToProto(code string) (pb.ConversationRole, error) {
	v, ok := pb.ConversationRole_value["CONVERSATION_ROLE_"+code]
	if !ok || v == int32(pb.ConversationRole_CONVERSATION_ROLE_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：訊息角色 %q 沒有對應的 ConversationRole", code)
	}
	return pb.ConversationRole(v), nil
}

func interruptKindToProto(code string) (pb.InterruptKind, error) {
	v, ok := pb.InterruptKind_value["INTERRUPT_KIND_"+code]
	if !ok || v == int32(pb.InterruptKind_INTERRUPT_KIND_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：中斷種類 %q 沒有對應的 InterruptKind", code)
	}
	return pb.InterruptKind(v), nil
}

func slotKeyToProto(k procurement.SlotKey) (pb.SlotKey, error) {
	v, ok := pb.SlotKey_value["SLOT_KEY_"+string(k)]
	if !ok || v == int32(pb.SlotKey_SLOT_KEY_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：欄位 %q 沒有對應的 SlotKey", k)
	}
	return pb.SlotKey(v), nil
}

// slotKeyFromProto 把請求的欄位代碼轉成模組的欄位代碼；沒給（UNSPECIFIED）是缺必填欄位。
func slotKeyFromProto(k pb.SlotKey) (procurement.SlotKey, error) {
	if k == pb.SlotKey_SLOT_KEY_UNSPECIFIED {
		return "", rpc.RequiredField("slot_key", "slot_key 不能是空的")
	}
	name, ok := strings.CutPrefix(k.String(), "SLOT_KEY_")
	if !ok {
		return "", rpc.RequiredField("slot_key", "slot_key 不認得")
	}
	return procurement.SlotKey(name), nil
}

func dispositionToProto(code string) (pb.CancelDisposition, error) {
	switch code {
	case procurement.CancelDispositionRequested:
		return pb.CancelDisposition_CANCEL_DISPOSITION_REQUESTED, nil
	case procurement.CancelDispositionAlreadyCancelled:
		return pb.CancelDisposition_CANCEL_DISPOSITION_ALREADY_CANCELLED, nil
	case procurement.CancelDispositionAlreadyTerminal:
		return pb.CancelDisposition_CANCEL_DISPOSITION_ALREADY_TERMINAL, nil
	}
	return 0, fmt.Errorf("buyerprocurement：取消結果 %q 沒有對應的 CancelDisposition", code)
}

func optionsToProto(in []procurement.QuestionOption) []*pb.QuestionOption {
	var out []*pb.QuestionOption
	for _, o := range in {
		out = append(out, &pb.QuestionOption{OptionId: o.OptionID, Label: o.Label})
	}
	return out
}

func snapshotToProto(s procurement.RuntimeSnapshot) (*pb.GetRuntimeSnapshotResponse, error) {
	status, err := requestStatusToProto(s.Status)
	if err != nil {
		return nil, err
	}
	out := &pb.GetRuntimeSnapshotResponse{
		ConversationId: s.ConversationID.String(), RequestId: s.RequestID.String(), Status: status, Title: s.Title,
	}
	if out.Messages, err = messagesToProto(s.Messages); err != nil {
		return nil, err
	}
	if out.Runs, err = runsToProto(s.Runs); err != nil {
		return nil, err
	}
	if s.ActiveRunID != nil {
		out.ActiveRunId = s.ActiveRunID.String()
	}
	if out.OpenConfirmation, err = confirmationToProto(s.OpenConfirmation); err != nil {
		return nil, err
	}
	out.PublishedRfq = publishedToProto(s.PublishedRfq)
	return out, nil
}

func messagesToProto(in []procurement.ConversationMessage) ([]*pb.ConversationMessage, error) {
	var out []*pb.ConversationMessage
	for _, m := range in {
		role, err := roleToProto(m.Role)
		if err != nil {
			return nil, err
		}
		msg := &pb.ConversationMessage{Id: m.ID.String(), Role: role, Text: m.Text, CreateTime: timestamppb.New(m.CreateTime)}
		for _, f := range m.RecordedThisTurn {
			k, err := slotKeyToProto(f.Key)
			if err != nil {
				return nil, err
			}
			msg.RecordedThisTurn = append(msg.RecordedThisTurn, &pb.RecordedField{Key: k, Label: f.Label, Value: f.Value})
		}
		out = append(out, msg)
	}
	return out, nil
}

func runsToProto(in []procurement.RunSummary) ([]*pb.RunSummary, error) {
	var out []*pb.RunSummary
	for _, r := range in {
		st, err := runStatusToProto(r.Status)
		if err != nil {
			return nil, err
		}
		rt, err := runTypeToProto(r.RunType)
		if err != nil {
			return nil, err
		}
		out = append(out, &pb.RunSummary{RunId: r.RunID.String(), RunType: rt, Status: st, LastCursor: r.LastCursor, Terminal: r.Terminal})
	}
	return out, nil
}

func confirmationToProto(c *procurement.OpenConfirmation) (*pb.OpenConfirmation, error) {
	if c == nil {
		return nil, nil
	}
	kind, err := interruptKindToProto(c.Kind)
	if err != nil {
		return nil, err
	}
	key, err := slotKeyToProto(c.Question.SlotKey)
	if err != nil {
		return nil, err
	}
	return &pb.OpenConfirmation{
		InterruptId: c.InterruptID.String(), Kind: kind, Processing: c.Processing,
		Question: &pb.ClarifyQuestion{
			SlotKey: key, Prompt: c.Question.Prompt, Options: optionsToProto(c.Question.Options), UnitOptions: optionsToProto(c.Question.UnitOptions),
			AllowFreeText: c.Question.AllowFreeText, CanSkip: c.Question.CanSkip,
		},
	}, nil
}

func publishedToProto(p *procurement.PublishedRfq) *pb.PublishedRfq {
	if p == nil {
		return nil
	}
	return &pb.PublishedRfq{
		PublicationId: p.PublicationID.String(), Version: fmt.Sprint(p.Version), PublishTime: timestamppb.New(p.PublishTime),
		RecipientCount: p.RecipientCount, QueuedCount: p.QueuedCount, SellerFacingHash: p.SellerFacingHash,
	}
}

func timestampOf(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

func itemResolutionKindToProto(code string) (pb.ItemResolutionKind, error) {
	v, ok := pb.ItemResolutionKind_value["ITEM_RESOLUTION_KIND_"+code]
	if !ok || v == int32(pb.ItemResolutionKind_ITEM_RESOLUTION_KIND_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：品項解析種類 %q 沒有對應的 ItemResolutionKind", code)
	}
	return pb.ItemResolutionKind(v), nil
}

func failureReasonToProto(code string) (pb.RunFailureReason, error) {
	v, ok := pb.RunFailureReason_value["RUN_FAILURE_REASON_"+code]
	if !ok || v == int32(pb.RunFailureReason_RUN_FAILURE_REASON_UNSPECIFIED) {
		return 0, fmt.Errorf("buyerprocurement：失敗原因 %q 沒有對應的 RunFailureReason", code)
	}
	return pb.RunFailureReason(v), nil
}

func slotsToProto(s procurement.RequirementSlots) (*pb.Slots, error) {
	out := &pb.Slots{
		Item: s.Item, Quantity: s.Quantity, Unit: s.Unit, Packaging: s.Packaging, BudgetPerUnit: s.BudgetPerUnit,
		DeliveryCity: s.DeliveryCity, ExactMatchRequired: s.ExactMatchRequired,
	}
	if d := s.DeliveryDate; d != nil {
		out.DeliveryDate = &date.Date{Year: int32(d.Year()), Month: int32(d.Month()), Day: int32(d.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
	}
	if r := s.ItemResolution; r != nil {
		kind, err := itemResolutionKindToProto(r.Kind)
		if err != nil {
			return nil, err
		}
		out.ItemResolution = &pb.ItemResolution{Kind: kind, Label: r.Label, DisplayValue: r.DisplayValue}
	}
	return out, nil
}

func requestSnapshotToProto(s procurement.RequestSnapshot) (*pb.GetSnapshotResponse, error) {
	status, err := requestStatusToProto(s.Status)
	if err != nil {
		return nil, err
	}
	sl, err := slotsToProto(s.Slots)
	if err != nil {
		return nil, err
	}
	out := &pb.GetSnapshotResponse{RequestId: s.RequestID.String(), RevisionId: s.RevisionID.String(), Status: status, Slots: sl}
	for _, k := range s.MissingRequired {
		key, err := slotKeyToProto(k)
		if err != nil {
			return nil, err
		}
		out.MissingRequired = append(out.MissingRequired, key)
	}
	if out.Runs, err = runsToProto(s.Runs); err != nil {
		return nil, err
	}
	if s.ActiveRunID != nil {
		out.ActiveRunId = s.ActiveRunID.String()
	}
	if out.OpenConfirmation, err = confirmationToProto(s.OpenConfirmation); err != nil {
		return nil, err
	}
	out.PublishedRfq = publishedToProto(s.PublishedRfq)
	return out, nil
}

func candidateSearchToProto(s procurement.CandidateSearch, cards map[int64]*commonpb.StoreCard) (*pb.GetCandidateSearchResponse, error) {
	status, err := runStatusToProto(s.Status)
	if err != nil {
		return nil, err
	}
	out := &pb.GetCandidateSearchResponse{RequestId: s.RequestID.String(), RunId: s.RunID.String(), CandidateSetId: s.CandidateSetID.String(), Status: status}
	if s.FailureReason != "" {
		if out.FailureReason, err = failureReasonToProto(s.FailureReason); err != nil {
			return nil, err
		}
	}
	for _, c := range s.Candidates {
		card, ok := cards[c.StoreID]
		if !ok {
			return nil, fmt.Errorf("buyerprocurement：候選 %s 的店 %d 沒有店家卡片（搜尋只收曾公開過的店）", c.ID, c.StoreID)
		}
		cand := &pb.Candidate{Id: c.ID.String(), FactsTime: timestamppb.New(c.FactsAsOf), Selected: c.Selected, Store: card}
		for _, f := range c.MatchedFacts {
			k, err := slotKeyToProto(f.SlotKey)
			if err != nil {
				return nil, err
			}
			cand.MatchedFacts = append(cand.MatchedFacts, &pb.MatchedFact{SlotKey: k, Label: f.Label})
		}
		for _, f := range c.UnverifiableFacts {
			k, err := slotKeyToProto(f.SlotKey)
			if err != nil {
				return nil, err
			}
			cand.UnverifiableFacts = append(cand.UnverifiableFacts, &pb.UnverifiableFact{SlotKey: k, Reason: f.Reason})
		}
		out.Candidates = append(out.Candidates, cand)
	}
	return out, nil
}
