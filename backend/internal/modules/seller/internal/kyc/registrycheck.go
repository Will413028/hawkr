package kyc

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
)

// 公司型申請送件時的登記資料查詢快照（ADR 2026-10-19-seller-kyc-payuni-consolidated「登記資料快照」）。流程（與 payuni 的 sync 同一個
// 三段式，backend AGENTS.md 規則 5）：
//
//  1. 送件（或後台重新查詢）的 transaction 內新增一列 PENDING 的查詢（kyc_registration_checks，綁定這一輪送件的 submission_seq）並
//     排入 job，args 只有 check_id；
//  2. job 在 transaction 內認領（讀查詢列與申請要比對的欄位；申請已不是這一輪審核中、或同一輪已有更新的查詢就寫 CANCELLED 並結束），
//     transaction 外呼叫資料來源（有自己的 deadline），
//  3. 再用另一個 transaction 寫入結果與比對；寫入只在查詢仍是 PENDING 時發生，所以重複投遞、重試與先後完成的多筆查詢都不會覆寫
//     已寫入的結果，也不依賴順序：後台顯示的是申請目前這一輪（submission_seq）seq 最大的已完成查詢。
//
// job 以 check_id 為 River 的唯一鍵（同一筆查詢同時最多一個排隊中的 job），讓掃描可以放心補排；重複投遞的冪等仍由「仍是 PENDING
// 才寫」保證。
//
// 查詢是有期限的資源（AIP-151 的語意：狀態放在資源上，終態由伺服器保證）：建立查詢時寫入 deadline_at（requested_at 加
// registryDeadline），後台的 pending_registration_check.deadline_time 讀這個欄位。伺服器保證它在 deadline 之前到終態。PENDING 的出口：
//   - job 寫入結果（最後一次嘗試的任何失敗都記成 UNAVAILABLE）；job 執行時已過期限就直接記 UNAVAILABLE、不再呼叫資料來源；
//   - 認領時放棄（CANCELLED）；
//   - 後台重新查詢把這一輪的 PENDING 全部取代（CANCELLED）；
//   - 掃描 job（seller.kyc_registry_sweep，每 registrySweepInterval 一次）：deadline_at 不晚於「現在加一個掃描間隔」的 PENDING 收成終態
//     （仍屬審核中這一輪的記成 UNAVAILABLE，其餘 CANCELLED），所以最晚在期限之前的最後一次掃描收掉；閒置超過 registryResyncIdle 的
//     PENDING 補排 job（程序當掉或 River 丟棄 job 之後 job 不存在；唯一鍵讓仍在排隊、執行、等重試的 job 不會重複）。
//
// River 的重試在期限內照舊進行；結束的時間只由 deadline 與掃描決定，registryMaxAttempts 只是防止無窮重試的上限。後台重新查詢仍然是
// 「現在就要一筆新的」的出口（取代進行中的）。
//
// 比對只記錄（mismatched_fields、unverifiable_fields），不改申請狀態；結果是審核員的依據，不是自動決定。

// Registry 是登記資料來源的 port：真正的實作是 platform/gcis 的 Client（回來源的原始資料，轉成模組的型別只有 model.NewRegistration
// 一處），測試用替身。found 為 false 且沒有錯誤是查無；來源這一次沒有給出可用的答案回 gcis.ErrUnavailable，統編格式不合回
// gcis.ErrInvalidTaxID。外部呼叫，不可在資料庫 transaction 內呼叫。
type Registry interface {
	Lookup(ctx context.Context, taxID string) (reg gcis.Registration, found bool, err error)
}

const (
	// registryCallTimeout 是 job 內呼叫資料來源的上限（client 自己也有總 deadline，這是再多一層）。
	registryCallTimeout = 15 * time.Second
	// registryJobTimeout 是一次 job 的上限。
	registryJobTimeout = 60 * time.Second
	// registryWriteTimeout 是寫結果的 transaction 的上限（job 的 ctx 逾時被取消後仍要能寫入）。
	registryWriteTimeout = 15 * time.Second
	// registryMaxAttempts 只防重試失控：終態由 deadline_at 保證（逾期的 job 下一次執行直接記 UNAVAILABLE，掃描收掉沒有 job 的逾期查詢，見
	// registryDeadline）；次數用盡時仍把查詢記為 UNAVAILABLE，是這個保證之外的保險，不是結束重試的時間來源。
	registryMaxAttempts = 8
	// registryDeadline 是一筆查詢從被要求到伺服器保證終態的時間。業界對互動情境（審核員按下去等結果）的做法是幾分鐘內有結果；
	// 資料來源（GCIS）沒有 SLA，所以不等它：有掃描 job 保證終態、審核員隨時可以重新查詢，逾時收成 UNAVAILABLE 讓人重查，比等一個
	// 可能一個多小時才放棄的重試鏈合理。15 分鐘涵蓋 River 前幾次重試（退避 1、16、81、256 秒），來源短暫失靈時仍有機會自己恢復。
	registryDeadline = 15 * time.Minute
	// registrySweepInterval 是掃描 job 的間隔；掃描提前一個間隔收掉過期的查詢，所以終態一定在 deadline 之前。
	registrySweepInterval = time.Minute
	// registryResyncIdle 是 PENDING 閒置多久之後由掃描補排 job（保底，平常 job 一定存在；唯一鍵讓還在跑的不會重複）。
	registryResyncIdle = 3 * time.Minute
	// registrySweepBatch 是掃描一次補排的上限。
	registrySweepBatch = 100
)

// registryCheckDeadline 是一筆查詢的期限。
func registryCheckDeadline(requestedAt time.Time) time.Time { return requestedAt.Add(registryDeadline) }

// 唯一鍵涵蓋「還沒有結果」的狀態（排隊中、執行中、等重試）；不含 completed，也不含 discarded／cancelled：被丟棄的 job 之後要能補排。
func registryOpenStates() []rivertype.JobState {
	return []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning, rivertype.JobStateRetryable,
		rivertype.JobStateScheduled,
	}
}

// RegistryCheckArgs 是一次登記資料查詢的 job，以 check_id 為唯一鍵。
type RegistryCheckArgs struct {
	CheckID uuid.UUID `json:"check_id" river:"unique"`
}

// Kind 是 River 的 job 種類。
func (RegistryCheckArgs) Kind() string { return "seller.kyc_registry_check" }

// InsertOpts 是排入選項：以 check_id 去重（不含已完成的），重試上限 registryMaxAttempts 只防失控。
func (RegistryCheckArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: registryMaxAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: registryOpenStates()}}
}

// enqueueRegistryCheck 排入查詢 job；inserted 為 false 表示被唯一鍵略過（同一筆查詢還有排隊、執行或等重試的 job）。
func (s *Service) enqueueRegistryCheck(ctx context.Context, checkID uuid.UUID) (inserted bool, err error) {
	args := RegistryCheckArgs{CheckID: checkID}
	opts := args.InsertOpts()
	return s.d.Jobs.EnqueueReport(ctx, args, &opts)
}

// requestRegistryCheck 新增一列 PENDING 的查詢並排入 job；呼叫方在 transaction 內、已持有申請列的鎖（seq 因此不會重複）。
func (s *Service) requestRegistryCheck(ctx context.Context, q *store.Queries, applicationID uuid.UUID, kind model.RegistrationRequestKind, adminID *int64, now time.Time) (store.SellerKycRegistrationCheck, error) {
	row, err := q.InsertKYCRegistrationCheck(ctx, store.InsertKYCRegistrationCheckParams{
		CheckID: uuid.NewV7(), ApplicationID: applicationID, RequestKind: string(kind), RequestedByAdminID: adminID, Now: now,
		DeadlineAt: registryCheckDeadline(now),
	})
	if err != nil {
		return store.SellerKycRegistrationCheck{}, err
	}
	_, err = s.enqueueRegistryCheck(ctx, row.CheckID)
	return row, err
}

// RequeryRegistration 是後台對審核中的公司型申請重新查詢登記資料：在同一個 transaction 內把這一輪還沒查完的查詢全部取代
// （CANCELLED），再新增一筆並排入 job，回應不等結果。所以任何原因留下的 PENDING（程序當掉、job 被丟棄）都能由它收掉；被取代的 job
// 之後認領時看到它已不是 PENDING，直接結束。申請不是 SUBMITTED 或不是公司型回 FailedPrecondition，不存在或是草稿回 NotFound。鎖與
// 核准、退回相同（帳號全部的 stint 先、申請列後），所以與它們序列化：重新查詢不會晚於狀態改變。回傳新增的這筆進行中的查詢（帶期限）。
func (s *Service) RequeryRegistration(ctx context.Context, in model.RequeryKYCRegistrationInput) (model.RegistrationCheck, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.RegistrationCheck{}, err
	}
	var out model.RegistrationCheck
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, row, err := s.lockApplication(ctx, q, in.ApplicationID)
		if err != nil {
			return err
		}
		reasons, err := s.reviewReasons(ctx, q, locked, row, model.StoreActionRequeryKYC)
		if err != nil {
			return err
		}
		if len(reasons) > 0 {
			return model.ErrStoreActionUnavailable(reasons, model.StoreActionRequeryKYC)
		}
		now := s.d.Now().UTC()
		if _, err := q.SupersedePendingKYCRegistrationChecks(ctx, store.SupersedePendingKYCRegistrationChecksParams{ApplicationID: in.ApplicationID, Now: &now}); err != nil {
			return err
		}
		row2, err := s.requestRegistryCheck(ctx, q, in.ApplicationID, model.RegistrationRequestAdminRequery, &in.AdminID, now)
		if err != nil {
			return err
		}
		out = toPendingCheck(row2)
		return nil
	})
	return out, err
}

// SweepRegistryChecks 是掃描 job 的內容（給 worker 與測試呼叫）：先把期限在「現在加一個掃描間隔」之前還在 PENDING 的查詢收成終態（保證終態），
// 再把閒置超過 registryResyncIdle 的 PENDING 補排 job（job 遺失、當掉或被丟棄時的保底；唯一鍵讓還在排隊、執行或等重試的 job 不會重複）。
// 回傳收掉的查詢數與實際排入的補排數（被唯一鍵略過的不算）。
func (s *Service) SweepRegistryChecks(ctx context.Context) (expired int64, resynced int, err error) {
	now := s.d.Now().UTC()
	q := store.New(s.d.DB.Conn(ctx))
	expired, err = q.ExpireKYCRegistrationChecks(ctx, store.ExpireKYCRegistrationChecksParams{Now: &now, DeadlineBefore: now.Add(registrySweepInterval)})
	if err != nil {
		return 0, 0, err
	}
	if expired > 0 {
		s.d.Logger.WarnContext(ctx, "seller：登記資料查詢超過期限仍未查完，已收成終態", "count", expired)
	}
	ids, err := q.KYCRegistrationChecksToResync(ctx, store.KYCRegistrationChecksToResyncParams{IdleBefore: now.Add(-registryResyncIdle), BatchSize: registrySweepBatch})
	if err != nil {
		return expired, 0, err
	}
	for _, id := range ids {
		var inserted bool
		if err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
			var err error
			inserted, err = s.enqueueRegistryCheck(ctx, id)
			return err
		}); err != nil {
			return expired, resynced, err
		}
		if inserted {
			resynced++
		}
	}
	return expired, resynced, nil
}

// RegistrySweepArgs 是定期的掃描 job（補排遺失的查詢 job、把過期的 PENDING 收成終態）。
type RegistrySweepArgs struct{}

// Kind 是 River 的 job 種類。
func (RegistrySweepArgs) Kind() string { return "seller.kyc_registry_sweep" }

// InsertOpts 把同一段時間內的排程與重啟合併。
func (RegistrySweepArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByPeriod: registrySweepInterval / 2}}
}

type registrySweepWorker struct {
	river.WorkerDefaults[RegistrySweepArgs]
	service *Service
}

// Work 執行掃描。
func (w *registrySweepWorker) Work(ctx context.Context, _ *river.Job[RegistrySweepArgs]) error {
	_, _, err := w.service.SweepRegistryChecks(ctx)
	return err
}

// Timeout 是一次掃描的時間上限。
func (w *registrySweepWorker) Timeout(*river.Job[RegistrySweepArgs]) time.Duration {
	return 2 * time.Minute
}

// PeriodicJobs 回傳啟動即執行、每 registrySweepInterval 的掃描。
func (s *Service) PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{river.NewPeriodicJob(
		river.PeriodicInterval(registrySweepInterval),
		func() (river.JobArgs, *river.InsertOpts) { return RegistrySweepArgs{}, nil },
		&river.PeriodicJobOpts{ID: "seller.kyc_registry_sweep", RunOnStart: true},
	)}
}

// LookupRegistration 是互動的統編查詢（賣家填表時帶入公司資料）：呼叫資料來源並轉成模組的型別（與送件的查詢快照同一個入口
// model.NewRegistration）。外部呼叫，不可在 transaction 內；沒有 deadline 以外的重試。查無是 found 為 false；來源沒有給出可用的答案
// 回 Unavailable。統編的格式與檢查碼由呼叫方先驗證（model.NormalizeTaxID）。
func (s *Service) LookupRegistration(ctx context.Context, taxID string) (model.Registration, bool, error) {
	if _, ok := db.TxFrom(ctx); ok {
		return model.Registration{}, false, apperr.FailedPrecondition("統編查詢必須在 transaction 外")
	}
	callCtx, cancel := context.WithTimeout(ctx, registryCallTimeout)
	defer cancel()
	src, found, err := s.d.Registry.Lookup(callCtx, taxID)
	switch {
	case errors.Is(err, gcis.ErrUnavailable):
		return model.Registration{}, false, apperr.Unavailable("統一編號查詢暫時無法使用，請稍後再試或手動填寫")
	case err != nil:
		return model.Registration{}, false, err
	case !found:
		return model.Registration{}, false, nil
	}
	return model.NewRegistration(src), true, nil
}

// registryClaim 是 job 認領時讀到的：要比對的申請欄位，與要查的統編。
type registryClaim struct {
	check   store.ClaimKYCRegistrationCheckRow
	subject model.RegistrationSubject
}

// RunRegistryCheck 執行一次登記資料查詢（job 的內容）。查詢不是 PENDING（已寫入、或個資清理刪掉了它）就結束；申請已經不是這一輪
// 送件審核中、或同一輪已有更新的查詢時，認領就把它寫成 CANCELLED 並結束（見 claimRegistryCheck）。來源暫時不可用、呼叫方取消或沒有分類的錯誤，在不是最後
// 一次嘗試時回傳錯誤讓 River 重試；final（最後一次嘗試）時一律把查詢記為 UNAVAILABLE，不留 PENDING。統編格式被來源拒絕是不變量被破壞（申請上的統編在草稿時已驗證）：記 error 並寫 UNAVAILABLE，
// 重試不會改變結果。
func (s *Service) RunRegistryCheck(ctx context.Context, checkID uuid.UUID, final bool) error {
	claim, ok, err := s.claimRegistryCheck(ctx, checkID)
	if err != nil || !ok {
		return err
	}
	if !s.d.Now().UTC().Before(claim.check.DeadlineAt) {
		// 期限已到（重試拖過了期限，或 job 很晚才醒來）：資料來源沒有在期限內給出答案，直接收成終態，不再呼叫它。
		s.d.Logger.WarnContext(ctx, "seller：登記資料查詢已過期限，記為不可用、不再呼叫資料來源", "check_id", checkID, "deadline", claim.check.DeadlineAt)
		return s.writeRegistryResult(ctx, claim, model.RegistrationUnavailable, model.Registration{})
	}
	taxID := *claim.check.TaxID
	callCtx, cancel := context.WithTimeout(ctx, registryCallTimeout)
	src, found, lookupErr := s.d.Registry.Lookup(callCtx, taxID)
	cancel()
	switch {
	case errors.Is(lookupErr, gcis.ErrInvalidTaxID):
		s.d.Logger.ErrorContext(ctx, "seller：登記資料來源拒絕申請上的統編格式，查詢記為不可用", "check_id", checkID)
		return s.writeRegistryResult(ctx, claim, model.RegistrationUnavailable, model.Registration{})
	case lookupErr != nil:
		if !final {
			return fmt.Errorf("seller：登記資料來源這次沒有給出答案，稍後重試：%w", lookupErr)
		}
		s.d.Logger.WarnContext(ctx, "seller：登記資料查詢重試用盡，記為不可用", "check_id", checkID, "error", lookupErr)
		return s.writeRegistryResult(ctx, claim, model.RegistrationUnavailable, model.Registration{})
	case !found:
		return s.writeRegistryResult(ctx, claim, model.RegistrationNotFound, model.Registration{})
	}
	return s.writeRegistryResult(ctx, claim, model.RegistrationFound, model.NewRegistration(src))
}

// claimRegistryCheck 在 transaction 內讀查詢列與申請的比對欄位；查詢列不存在（個資清理已刪除）或不是 PENDING 時 ok 為 false。
// 申請不是 SUBMITTED、申請的輪次（submission_seq）與查詢綁定的不同（退回後重送過）、同一輪已有更新的查詢、申請已不是公司型或沒有
// 統編時，這筆查詢不屬於任何審核中的送件：在同一個 transaction 內寫成 CANCELLED 並結束（不是錯誤），否則它會拿草稿比對、之後成為下一輪的快照。
func (s *Service) claimRegistryCheck(ctx context.Context, checkID uuid.UUID) (out registryClaim, ok bool, err error) {
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		row, err := q.ClaimKYCRegistrationCheck(ctx, checkID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.Outcome != string(model.RegistrationPending) {
			return nil
		}
		current := row.ApplicationStatus == string(model.KYCStatusSubmitted) && row.ApplicationSubmissionSeq == row.CheckSubmissionSeq &&
			!row.Superseded && row.SellerType == string(model.KYCCompany) && row.TaxID != nil
		if !current {
			s.d.Logger.InfoContext(ctx, "seller：登記資料查詢不屬於審核中的這一輪送件或已被更新的取代，放棄", "check_id", checkID, "application_status", row.ApplicationStatus)
			now := s.d.Now().UTC()
			_, err := q.CompleteKYCRegistrationCheck(ctx, store.CompleteKYCRegistrationCheckParams{
				CheckID: checkID, Outcome: string(model.RegistrationCancelled), QueriedAt: &now, MismatchedFields: []string{}, UnverifiableFields: []string{},
			})
			return err
		}
		out = registryClaim{check: row, subject: model.RegistrationSubject{
			CompanyName: row.CompanyName, TownCode: row.CompanyTownCode, AddressDetail: row.CompanyAddressDetail,
			RegisterDate: row.CompanyRegisterDate,
		}}
		ok = true
		return nil
	})
	return out, ok, err
}

// writeRegistryResult 在一個 transaction 內寫入結果與比對（FOUND 才有）。ctx 逾時被取消之後仍要能寫入，所以用自己的 deadline。
// 查詢已經不是 PENDING（重複投遞或別的 job 先寫了）或列被個資清理刪除時，不寫入、不是錯誤。
func (s *Service) writeRegistryResult(ctx context.Context, claim registryClaim, outcome model.RegistrationOutcome, reg model.Registration) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), registryWriteTimeout)
	defer cancel()
	now := s.d.Now().UTC()
	params := store.CompleteKYCRegistrationCheckParams{
		CheckID: claim.check.CheckID, Outcome: string(outcome), QueriedAt: &now, MismatchedFields: []string{}, UnverifiableFields: []string{},
	}
	if outcome == model.RegistrationFound {
		params.TaxID, params.Name, params.AddressText = &reg.TaxID, &reg.Name, &reg.Address
		if reg.TownCode != "" {
			params.TownCode, params.AddressDetail = &reg.TownCode, &reg.AddressDetail
		}
		params.SetupDate = dateOrNil(reg.SetupDate)
		if reg.OrganizationType != "" {
			params.OrganizationType = &reg.OrganizationType
		}
		status := string(reg.Status)
		params.Status, params.StatusText = &status, &reg.StatusText
		params.SuspendedFrom, params.SuspendedUntil = dateOrNil(reg.SuspendedFrom), dateOrNil(reg.SuspendedUntil)
		params.MismatchedFields, params.UnverifiableFields = model.CompareRegistration(claim.subject, reg)
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).CompleteKYCRegistrationCheck(ctx, params)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
}

func dateOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

type registryCheckWorker struct {
	river.WorkerDefaults[RegistryCheckArgs]
	service *Service
}

// Work 執行一次查詢；最後一次嘗試的任何失敗都由 RunRegistryCheck 記錄成結果，不留 PENDING。
func (w *registryCheckWorker) Work(ctx context.Context, job *river.Job[RegistryCheckArgs]) error {
	return w.service.RunRegistryCheck(ctx, job.Args.CheckID, job.Attempt >= job.MaxAttempts)
}

// Timeout 是一次查詢的時間上限。
func (w *registryCheckWorker) Timeout(*river.Job[RegistryCheckArgs]) time.Duration {
	return registryJobTimeout
}

// AddWorkers 登記 River worker。
func (s *Service) AddWorkers(workers *river.Workers) {
	river.AddWorker(workers, &registryCheckWorker{service: s})
	river.AddWorker(workers, &registrySweepWorker{service: s})
}

// toRegistrationCheck 把查詢列轉成模組的型別。
func toRegistrationCheck(row store.SellerKycRegistrationCheck) model.RegistrationCheck {
	out := model.RegistrationCheck{
		ID: row.CheckID, Seq: row.Seq, RequestedAt: row.RequestedAt, Outcome: model.RegistrationOutcome(row.Outcome), QueriedAt: row.QueriedAt,
		MismatchedFields: row.MismatchedFields, UnverifiableFields: row.UnverifiableFields,
	}
	if out.Outcome != model.RegistrationFound {
		return out
	}
	reg := &model.Registration{
		TaxID: deref(row.TaxID), Name: deref(row.Name), Address: deref(row.AddressText),
		OrganizationType: deref(row.OrganizationType), Status: model.RegistrationStatus(deref(row.Status)), StatusText: deref(row.StatusText),
		TownCode: deref(row.TownCode), AddressDetail: deref(row.AddressDetail),
	}
	if row.SetupDate != nil {
		reg.SetupDate = *row.SetupDate
	}
	if row.SuspendedFrom != nil {
		reg.SuspendedFrom = *row.SuspendedFrom
	}
	if row.SuspendedUntil != nil {
		reg.SuspendedUntil = *row.SuspendedUntil
	}
	out.Registration = reg
	return out
}

// toPendingCheck 把進行中（PENDING）的查詢列轉成模組的型別：只有身分、要求時間與期限。
func toPendingCheck(row store.SellerKycRegistrationCheck) model.RegistrationCheck {
	deadline := row.DeadlineAt
	return model.RegistrationCheck{ID: row.CheckID, Seq: row.Seq, RequestedAt: row.RequestedAt, Outcome: model.RegistrationPending, Deadline: &deadline}
}

// shownCheck 是後台目前顯示的登記資料查詢：申請目前這一輪送件（submission_seq）seq 最大的已完成列；沒有時 row 是 nil（序號 0、不引用）。
type shownCheck struct {
	row *store.SellerKycRegistrationCheck
}

func (c shownCheck) seq() int64 {
	if c.row == nil {
		return 0
	}
	return c.row.Seq
}

func (c shownCheck) id() *uuid.UUID {
	if c.row == nil {
		return nil
	}
	return &c.row.CheckID
}

// latestCheck 讀申請目前顯示的登記資料查詢；只有公司型申請有查詢，個人型不讀。
func (s *Service) latestCheck(ctx context.Context, q *store.Queries, row store.SellerKycApplication) (shownCheck, error) {
	if row.SellerType != string(model.KYCCompany) {
		return shownCheck{}, nil
	}
	check, err := q.LatestCompletedKYCRegistrationCheck(ctx, row.ApplicationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return shownCheck{}, nil
	}
	if err != nil {
		return shownCheck{}, err
	}
	return shownCheck{row: &check}, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
