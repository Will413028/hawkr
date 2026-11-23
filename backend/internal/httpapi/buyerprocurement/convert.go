package buyerprocurement

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
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
	for _, m := range s.Messages {
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
		out.Messages = append(out.Messages, msg)
	}
	for _, r := range s.Runs {
		st, err := runStatusToProto(r.Status)
		if err != nil {
			return nil, err
		}
		rt, err := runTypeToProto(r.RunType)
		if err != nil {
			return nil, err
		}
		out.Runs = append(out.Runs, &pb.RunSummary{RunId: r.RunID.String(), RunType: rt, Status: st, LastCursor: r.LastCursor, Terminal: r.Terminal})
	}
	if s.ActiveRunID != nil {
		out.ActiveRunId = s.ActiveRunID.String()
	}
	if c := s.OpenConfirmation; c != nil {
		kind, err := interruptKindToProto(c.Kind)
		if err != nil {
			return nil, err
		}
		key, err := slotKeyToProto(c.Question.SlotKey)
		if err != nil {
			return nil, err
		}
		out.OpenConfirmation = &pb.OpenConfirmation{
			InterruptId: c.InterruptID.String(), Kind: kind, Processing: c.Processing,
			Question: &pb.ClarifyQuestion{
				SlotKey: key, Prompt: c.Question.Prompt, Options: optionsToProto(c.Question.Options), UnitOptions: optionsToProto(c.Question.UnitOptions),
				AllowFreeText: c.Question.AllowFreeText, CanSkip: c.Question.CanSkip,
			},
		}
	}
	if p := s.PublishedRfq; p != nil {
		out.PublishedRfq = &pb.PublishedRfq{
			PublicationId: p.PublicationID.String(), Version: fmt.Sprint(p.Version), PublishTime: timestamppb.New(p.PublishTime),
			RecipientCount: p.RecipientCount, QueuedCount: p.QueuedCount, ConfirmedRfqSnapshotId: p.ConfirmedRfqSnapshotID.String(),
			RfqSnapshotHash: p.RfqSnapshotHash,
		}
	}
	return out, nil
}
