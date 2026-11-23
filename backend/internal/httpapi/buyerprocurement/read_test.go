package buyerprocurement

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestReadAndConversationMethodsReturnNotFoundForAnotherBuyer(t *testing.T) {
	c := setup(t, &fakeModule{})
	other := accountID + 1
	id := ownedRun.String()
	calls := map[string]func() error{
		"GetSnapshot": func() error {
			_, err := c.GetSnapshot(t.Context(), as(connect.NewRequest(&pb.GetSnapshotRequest{RequestId: id}), other))
			return err
		},
		"GetConversationMessages": func() error {
			_, err := c.GetConversationMessages(t.Context(), as(connect.NewRequest(&pb.GetConversationMessagesRequest{ConversationId: id}), other))
			return err
		},
		"RenameConversation": func() error {
			_, err := c.RenameConversation(t.Context(), as(connect.NewRequest(&pb.RenameConversationRequest{ConversationId: id, Title: "x", ClientRequestId: goodID}), other))
			return err
		},
		"DeleteConversation": func() error {
			_, err := c.DeleteConversation(t.Context(), as(connect.NewRequest(&pb.DeleteConversationRequest{ConversationId: id, ClientRequestId: goodID}), other))
			return err
		},
		"GetCandidateSearch": func() error {
			_, err := c.GetCandidateSearch(t.Context(), as(connect.NewRequest(&pb.GetCandidateSearchRequest{RequestId: id, RunId: id}), other))
			return err
		},
	}
	for name, call := range calls {
		if got := connect.CodeOf(call()); got != connect.CodeNotFound {
			t.Errorf("%s：別的買家 = %v，要 NOT_FOUND", name, got)
		}
	}
}

func TestListConversationsPassesPagingAndMapsViolations(t *testing.T) {
	now := time.Date(2026, 11, 23, 1, 2, 3, 0, time.UTC)
	m := &fakeModule{page: procurement.ConversationPage{
		Conversations: []procurement.ConversationSummary{{ID: ownedRun, Title: "蘋果｜台北市", LastMessagePreview: "好的", UpdateTime: now}}, NextPageToken: "next",
	}}
	c := setup(t, m)
	res, err := c.ListConversations(t.Context(), as(connect.NewRequest(&pb.ListConversationsRequest{PageSize: 7, PageToken: "tok"}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	got := res.Msg
	if len(got.GetConversations()) != 1 || got.GetConversations()[0].GetId() != ownedRun.String() || got.GetConversations()[0].GetTitle() != "蘋果｜台北市" ||
		got.GetConversations()[0].GetLastMessagePreview() != "好的" || !got.GetConversations()[0].GetUpdateTime().AsTime().Equal(now) || got.GetNextPageToken() != "next" {
		t.Errorf("列表 = %v", got)
	}
	if m.pageSizes[0] != 7 || m.pageTokens[0] != "tok" {
		t.Errorf("傳給模組的分頁 = %v %v", m.pageSizes, m.pageTokens)
	}
	m.listErr = apperr.InvalidInputOn([]string{"page_token"}, "壞 token")
	_, err = c.ListConversations(t.Context(), as(connect.NewRequest(&pb.ListConversationsRequest{PageToken: "bad"}), accountID))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("壞 token = %v", err)
	}
	if f := violationFields(t, err); len(f) != 1 || f[0] != "page_token" {
		t.Errorf("欄位 = %v", f)
	}
}

func TestRenameAndDeleteValidateTheIdempotencyKeyAndMapTitleViolations(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	id := ownedRun.String()
	_, err := c.RenameConversation(t.Context(), as(connect.NewRequest(&pb.RenameConversationRequest{ConversationId: id, Title: "x"}), accountID))
	if f := violationFields(t, err); len(f) != 1 || f[0] != "client_request_id" {
		t.Errorf("缺 client_request_id = %v", f)
	}
	_, err = c.DeleteConversation(t.Context(), as(connect.NewRequest(&pb.DeleteConversationRequest{ConversationId: id}), accountID))
	if f := violationFields(t, err); len(f) != 1 || f[0] != "client_request_id" {
		t.Errorf("刪除缺 client_request_id = %v", f)
	}
	m.renameErr = apperr.InvalidInputOn([]string{"title"}, "標題太長")
	_, err = c.RenameConversation(t.Context(), as(connect.NewRequest(&pb.RenameConversationRequest{ConversationId: id, Title: "x", ClientRequestId: goodID}), accountID))
	if f := violationFields(t, err); len(f) != 1 || f[0] != "title" {
		t.Errorf("標題違規 = %v", f)
	}
	m.renameErr = nil
	if _, err := c.RenameConversation(t.Context(), as(connect.NewRequest(&pb.RenameConversationRequest{ConversationId: id, Title: "新名字", ClientRequestId: goodID}), accountID)); err != nil || m.renamed[len(m.renamed)-1] != "新名字" {
		t.Errorf("改名 = %v %v", err, m.renamed)
	}
	if _, err := c.DeleteConversation(t.Context(), as(connect.NewRequest(&pb.DeleteConversationRequest{ConversationId: id, ClientRequestId: goodID}), accountID)); err != nil || m.deleted != 1 {
		t.Errorf("刪除 = %v %d", err, m.deleted)
	}
}

func TestGetSnapshotMapsEveryFieldAndRejectsUnspecified(t *testing.T) {
	now := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	item, qty, city := "蘋果", int32(3), "台北市"
	date := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	active := ownedRun
	m := &fakeModule{requestSnapshot: procurement.RequestSnapshot{
		RequestID: ownedRun, RevisionID: ownedRun, Status: "CLARIFYING",
		Slots: procurement.RequirementSlots{Item: &item, Quantity: &qty, DeliveryCity: &city, DeliveryDate: &date,
			ItemResolution: &procurement.ItemResolution{Kind: "CATEGORY_CONFIRMED", Label: "蘋果", DisplayValue: "蘋果"}},
		MissingRequired:    []procurement.SlotKey{"UNIT"},
		ItemDisambiguation: []string{"蘋果", "蘋果乾"},
		Runs:               []procurement.RunSummary{{RunID: ownedRun, RunType: "TURN", Status: "RUNNING", LastCursor: "2"}},
		ActiveRunID:        &active,
		PublishedRfq:       &procurement.PublishedRfq{PublicationID: ownedRun, Version: 1, PublishTime: now, ConfirmedRfqSnapshotID: ownedRun, RfqSnapshotHash: "abc"},
	}}
	c := setup(t, m)
	res, err := c.GetSnapshot(t.Context(), as(connect.NewRequest(&pb.GetSnapshotRequest{RequestId: ownedRun.String()}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	s := res.Msg
	d := s.GetSlots().GetDeliveryDate()
	if s.GetRevisionId() != ownedRun.String() || s.GetStatus() != pb.RequestStatus_REQUEST_STATUS_CLARIFYING || s.GetSlots().GetItem() != "蘋果" || s.GetSlots().GetQuantity() != 3 ||
		s.GetSlots().GetDeliveryCity() != "台北市" || d.GetYear() != 2026 || d.GetMonth() != 12 || d.GetDay() != 1 ||
		s.GetSlots().GetItemResolution().GetKind() != pb.ItemResolutionKind_ITEM_RESOLUTION_KIND_CATEGORY_CONFIRMED ||
		len(s.GetMissingRequired()) != 1 || s.GetMissingRequired()[0] != pb.SlotKey_SLOT_KEY_UNIT ||
		len(s.GetItemDisambiguation().GetCandidates()) != 2 || s.GetItemDisambiguation().GetCandidates()[1].GetLabel() != "蘋果乾" ||
		s.GetActiveRunId() != active.String() || s.GetRuns()[0].GetStatus() != pb.RunStatus_RUN_STATUS_RUNNING ||
		s.GetPublishedRfq().GetRfqSnapshotHash() != "abc" || s.GetOpenConfirmation() != nil {
		t.Fatalf("快照 = %v", s)
	}
	m.requestSnapshot.Status = ""
	_, err = c.GetSnapshot(t.Context(), as(connect.NewRequest(&pb.GetSnapshotRequest{RequestId: ownedRun.String()}), accountID))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("空的需求狀態 = %v，要 INTERNAL", err)
	}
}

func TestGetCandidateSearchAssemblesStoreCardsAndFacts(t *testing.T) {
	now := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	cand := func(store int64, selected bool) procurement.Candidate {
		return procurement.Candidate{
			ID: uuid.NewV7(), StoreID: store, Selected: selected, FactsAsOf: now,
			MatchedFacts:      []procurement.MatchedFact{{SlotKey: "ITEM", Label: "品項"}},
			UnverifiableFacts: []procurement.UnverifiableFact{{SlotKey: "QUANTITY", Reason: "請確認數量 3"}},
		}
	}
	m := &fakeModule{search: procurement.CandidateSearch{
		RequestID: ownedRun, RunID: ownedRun, CandidateSetID: ownedRun, Status: "SUCCEEDED",
		Candidates: []procurement.Candidate{cand(900, true), cand(901, false)},
	}}
	c := setup(t, m)
	req := func() *connect.Request[pb.GetCandidateSearchRequest] {
		return as(connect.NewRequest(&pb.GetCandidateSearchRequest{RequestId: ownedRun.String(), RunId: ownedRun.String()}), accountID)
	}
	res, err := c.GetCandidateSearch(t.Context(), req())
	if err != nil {
		t.Fatal(err)
	}
	got := res.Msg
	if got.GetStatus() != pb.RunStatus_RUN_STATUS_SUCCEEDED || got.GetFailureReason() != pb.RunFailureReason_RUN_FAILURE_REASON_UNSPECIFIED || len(got.GetCandidates()) != 2 {
		t.Fatalf("成功 = %v", got)
	}
	first, second := got.GetCandidates()[0], got.GetCandidates()[1]
	if first.GetStore().GetName() != "店900" || !first.GetSelected() || first.GetMatchedFacts()[0].GetSlotKey() != pb.SlotKey_SLOT_KEY_ITEM ||
		first.GetUnverifiableFacts()[0].GetReason() != "請確認數量 3" || !first.GetFactsTime().AsTime().Equal(now) {
		t.Errorf("候選 = %v", first)
	}
	if second.GetStore().GetName() != "店901" || second.GetSelected() {
		t.Errorf("第二個候選 = %v", second)
	}
	m.search.Candidates = append(m.search.Candidates, cand(404, false))
	if _, err := c.GetCandidateSearch(t.Context(), req()); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("候選的店沒有店家卡片 = %v，要 INTERNAL（搜尋只收曾公開過的店）", err)
	}

	m.search = procurement.CandidateSearch{RequestID: ownedRun, RunID: ownedRun, CandidateSetID: ownedRun, Status: "QUEUED"}
	res, err = c.GetCandidateSearch(t.Context(), req())
	if err != nil || res.Msg.GetStatus() != pb.RunStatus_RUN_STATUS_QUEUED || len(res.Msg.GetCandidates()) != 0 || res.Msg.GetCandidateSetId() != ownedRun.String() {
		t.Errorf("QUEUED = %v %v", res, err)
	}
	m.search = procurement.CandidateSearch{RequestID: ownedRun, RunID: ownedRun, CandidateSetID: ownedRun, Status: "FAILED", FailureReason: "SEARCH_FAILED"}
	res, err = c.GetCandidateSearch(t.Context(), req())
	if err != nil || res.Msg.GetFailureReason() != pb.RunFailureReason_RUN_FAILURE_REASON_SEARCH_FAILED || len(res.Msg.GetCandidates()) != 0 {
		t.Errorf("FAILED = %v %v", res, err)
	}
	m.search.FailureReason = "NOT_A_REASON"
	if _, err := c.GetCandidateSearch(t.Context(), req()); connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("不認得的失敗原因 = %v，要 INTERNAL", err)
	}
}
