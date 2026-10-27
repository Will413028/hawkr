package grpc

import (
	"context"
	"errors"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// 本檔是 M4「AMEGO 帳戶」寫入面的 Layer 5：賣家側 §6.1.8 的端點 24／25／26／27，
// ADM 側 §6.1.13 的端點 44／45／66／67。讀取面（端點 23／42／43）在 query.go。
//
// 四條規則：
//
//	① **業務錯誤走 envelope、故障走 gRPC error**。GetCode() 是 Gateway 的
//	   EnvelopeGuardInterceptor 唯一的判定依據，漏了會靜默當成功。
//	② **9331 帶 data**（§6.1.8 端點 25：「200 ＋ preserveData」）：驗證失敗時前端要能
//	   同時拿到「失敗了」與「現在的狀態是 INVALID_CREDENTIAL」，少了 data 就只剩一句紅字。
//	   9332 走 envelopeCodeToGRPC 的 fallback ⇒ HTTP 409 ＋ 正確業務碼（**不是 503**：
//	   codes.Unavailable 會把業務碼吃掉，而端點 25 的整個設計建立在 9331／9332 可區分之上）。
//	③ **回應永遠不包含 AppKey，任何角色都不行**。這由 proto 的 reserved 90 保證（加回去
//	   是 buf build 失敗，不是靜默外洩），本層不需要、也不得再做一次遮罩。
//	④ **長度／必填／格式是契約違規不是業務結果** ⇒ InvalidArgument（400），不借業務碼。

// PutSellerProviderCredentials 是端點 24（憑證寫入，write-only）。
func (s *InvoiceServer) PutSellerProviderCredentials(
	ctx context.Context, req *pb.PutSellerProviderCredentialsRequest,
) (*pb.SellerProviderAccountResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	return s.providerAccount(ctx, "invoice.provider.put_credentials.failed", func() (usecase.ProviderAccountView, error) {
		return s.providerAccounts.PutCredentials(ctx, usecase.PutProviderCredentialsInput{
			StoreID:         storeID,
			Provider:        req.GetProvider(),
			SellerTaxID:     req.GetSellerTaxId(),
			AppKey:          req.GetAppKey(),
			ConsentVersion:  req.GetConsentVersion(),
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
		})
	})
}

// VerifySellerProviderAccount 是端點 25（測試連線）。
//
// 它是全書三支「在請求路徑上同步等 AMEGO」之一 ⇒ gateway deadline 走
// INVOICE_GRPC_DEADLINE_CREDENTIAL_TEST_MS（15 s）。本層不自己設 deadline：
// 設了會與 gateway 那一份漂移，而漂移的方向是「前端顯示失敗、憑證其實驗過了」。
func (s *InvoiceServer) VerifySellerProviderAccount(
	ctx context.Context, req *pb.VerifySellerProviderAccountRequest,
) (*pb.SellerProviderAccountResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	return s.providerAccount(ctx, "invoice.provider.verify.failed", func() (usecase.ProviderAccountView, error) {
		return s.providerAccounts.Verify(ctx, usecase.VerifyProviderAccountInput{
			StoreID:         storeID,
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
		})
	})
}

// ActivateSellerProviderAccount 是端點 26（確認啟用切點）。
func (s *InvoiceServer) ActivateSellerProviderAccount(
	ctx context.Context, req *pb.ActivateSellerProviderAccountRequest,
) (*pb.SellerProviderAccountResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	cutoverAt, err := parseISO8601(req.GetCutoverAt())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "cutover_at: %v", err)
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	return s.providerAccount(ctx, "invoice.provider.activate.failed", func() (usecase.ProviderAccountView, error) {
		return s.providerAccounts.Activate(ctx, usecase.ActivateProviderAccountInput{
			StoreID:         storeID,
			CutoverAt:       cutoverAt,
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
		})
	})
}

// RotateSellerProviderCredentials 是端點 27（憑證輪替）。
func (s *InvoiceServer) RotateSellerProviderCredentials(
	ctx context.Context, req *pb.RotateSellerProviderCredentialsRequest,
) (*pb.SellerProviderAccountResponse, error) {
	storeID, err := requirePrincipal(req.GetStoreId(), "store_id")
	if err != nil {
		return nil, err
	}
	version, err := parseVersion(req.GetExpectedVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	return s.providerAccount(ctx, "invoice.provider.rotate.failed", func() (usecase.ProviderAccountView, error) {
		return s.providerAccounts.RotateCredentials(ctx, usecase.RotateProviderCredentialsInput{
			StoreID:         storeID,
			AppKey:          req.GetAppKey(),
			ExpectedVersion: version,
			IdempotencyKey:  req.GetIdempotencyKey(),
		})
	})
}

// HealthCheckProviderAccountForAdmin 是端點 44（ADM 主動連線檢查）。
//
// ⚠️ 本支**沒有 expectedVersion**（proto 逐字）：ADM 按的是「再測一次」，它不覆蓋任何
// 賣家填的東西。回應型別是 GetProviderAccountForAdminResponse（與端點 43 同一個 data）。
func (s *InvoiceServer) HealthCheckProviderAccountForAdmin(
	ctx context.Context, req *pb.HealthCheckProviderAccountForAdminRequest,
) (*pb.GetProviderAccountForAdminResponse, error) {
	actor, err := requirePrincipal(req.GetActorAccountId(), "actor_account_id")
	if err != nil {
		return nil, err
	}
	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	view, err := s.providerAccounts.HealthCheckForAdmin(ctx, usecase.AdminProviderHealthCheckInput{
		ActorAccountID: actor,
		StoreID:        storeID,
		IdempotencyKey: req.GetIdempotencyKey(),
	})
	if err != nil {
		if st, ok := providerInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			// 9331／9332 帶 data：ADM 要同時看到「失敗了」與「現在的狀態」。
			// view 在業務失敗時仍是有效的（usecase 先落地狀態才回碼）。
			return &pb.GetProviderAccountForAdminResponse{
				Code: code, Message: msg, Data: toProviderAccountData(view),
			}, nil
		}
		return nil, s.internal(ctx, "invoice.provider.health_check.failed", err)
	}
	return &pb.GetProviderAccountForAdminResponse{
		Code: 0, Message: msgSuccess, Data: toProviderAccountData(view),
	}, nil
}

// ReactivateProviderAccountForAdmin 是端點 66（恢復連線 ＝ P15）。
//
// 🔴 恢復前**強制重驗**，不得直接回 ACTIVE：所以它與端點 44 一樣同步等 AMEGO，
// deadline 走同一個 15 s。
func (s *InvoiceServer) ReactivateProviderAccountForAdmin(
	ctx context.Context, req *pb.ReactivateProviderAccountForAdminRequest,
) (*pb.GetProviderAccountForAdminResponse, error) {
	in, err := adminProviderActionInput(req.GetActorAccountId(), req.GetStoreId(),
		req.GetReason(), req.GetConfirmed(), req.GetExpectedVersion(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	view, err := s.providerAccounts.ReactivateForAdmin(ctx, in)
	if err != nil {
		if st, ok := providerInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.GetProviderAccountForAdminResponse{
				Code: code, Message: msg, Data: toProviderAccountData(view),
			}, nil
		}
		return nil, s.internal(ctx, "invoice.provider.reactivate.failed", err)
	}
	return &pb.GetProviderAccountForAdminResponse{
		Code: 0, Message: msgSuccess, Data: toProviderAccountData(view),
	}, nil
}

// DeactivateProviderAccountForAdmin 是端點 45（停用連線 ＝ P14）。可逆。
func (s *InvoiceServer) DeactivateProviderAccountForAdmin(
	ctx context.Context, req *pb.DeactivateProviderAccountForAdminRequest,
) (*pb.DeactivateProviderAccountForAdminResponse, error) {
	in, err := adminProviderActionInput(req.GetActorAccountId(), req.GetStoreId(),
		req.GetReason(), req.GetConfirmed(), req.GetExpectedVersion(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	res, err := s.providerAccounts.DeactivateForAdmin(ctx, in)
	if err != nil {
		if st, ok := providerInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.DeactivateProviderAccountForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.provider.deactivate.failed", err)
	}
	return &pb.DeactivateProviderAccountForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.DeactivateProviderAccountForAdminData{
			StoreId:             fmtInt64(res.StoreID),
			Status:              res.Status,
			DeactivatedAt:       fmtTimeValue(res.OccurredAt),
			AffectedScopeNotice: res.AffectedScope,
			PendingOrderCount:   fmtInt64(res.PendingOrderCount),
			AuditLogId:          fmtID(res.AuditLogID),
		},
	}, nil
}

// RevokeProviderAccountForAdmin 是端點 67（撤銷 ＝ P16）。**不可逆**。
//
// 🔴 破壞性防護的逐條清單寫在 usecase.ProviderAccount.RevokeForAdmin 的註解上，
// 而**每一條的執行點都不在本層**：本層只負責把 confirmed／reason／expectedVersion
// 從 wire 型別轉進去。特別是「查不到就當可以撤銷」這個分支在本層與 usecase 都不存在——
// loadAccount 查無一律回 9330 並中止。
func (s *InvoiceServer) RevokeProviderAccountForAdmin(
	ctx context.Context, req *pb.RevokeProviderAccountForAdminRequest,
) (*pb.RevokeProviderAccountForAdminResponse, error) {
	in, err := adminProviderActionInput(req.GetActorAccountId(), req.GetStoreId(),
		req.GetReason(), req.GetConfirmed(), req.GetExpectedVersion(), req.GetIdempotencyKey())
	if err != nil {
		return nil, err
	}
	res, err := s.providerAccounts.RevokeForAdmin(ctx, in)
	if err != nil {
		if st, ok := providerInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.RevokeProviderAccountForAdminResponse{Code: code, Message: msg}, nil
		}
		return nil, s.internal(ctx, "invoice.provider.revoke.failed", err)
	}
	return &pb.RevokeProviderAccountForAdminResponse{
		Code: 0, Message: msgSuccess,
		Data: &pb.RevokeProviderAccountForAdminData{
			StoreId:             fmtInt64(res.StoreID),
			Status:              res.Status,
			RevokedAt:           fmtTimeValue(res.OccurredAt),
			AffectedScopeNotice: res.AffectedScope,
			PendingOrderCount:   fmtInt64(res.PendingOrderCount),
			AuditLogId:          fmtID(res.AuditLogID),
		},
	}, nil
}

// providerAccount 是四支賣家端點共用的錯誤分流與回應組裝。
//
// 🔴 業務失敗**仍然帶 data**：usecase 先把狀態落地才回碼（見 verifyFlow 的末段），
// 所以 view 在這一格是有效的。少了 data，S03 顯示「驗證失敗」卻不知道現在是
// INVALID_CREDENTIAL（要重打 AppKey）還是 PROVIDER_UNAVAILABLE（不要動 AppKey）。
func (s *InvoiceServer) providerAccount(
	ctx context.Context, event string, run func() (usecase.ProviderAccountView, error),
) (*pb.SellerProviderAccountResponse, error) {
	view, err := run()
	if err != nil {
		if st, ok := providerInvalidArgument(err); ok {
			return nil, st
		}
		if code, msg, ok := bizOf(err); ok {
			return &pb.SellerProviderAccountResponse{
				Code: code, Message: msg, Data: toProviderAccountData(view),
			}, nil
		}
		return nil, s.internal(ctx, event, err)
	}
	return &pb.SellerProviderAccountResponse{
		Code: 0, Message: msgSuccess, Data: toProviderAccountData(view),
	}, nil
}

// adminProviderActionInput 解析端點 45／66／67 的共通入參。
//
// confirmed／reason 的檢查在 usecase（那是業務守衛的一部分，必須與 CAS 在同一層被測到）；
// 本函式只做型別邊界。
func adminProviderActionInput(
	actorAccountID int64, storeID, reason string, confirmed bool, expectedVersion, idempotencyKey string,
) (usecase.AdminProviderActionInput, error) {
	actor, err := requirePrincipal(actorAccountID, "actor_account_id")
	if err != nil {
		return usecase.AdminProviderActionInput{}, err
	}
	id, err := parseID(storeID)
	if err != nil {
		return usecase.AdminProviderActionInput{}, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	version, err := parseVersion(expectedVersion)
	if err != nil {
		return usecase.AdminProviderActionInput{}, status.Errorf(codes.InvalidArgument, "expected_version: %v", err)
	}
	return usecase.AdminProviderActionInput{
		ActorAccountID:  actor,
		StoreID:         id,
		Reason:          reason,
		Confirmed:       confirmed,
		ExpectedVersion: version,
		IdempotencyKey:  idempotencyKey,
	}, nil
}

// providerInvalidArgument 把 usecase 的型別邊界錯誤映成 400。
//
// 它們刻意不是 DomainError：沒有任何業務碼描述「你的統編少一碼」，而 bizOf 會把
// DomainError 折成 envelope 的業務碼。
func providerInvalidArgument(err error) (error, bool) {
	switch {
	case errors.Is(err, usecase.ErrProviderUnsupported),
		errors.Is(err, usecase.ErrProviderTaxIDInvalid),
		errors.Is(err, usecase.ErrProviderAppKeyRequired),
		errors.Is(err, usecase.ErrProviderAppKeyTooLong),
		errors.Is(err, usecase.ErrProviderCutoverInvalid),
		errors.Is(err, usecase.ErrProviderCutoverInPast),
		errors.Is(err, usecase.ErrProviderReasonRequired),
		errors.Is(err, usecase.ErrProviderReasonTooLong),
		errors.Is(err, usecase.ErrProviderConfirmRequired),
		errors.Is(err, usecase.ErrProviderConsentRequired),
		errors.Is(err, usecase.ErrProviderConsentTooLong):
		return status.Errorf(codes.InvalidArgument, "%v", err), true
	}
	return nil, false
}

// parseISO8601 解析對外時間欄（§4：ISO 8601 UTC）。
//
// 🔴 只收帶時區的形式並一律轉 UTC：收一個沒有時區的字串等於讓「切點」的意義取決於
// 送出者所在的時區，而切點決定的是哪些訂單走自動開票。
func parseISO8601(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errEmptyID
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
