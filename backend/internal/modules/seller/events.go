package seller

import "github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"

// StoreOpened 在開店的 transaction 內發出。
type StoreOpened = model.StoreOpened

// StoreProfileChanged 在 profile、logo 或封面寫入的 transaction 內發出。
type StoreProfileChanged = model.StoreProfileChanged

// StoreSuspended 在店家從未停權變成停權的 transaction 內發出（停權措施生效）。
type StoreSuspended = model.StoreSuspended

// StoreUnsuspended 在店家從停權恢復的 transaction 內發出（措施結束、撤銷、接受申覆或到期）。
type StoreUnsuspended = model.StoreUnsuspended

// StoreExposureChanged 在店家最後公告的可曝光改變的 transaction 內發出：可曝光翻轉的寫入由它自己的 transaction 發出，
// 時間造成的變化（處分到期）由對帳 job 補發；Version 是這家店的公告序號（從 1 開始），消費者依它排序、丟棄舊版本。
type StoreExposureChanged = model.StoreExposureChanged

// StorePendingDeletion 在店家隨帳號刪除進入等待的 transaction 內發出。
type StorePendingDeletion = model.StorePendingDeletion

// StoreClosed 在 stint 關閉的 transaction 內發出。關店可逆，它不是下游刪除資料的訊號（那是 StoreRetired）。
type StoreClosed = model.StoreClosed

// StoreReopened 在已關閉的 stint 恢復營業（終止合作撤銷或申覆成立）的 transaction 內發出。
type StoreReopened = model.StoreReopened

// StoreRetired 在 stint 退場（關店後的第一段清理，一般關店 30 天、終止合作取 30 天與最晚申覆期限較晚者）的 transaction 內發出，一個店家只發一次；個資在第二段（保留期滿）才清：此後恢復營業不可能，下游的店家資料可以刪除。
type StoreRetired = model.StoreRetired

// ReportReceived 在每次送出的 transaction 內發出：新的檢舉、補充（SupplementID）、補充已達上限而沒有存下的匿名送出（Discarded）。ConfirmationEmail 是這次送出留的 email
// （只有匿名）：確認信寄到這裡；它只在事件與 A8 同一個 transaction 排入的 job 參數裡，不落 seller 的表。
// 義務（A8 的訂閱者）：訂閱者不得在 job 完成後保留這個欄位——River v0.47.0 預設保留完成的 job 24 小時、丟棄（寄送失敗用盡重試）的 job 7 天
// （`rivershared/riversharedmaintenance/river_shared_maintenance.go`），job 參數裡的地址會跟著留那麼久，所以 A8 的寄送 job 必須在完成或丟棄時清掉參數裡的地址
// （或參數只放 report／supplement 的參照，由 job 執行時再讀；地址就落在 seller，回到 365 天的保留規則管得到的範圍）。計畫書 A8 列了同一條。
type ReportReceived = model.ReportReceived

// ReportResolved 在通報有了結果的 transaction 內發出，每筆通報一次：個別設定、直接結掉或案件結案帶入都算。
type ReportResolved = model.ReportResolved

// MeasureCreated 在建立措施的 transaction 內發出，帶開始時間（StartsAt）與終止合作的事由；沒有獨立的「終止預告」事件。
type MeasureCreated = model.MeasureCreated

// MeasureRevised 在措施賣家可見的內容實際改變的 transaction 內發出。
type MeasureRevised = model.MeasureRevised

// MeasureEnded 在 admin 結束 RISK 措施、接受解除請求，或店家關閉而結束預告終止的 transaction 內發出。
type MeasureEnded = model.MeasureEnded

// MeasureRevoked 在措施撤銷（admin 撤銷、接受申覆或連帶撤銷）的 transaction 內發出。
type MeasureRevoked = model.MeasureRevoked

// MeasureRequestSubmitted 在賣家提出請求（申覆或解除請求，看 Kind）的 transaction 內發出。
type MeasureRequestSubmitted = model.MeasureRequestSubmitted

// MeasureRequestDecided 在 admin 裁定請求的 transaction 內發出。
type MeasureRequestDecided = model.MeasureRequestDecided

// MeasureRequestClosed 在待處理的請求以 CLOSED 結案的 transaction 內發出，Cause 是措施撤銷或措施已結束。
type MeasureRequestClosed = model.MeasureRequestClosed

// FoodRegistrationDecided 在管理員裁定食品業者登錄（核准或退回，看 Outcome）的 transaction 內發出。
type FoodRegistrationDecided = model.FoodRegistrationDecided

// FoodRegistrationRevoked 在管理員撤銷核准中的食品業者登錄的 transaction 內發出。
type FoodRegistrationRevoked = model.FoodRegistrationRevoked

// KYCSubmitted 在賣家送出 KYC 申請的 transaction 內發出。
type KYCSubmitted = model.KYCSubmitted

// KYCApproved 在管理員核准 KYC 申請的 transaction 內發出（人工決定；PayUni 的綁定結果是後續階段的事件）。
type KYCApproved = model.KYCApproved

// KYCRejected 在管理員退回 KYC 申請的 transaction 內發出，對應舊的 merchant.verification.rejected。
type KYCRejected = model.KYCRejected

// KYCCancelled 在賣家取消 KYC 申請的 transaction 內發出。
type KYCCancelled = model.KYCCancelled

// PayUniForwarded 在 PayUni 收下註冊（綁定成功）的 transaction 內發出，對應舊的 merchant.verification.forwarded。
type PayUniForwarded = model.PayUniForwarded

// PayUniApproved 在 PayUni 審核通過（套用觀測）的 transaction 內發出，對應 merchant.verification.approved。
type PayUniApproved = model.PayUniApproved

// PayUniPendingInfo 在 PayUni 要求補件的 transaction 內發出，對應 merchant.verification.pending_info。
type PayUniPendingInfo = model.PayUniPendingInfo

// PayUniUnderReview 在 PayUni 從待補件回到審核中的 transaction 內發出，對應 merchant.verification.under_review。
type PayUniUnderReview = model.PayUniUnderReview

// PayUniRejected 在 PayUni 拒絕或關閉的 transaction 內發出，對應 merchant.verification.rejected。
type PayUniRejected = model.PayUniRejected

// PayUniBindFailed 在綁定失敗的 transaction 內發出（與 bind_status 同一個 transaction；舊系統沒有這個事件）。
type PayUniBindFailed = model.PayUniBindFailed

// ProspectArchived 在業者由進行中封存的 transaction 內發出；重送、還原與合併造成的封存都不發這個事件。
type ProspectArchived = model.ProspectArchived

// ProspectMerged 在合併業者的 transaction 內發出（輸家封存並指向贏家；輸家的封存只由這個事件表達）。
type ProspectMerged = model.ProspectMerged
