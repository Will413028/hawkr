package seller

import (
	"context"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// onboardingModule 是 GetMyOnboarding 的來源：MyKYC 回 kyc（或 kycErr），Store 回 reread（refresh 同步之後重讀店家，條件已經是同步之後的）。
type onboardingModule struct {
	Module
	kyc     model.KYCApplication
	kycErr  error
	reread  model.Store
	rereads int
}

func (m *onboardingModule) MyKYC(context.Context, int64) (model.KYCApplication, error) {
	return m.kyc, m.kycErr
}

func (m *onboardingModule) Store(context.Context, int64) (model.Store, error) {
	m.rereads++
	return m.reread, nil
}

type onboardingAdminModule struct {
	AdminModule
	store  model.Store
	kyc    model.KYCApplication
	found  bool
	kycErr error
}

func (m *onboardingAdminModule) Store(context.Context, int64) (model.Store, error) {
	return m.store, nil
}

func (m *onboardingAdminModule) StoreKYC(context.Context, int64) (model.KYCApplication, bool, error) {
	return m.kyc, m.found, m.kycErr
}

func storeWithGate(kyc, transfers bool) model.Store {
	s := currentStore()
	s.Conditions.KYCApproved, s.Conditions.TransfersActive = kyc, transfers
	return s
}

func taskStates(resp *pb.GetMyOnboardingResponse) map[pb.OnboardingTaskId]pb.OnboardingTaskState {
	out := map[pb.OnboardingTaskId]pb.OnboardingTaskState{}
	for _, t := range resp.GetTasks() {
		out[t.GetId()] = t.GetState()
	}
	return out
}

func TestOnboardingEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"OnboardingTaskId":    rpc.EnumMismatches[pb.OnboardingTaskId](model.OnboardingTaskIDValues()),
		"OnboardingTaskState": rpc.EnumMismatches[pb.OnboardingTaskState](model.OnboardingTaskStateValues()),
		"OnboardingAction":    rpc.EnumMismatches[pb.OnboardingAction](model.OnboardingActionValues()),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

// GetMyOnboarding 把任務清單轉成契約：順序、id、state、action 與 LocalizedMessage 都設，complete 等於閘門，不出現 UNSPECIFIED。
func TestGetMyOnboardingReturnsOrderedTasksAndTheGate(t *testing.T) {
	m := &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusSubmitted}}
	pay := &paymentModule{configured: true}
	h := &App{m: m, payments: pay}
	got, err := h.GetMyOnboarding(ctxWithStore(storeWithGate(false, false)), connect.NewRequest(&pb.GetMyOnboardingRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var ids []pb.OnboardingTaskId
	for _, task := range got.Msg.GetTasks() {
		ids = append(ids, task.GetId())
		if task.GetId() == pb.OnboardingTaskId_ONBOARDING_TASK_ID_UNSPECIFIED || task.GetState() == pb.OnboardingTaskState_ONBOARDING_TASK_STATE_UNSPECIFIED ||
			task.GetAction() == pb.OnboardingAction_ONBOARDING_ACTION_UNSPECIFIED || task.GetTitle().GetMessage() == "" || task.GetStatus().GetMessage() == "" || task.GetStatus().GetLocale() == "" {
			t.Errorf("任務有沒設的欄位：%v", task)
		}
	}
	want := []pb.OnboardingTaskId{pb.OnboardingTaskId_ONBOARDING_TASK_ID_PROFILE, pb.OnboardingTaskId_ONBOARDING_TASK_ID_PLATFORM_REVIEW, pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT}
	if !slices.Equal(ids, want) || got.Msg.GetComplete() {
		t.Fatalf("ids=%v complete=%v", ids, got.Msg.GetComplete())
	}
	states := taskStates(got.Msg)
	if states[pb.OnboardingTaskId_ONBOARDING_TASK_ID_PLATFORM_REVIEW] != pb.OnboardingTaskState_ONBOARDING_TASK_STATE_IN_PROGRESS ||
		states[pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT] != pb.OnboardingTaskState_ONBOARDING_TASK_STATE_LOCKED {
		t.Fatalf("states=%v", states)
	}
	if len(pay.syncedFor) != 0 || m.rereads != 0 {
		t.Fatalf("refresh 沒設卻同步了：%v、重讀店家 %d 次", pay.syncedFor, m.rereads)
	}
}

// 讀取失敗只讓需要那個來源的任務是 UNKNOWN，方法本身成功（D26）：KYC 讀不到時平台審核是 UNKNOWN；KYC 已核准、收款帳戶讀不到時只有收款帳戶是 UNKNOWN。
// 沒有收款帳戶列（NotFound）不是失敗。
// mutation：KYC 讀取失敗時回錯誤 → 這個測試的 err 不是 nil，失敗。
func TestGetMyOnboardingMarksOnlyTheFailedSourceUnknown(t *testing.T) {
	const (
		review  = pb.OnboardingTaskId_ONBOARDING_TASK_ID_PLATFORM_REVIEW
		payment = pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT
		profile = pb.OnboardingTaskId_ONBOARDING_TASK_ID_PROFILE
	)
	unknown, locked, done, progress := pb.OnboardingTaskState_ONBOARDING_TASK_STATE_UNKNOWN, pb.OnboardingTaskState_ONBOARDING_TASK_STATE_LOCKED,
		pb.OnboardingTaskState_ONBOARDING_TASK_STATE_COMPLETED, pb.OnboardingTaskState_ONBOARDING_TASK_STATE_IN_PROGRESS
	boom := errors.New("來源壞了")
	cases := []struct {
		name   string
		store  model.Store
		kycErr error
		payErr error
		want   map[pb.OnboardingTaskId]pb.OnboardingTaskState
	}{
		{"KYC 讀不到", storeWithGate(false, false), boom, nil, map[pb.OnboardingTaskId]pb.OnboardingTaskState{review: unknown, payment: locked}},
		{"收款帳戶讀不到", storeWithGate(true, false), nil, boom, map[pb.OnboardingTaskId]pb.OnboardingTaskState{review: done, payment: unknown}},
		{"收款帳戶沒有列不是失敗", storeWithGate(true, false), nil, apperr.NotFound("沒有"), map[pb.OnboardingTaskId]pb.OnboardingTaskState{review: done, payment: progress}},
		{"兩個來源都讀不到、閘門已通過：完成的任務不需要讀取結果", storeWithGate(true, true), boom, boom, map[pb.OnboardingTaskId]pb.OnboardingTaskState{review: done, payment: done}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusApproved}, kycErr: tc.kycErr}
			h := &App{m: m, payments: &paymentModule{err: tc.payErr}}
			got, err := h.GetMyOnboarding(ctxWithStore(tc.store), connect.NewRequest(&pb.GetMyOnboardingRequest{}))
			if err != nil {
				t.Fatalf("單一來源讀取失敗讓方法失敗：%v", err)
			}
			states := taskStates(got.Msg)
			for id, want := range tc.want {
				if states[id] != want {
					t.Errorf("%v：%v，要 %v", id, states[id], want)
				}
			}
			if states[profile] != pb.OnboardingTaskState_ONBOARDING_TASK_STATE_ACTION_REQUIRED {
				t.Errorf("商店資料（缺 logo）：%v", states[profile])
			}
		})
	}
}

// refresh 為 true 時先同步重讀收款帳戶，再以同步之後的店家條件組清單；同步失敗只記錄，以原本的條件回傳，不讓方法失敗；refresh 為 false 不同步。
// mutation：refresh 不呼叫 SyncAccount → syncedFor 是空的，失敗。
func TestGetMyOnboardingRefreshSyncsThePaymentAccountFirst(t *testing.T) {
	before, after := storeWithGate(true, false), storeWithGate(true, true)
	m := &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusApproved}, reread: after}
	pay := &paymentModule{account: payment.Account{Status: payment.StatusEnabled}}
	h := &App{m: m, payments: pay}
	got, err := h.GetMyOnboarding(ctxWithStore(before), connect.NewRequest(&pb.GetMyOnboardingRequest{Refresh: true}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pay.syncedFor, []int64{11}) || m.rereads != 1 {
		t.Fatalf("同步的店家=%v、重讀店家 %d 次", pay.syncedFor, m.rereads)
	}
	if !got.Msg.GetComplete() || taskStates(got.Msg)[pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT] != pb.OnboardingTaskState_ONBOARDING_TASK_STATE_COMPLETED {
		t.Fatalf("同步之後的條件沒有反映：complete=%v tasks=%v", got.Msg.GetComplete(), got.Msg.GetTasks())
	}

	pay = &paymentModule{account: payment.Account{Status: payment.StatusUnderReview}, syncErr: errors.New("金流商沒有回應")}
	m = &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusApproved}, reread: after}
	h = &App{m: m, payments: pay}
	got, err = h.GetMyOnboarding(ctxWithStore(before), connect.NewRequest(&pb.GetMyOnboardingRequest{Refresh: true}))
	if err != nil {
		t.Fatalf("同步失敗讓方法失敗：%v", err)
	}
	if len(pay.syncedFor) != 1 || m.rereads != 0 || got.Msg.GetComplete() {
		t.Fatalf("同步失敗時要以原本快取的狀態回傳：同步 %v、重讀 %d 次、complete=%v", pay.syncedFor, m.rereads, got.Msg.GetComplete())
	}
	if taskStates(got.Msg)[pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT] != pb.OnboardingTaskState_ONBOARDING_TASK_STATE_IN_PROGRESS {
		t.Fatalf("tasks=%v", got.Msg.GetTasks())
	}
}

// 開通任務依收款開通是否設定決定有沒有「開始設定」的下一步。
func TestGetMyOnboardingStartActionNeedsConfiguredPaymentOnboarding(t *testing.T) {
	for _, configured := range []bool{true, false} {
		h := &App{m: &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusApproved}}, payments: &paymentModule{
			configured: configured, caps: payment.Capabilities{OnboardingSurface: payment.SurfaceHosted}, account: payment.Account{Status: payment.StatusActionRequired},
		}}
		got, err := h.GetMyOnboarding(ctxWithStore(storeWithGate(true, false)), connect.NewRequest(&pb.GetMyOnboardingRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		var action pb.OnboardingAction
		for _, task := range got.Msg.GetTasks() {
			if task.GetId() == pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT {
				action = task.GetAction()
			}
		}
		want := pb.OnboardingAction_ONBOARDING_ACTION_NONE
		if configured {
			want = pb.OnboardingAction_ONBOARDING_ACTION_START_PAYMENT_ONBOARDING
		}
		if action != want {
			t.Errorf("configured=%v：action=%v，要 %v", configured, action, want)
		}
	}
}

// 後台的 GetStoreOnboarding 與賣家端是同一個函式：條件來自店家，後台讀不到草稿所以沒有申請就是「未送出」；讀取失敗只讓該項 UNKNOWN。
func TestGetStoreOnboardingUsesTheSameDerivation(t *testing.T) {
	m := &onboardingAdminModule{store: storeWithGate(true, true)}
	h := NewAdmin(m, rpc.Images{}, &paymentModule{account: payment.Account{Status: payment.StatusEnabled}}, nil)
	got, err := h.GetStoreOnboarding(adminCtx(1), connect.NewRequest(&adminpb.GetStoreOnboardingRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Msg.GetComplete() || len(got.Msg.GetTasks()) != 3 {
		t.Fatalf("complete=%v tasks=%v", got.Msg.GetComplete(), got.Msg.GetTasks())
	}

	m = &onboardingAdminModule{store: storeWithGate(false, false), kycErr: errors.New("壞了")}
	h = NewAdmin(m, rpc.Images{}, nil, nil)
	got, err = h.GetStoreOnboarding(adminCtx(1), connect.NewRequest(&adminpb.GetStoreOnboardingRequest{StoreId: "11"}))
	if err != nil {
		t.Fatalf("讀取失敗讓方法失敗：%v", err)
	}
	var review pb.OnboardingTaskState
	for _, task := range got.Msg.GetTasks() {
		if task.GetId() == pb.OnboardingTaskId_ONBOARDING_TASK_ID_PLATFORM_REVIEW {
			review = task.GetState()
		}
	}
	if review != pb.OnboardingTaskState_ONBOARDING_TASK_STATE_UNKNOWN || got.Msg.GetComplete() {
		t.Fatalf("review=%v complete=%v", review, got.Msg.GetComplete())
	}

	_, err = h.GetStoreOnboarding(adminCtx(1), connect.NewRequest(&adminpb.GetStoreOnboardingRequest{StoreId: "x"}))
	wantInvalidArgument(t, err)
}
