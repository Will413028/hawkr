package seller

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// kycReader 讀店家的 KYC 申請：found 為 false 表示沒有（不是失敗）。賣家端讀自己的申請（含草稿），後台只讀得到送出過的（草稿是賣家自己的）。
type kycReader func(ctx context.Context) (application model.KYCApplication, found bool, err error)

// readOnboarding 組出店家 store 的開通任務清單：KYC 與收款帳戶各自讀取，任何一個讀不到只讓需要它的任務是 UNKNOWN，不讓方法失敗（D26）；
// 讀取失敗記在 log。任務的完成由 store 帶的條件決定（讀店家時一併讀出），這裡讀到的只決定狀態句與下一步。
func readOnboarding(ctx context.Context, log *slog.Logger, store model.Store, kyc kycReader, payments PaymentReader) model.Onboarding {
	in := model.OnboardingInput{Store: store}
	app, found, err := kyc(ctx)
	switch {
	case err != nil:
		log.WarnContext(ctx, "httpapi/seller：開通任務讀不到 KYC 申請，平台審核標為未知", "store_id", store.StoreID, "error", err)
		in.KYC = model.OnboardingKYCRead{Err: err}
	default:
		in.KYC = model.OnboardingKYCRead{Found: found, Status: app.Status}
	}
	in.Payment = readPaymentAccount(ctx, log, store.StoreID, payments)
	return model.OnboardingTasksOf(uitext.LocaleFrom(ctx), in)
}

func readPaymentAccount(ctx context.Context, log *slog.Logger, storeID int64, payments PaymentReader) model.OnboardingPaymentRead {
	if payments == nil {
		return model.OnboardingPaymentRead{Err: errors.New("httpapi/seller：沒有收款帳戶的來源")}
	}
	acct, err := payments.Account(ctx, storeID)
	if err != nil && apperr.KindOf(err) != apperr.KindNotFound {
		log.WarnContext(ctx, "httpapi/seller：開通任務讀不到收款帳戶，收款帳戶標為未知", "store_id", storeID, "error", err)
		return model.OnboardingPaymentRead{Err: err}
	}
	caps, configured := payments.Capabilities()
	read := model.OnboardingPaymentRead{CanStartOnboarding: configured && caps.OnboardingSurface != ""}
	if err == nil {
		read.Found, read.Status = true, string(acct.Status)
	}
	return read
}

func toOnboardingTasks(o model.Onboarding) []*pb.OnboardingTask {
	out := make([]*pb.OnboardingTask, 0, len(o.Tasks))
	for _, t := range o.Tasks {
		out = append(out, &pb.OnboardingTask{
			Id:     rpc.EnumToProto[pb.OnboardingTaskId](t.ID),
			State:  rpc.EnumToProto[pb.OnboardingTaskState](t.State),
			Action: rpc.EnumToProto[pb.OnboardingAction](t.Action),
			Title:  rpc.LocalizedMessage(t.Title), Status: rpc.LocalizedMessage(t.Status),
		})
	}
	return out
}

// GetMyOnboarding 回傳呼叫者店家的開通任務。refresh 為 true 時先同步重讀金流商的收款帳戶；同步失敗只記錄，以快取的狀態繼續（不讓方法失敗）。
// 同步之後才由 payment 讀快取、由店家的條件讀 seller 自存的副本：同步在 payment 的 transaction 內發布 PayoutAccountChanged，seller 的副本在同一個
// transaction 更新，所以這裡讀到的條件已經是同步之後的；但 interceptor 在同步之前就讀了店家，所以條件要在同步之後重讀。
func (h *App) GetMyOnboarding(ctx context.Context, req *connect.Request[pb.GetMyOnboardingRequest]) (*connect.Response[pb.GetMyOnboardingResponse], error) {
	current, err := storeFrom(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetRefresh() {
		if _, err := h.payments.SyncAccount(ctx, current.StoreID); err != nil {
			h.dashboardLog().WarnContext(ctx, "httpapi/seller：開通任務同步重讀收款帳戶失敗，以快取的狀態回傳", "store_id", current.StoreID, "error", err)
		} else if current, err = h.m.Store(ctx, current.StoreID); err != nil {
			return nil, err
		}
	}
	kyc := func(ctx context.Context) (model.KYCApplication, bool, error) {
		app, err := h.m.MyKYC(ctx, current.AccountID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			return model.KYCApplication{}, false, nil
		}
		return app, err == nil, err
	}
	onboarding := readOnboarding(ctx, h.dashboardLog(), current, kyc, h.payments)
	return connect.NewResponse(&pb.GetMyOnboardingResponse{Tasks: toOnboardingTasks(onboarding), Complete: onboarding.Complete}), nil
}

// GetStoreOnboarding 回傳一家店的開通任務（後台只讀，沒有 refresh）；與賣家端是同一個函式，後台讀不到草稿。
func (h *Admin) GetStoreOnboarding(ctx context.Context, req *connect.Request[adminpb.GetStoreOnboardingRequest]) (*connect.Response[adminpb.GetStoreOnboardingResponse], error) {
	id, err := rpc.ParseID("store_id", req.Msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	store, err := h.m.Store(ctx, id)
	if err != nil {
		return nil, err
	}
	kyc := func(ctx context.Context) (model.KYCApplication, bool, error) { return h.m.StoreKYC(ctx, id) }
	onboarding := readOnboarding(ctx, h.log, store, kyc, h.payments)
	return connect.NewResponse(&adminpb.GetStoreOnboardingResponse{Tasks: toOnboardingTasks(onboarding), Complete: onboarding.Complete}), nil
}
