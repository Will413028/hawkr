package seller

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// onboardingModule 是 GetMyOnboarding 的來源：MyKYC 回 kyc（或 kycErr）。沒有實作 Store：GetMyOnboarding 只讀 interceptor 放進 ctx 的店家，
// 呼叫 Store 會因為嵌入的 Module 是 nil 而 panic。
type onboardingModule struct {
	Module
	kyc    model.KYCApplication
	kycErr error
}

func (m *onboardingModule) MyKYC(context.Context, int64) (model.KYCApplication, error) {
	return m.kyc, m.kycErr
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

var quietLog = slog.New(slog.DiscardHandler)

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
	h := &App{m: m, payments: pay, log: quietLog}
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
	if len(pay.syncedFor) != 0 {
		t.Fatalf("唯讀的方法同步了金流商：%v", pay.syncedFor)
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
			h := &App{m: m, payments: &paymentModule{err: tc.payErr}, log: quietLog}
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

// RefreshMyPaymentAccount 以呼叫者的店家同步重讀收款帳戶（D33），成功回空回應；重讀失敗原樣回錯誤（unavailable），不吞掉。
// mutation：不呼叫 SyncAccountUnlessFresh → syncedFor 是空的，失敗；失敗時回 nil → 第二段的 err 是 nil，失敗。
func TestRefreshMyPaymentAccountSyncsTheCallersStore(t *testing.T) {
	pay := &paymentModule{}
	h := &App{m: &onboardingModule{}, payments: pay, log: quietLog}
	if _, err := h.RefreshMyPaymentAccount(ctxWithStore(storeWithGate(true, false)), connect.NewRequest(&pb.RefreshMyPaymentAccountRequest{})); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(pay.syncedFor, []int64{11}) {
		t.Fatalf("同步的店家=%v，要 [11]", pay.syncedFor)
	}

	pay = &paymentModule{syncErr: apperr.Unavailable("暫時無法向金流商讀取收款帳戶")}
	h = &App{m: &onboardingModule{}, payments: pay, log: quietLog}
	_, err := h.RefreshMyPaymentAccount(ctxWithStore(storeWithGate(true, false)), connect.NewRequest(&pb.RefreshMyPaymentAccountRequest{}))
	if apperr.KindOf(err) != apperr.KindUnavailable || len(pay.syncedFor) != 1 {
		t.Fatalf("重讀失敗要回 unavailable：err=%v，同步 %v", err, pay.syncedFor)
	}
}

// 兩個方法的契約屬性：GetMyOnboarding 唯讀（NO_SIDE_EFFECTS、effect Read、停權也能讀：宣告的能力不要求未停權），
// RefreshMyPaymentAccount 是寫入（IDEMPOTENT、effect Write、停權時不能做）。
func TestOnboardingMethodsDeclareReadAndWriteCapabilities(t *testing.T) {
	read := declarations[sellerv1alpha1connect.SellerAppServiceGetMyOnboardingProcedure]
	write := declarations[sellerv1alpha1connect.SellerAppServiceRefreshMyPaymentAccountProcedure]
	if read.capability != model.CapAccountSelfRead || write.capability != model.CapStorefrontWrite {
		t.Fatalf("能力：GetMyOnboarding=%s、RefreshMyPaymentAccount=%s", read.capability, write.capability)
	}
	suspended := storeWithGate(true, true)
	suspended.Sanctions = sanctionsOf(true)
	active := map[model.Capability]bool{}
	for _, c := range model.ActiveCapabilities(model.CapabilitiesOf(suspended)) {
		active[c] = true
	}
	if active[write.capability] || !active[read.capability] {
		t.Fatalf("停權時要能讀開通任務、不能重讀收款帳戶：啟用的能力 %v", active)
	}
}

// 開通任務依收款開通是否設定決定有沒有「開始設定」的下一步。
func TestGetMyOnboardingStartActionNeedsConfiguredPaymentOnboarding(t *testing.T) {
	for _, configured := range []bool{true, false} {
		h := &App{m: &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusApproved}}, payments: &paymentModule{
			configured: configured, account: payment.Account{Status: payment.StatusActionRequired},
		}, log: quietLog}
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

func paymentTask(t *testing.T, resp *pb.GetMyOnboardingResponse) *pb.OnboardingTask {
	t.Helper()
	for _, task := range resp.GetTasks() {
		if task.GetId() == pb.OnboardingTaskId_ONBOARDING_TASK_ID_PAYMENT_ACCOUNT {
			return task
		}
	}
	t.Fatalf("沒有收款帳戶任務：%v", resp.GetTasks())
	return nil
}

// 收款帳戶讀取結果的兩個旗標進任務：建立失敗（有帳戶列、沒有外部帳戶、有 create_failed_at）給聯絡客服；設定好但還沒有帳戶列是進行中且沒有下一步。
// mutation 1：readPaymentAccount 不填 CreateFailed → 建立失敗的列變成「建立中」，失敗。
// mutation 2：不填 Configured → 設定好但還沒有帳戶列的列變成「尚未開放」，失敗。
func TestGetMyOnboardingReadsCreateFailedAndConfiguredFromPayment(t *testing.T) {
	failedAt := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		pay        *paymentModule
		wantState  pb.OnboardingTaskState
		wantAction pb.OnboardingAction
	}{
		{"建立失敗", &paymentModule{configured: true, account: payment.Account{Status: payment.StatusNotCreated, CreateFailedAt: &failedAt}},
			pb.OnboardingTaskState_ONBOARDING_TASK_STATE_ACTION_REQUIRED, pb.OnboardingAction_ONBOARDING_ACTION_CONTACT_SUPPORT},
		{"設定好、還沒有帳戶列", &paymentModule{configured: true, err: apperr.NotFound("沒有")},
			pb.OnboardingTaskState_ONBOARDING_TASK_STATE_IN_PROGRESS, pb.OnboardingAction_ONBOARDING_ACTION_NONE},
		{"沒有設定收款開通、沒有帳戶列", &paymentModule{err: apperr.NotFound("沒有")},
			pb.OnboardingTaskState_ONBOARDING_TASK_STATE_IN_PROGRESS, pb.OnboardingAction_ONBOARDING_ACTION_NONE},
	}
	for _, tc := range cases {
		h := &App{m: &onboardingModule{kyc: model.KYCApplication{Status: model.KYCStatusApproved}}, payments: tc.pay, log: quietLog}
		got, err := h.GetMyOnboarding(ctxWithStore(storeWithGate(true, false)), connect.NewRequest(&pb.GetMyOnboardingRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		task := paymentTask(t, got.Msg)
		if task.GetState() != tc.wantState || task.GetAction() != tc.wantAction {
			t.Errorf("%s：state=%v action=%v，要 %v %v", tc.name, task.GetState(), task.GetAction(), tc.wantState, tc.wantAction)
		}
	}
}

// 停權的店家可以讀開通任務，但需要 CAP_STOREFRONT_WRITE 的下一步不給（NONE）；狀態不變。
// mutation：OnboardingTasksOf 不看 Store.Can → 停權的列照給 SUBMIT_CERTIFICATION，失敗。
func TestGetMyOnboardingGivesNoWriteActionsToASuspendedStore(t *testing.T) {
	store := storeWithGate(false, false)
	store.Sanctions = sanctionsOf(true)
	h := &App{m: &onboardingModule{}, payments: &paymentModule{configured: true}, log: quietLog}
	got, err := h.GetMyOnboarding(ctxWithStore(store), connect.NewRequest(&pb.GetMyOnboardingRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range got.Msg.GetTasks() {
		if task.GetAction() != pb.OnboardingAction_ONBOARDING_ACTION_NONE {
			t.Errorf("停權的店家 %v 的下一步是 %v，要 NONE", task.GetId(), task.GetAction())
		}
	}
}

// GetMyOnboarding 的 log 是 App 自己的，不經儀表板的接線（Dashboard.Log 沒設也要記到）。
// mutation：改回 h.dashboardLog() → 這個測試的 buffer 是空的，失敗。
func TestGetMyOnboardingLogsToItsOwnLogger(t *testing.T) {
	var buf bytes.Buffer
	h := &App{m: &onboardingModule{kycErr: errors.New("壞了")}, payments: &paymentModule{}, log: slog.New(slog.NewTextHandler(&buf, nil))}
	if _, err := h.GetMyOnboarding(ctxWithStore(storeWithGate(false, false)), connect.NewRequest(&pb.GetMyOnboardingRequest{})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "KYC") {
		t.Fatalf("沒有記到 App 的 logger：%q", buf.String())
	}
}
