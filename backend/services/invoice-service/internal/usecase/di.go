package usecase

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module 是 Layer 4 的組裝。
//
// tsid.NewGenerator 放這一層是 §4.2.1 的定版：ID 的產生點是「業務決定要建立一個新實體」
// 的那一刻，不是 repository 收到參數的那一刻——把它下放到 repository，冪等鍵就會在每次
// 重試各自產生一個新 ID，而 ON CONFLICT DO NOTHING 是拿 operation_key 去比對的，
// 不是拿 ID。
var Module = fx.Module("usecase",
	fx.Provide(newIDGenerator),
	fx.Provide(newIssuanceConfig),
	fx.Provide(fx.Annotate(newProviderClient, fx.As(new(ProviderClient)))),
	fx.Provide(NewIssuance),
	fx.Provide(NewVoiding),
	// 折讓軸（#3073）。它與 Voiding 並列而不是併進去：兩者送的 payload、落的狀態、
	// 要維護的反正規化欄位完全不同（折讓要累加 allowance_total_amount 與逐行
	// allowanced_amount，作廢不用）。
	fx.Provide(newAllowance),
	// G01：`order.refund.finalized` 落到作廢或折讓（§7.2.2）。它是折讓軸唯一的入口
	// —— 沒有它，invoice_adjustments 上永遠不會出現任何一列，上面那條執行軸全是死碼。
	fx.Provide(newRefundAdjustment),
	// Checkout 是結帳鏈的兩支唯讀內部 RPC（§6.1.2）。它沒有 TxManager 也沒有
	// ProviderClient —— 兩支都不寫資料、也不打 AMEGO，多注入一個依賴就是多一條
	// 「有人以為可以在這裡開票」的路。
	fx.Provide(NewCheckout),
	// Query 是 §6.1 唯讀查詢面。它與 Checkout 一樣沒有 TxManager 與 ProviderClient：
	// 唯一的寫入是 ADM 揭露 P 類資料時的稽核列，走 AuditRepository 單列 append。
	fx.Provide(newTrack),
	fx.Provide(newHealthCheck),
	fx.Provide(newRecheck),
	fx.Provide(newGovernance),
	fx.Provide(newPDF),
	fx.Provide(NewQuery),
	// DocumentFile 是 §6.1.4 的 PDF 授權下載（端點 10／70 的中介資料 ＋ 三條
	// /file/content 的 bytes）。它與 Query 分開的理由不是「檔案 vs 查詢」，而是
	// **依賴集合不同**：它需要 media 的機敏檔讀回面，而 Query 有 15 個依賴且不該
	// 為了一條下載路徑再多一個對外呼叫。
	fx.Provide(NewDocumentFile),
	// TaxEvidenceFile 是後台稅籍審核頁的佐證檔下載（plan 8f-1）。授權述詞由本服務持有
	// （store 的稅務資格上有這個位置的佐證），取代已刪除的通用 admin 機敏下載。
	fx.Provide(NewTaxEvidenceFile),
	// Preference 是 B08 的寫入面。它是全 usecase 層第二個持有 TxManager 的型別，
	// 理由只有一個：ClearDefault ＋ SetDefault 必須同交易（§6.1.1 端點 5）。
	fx.Provide(newPreference),
	// Complaint 是 B07 的寫入面（§6.1.6 端點 13／63／64／65）。它同時持有 TxManager
	// 與 OutboxRepository：狀態轉移、時間軸事件與（K01 的）賣家通知必須同進同退。
	fx.Provide(NewComplaint),
	// B06 更正鏈（§7.3.5 R01–R04）：Correction 執行狀態機、CorrectionIntake 組裝。
	//
	// 🔴 兩者刻意分開：Correction 的職責是「吃事實 ＋ 吃改好的資料，執行分流後的那條邊」，
	// 它的每一支測試都能用純 fake 跑完。把 document 讀取、operation 查找、provider 憑證、
	// 明細重建掛進去，那句話就不成立，而它的測試會變成要先擺好五個 repo 才跑得動。
	fx.Provide(newCorrection),
	fx.Provide(newCorrectionIntake),
	// TaxProfile 是 M3 稅務資格狀態機的寫入面（§6.1.7 端點 18／19／20／22 ＋
	// §6.1.12 端點 40／41）。它是本層唯一同時持有 TxManager、OutboxRepository 與
	// **對外媒體換發**的型別——那三者缺一，稅務資格就分別會失去原子性、失去 ADM
	// 通知、失去佐證檔（換發沒做 ⇒ evidence_object_keys 永遠是空的 ⇒ T02 的守衛
	// 恆不成立 ⇒ 賣家永遠送不出審核）。
	fx.Provide(NewTaxProfile),
	// ADM 面三個寫入 usecase（§6.1.14 端點 52／53／56、§6.1.15 端點 57、
	// §6.1.16 端點 60）。三者都持有 TxManager：狀態 CAS、狀態歷程、ADM 稽核與
	// （端點 56 的）outbox 必須同進同退——少了交易會出現「狀態轉了但沒有人知道是誰轉的」，
	// 而 invoice_admin_audit_logs 是 5 年保存義務（§9.5）。
	fx.Provide(NewAdminJob),
	fx.Provide(NewAdminIssuance),
	fx.Provide(NewAdminComplaint),
	// ProviderAccount 是 M4「AMEGO 帳戶」狀態機的寫入面（§6.1.8 端點 24／25／26／27 ＋
	// §6.1.13 端點 44／45／66／67）。它是本層唯一同時持有 TxManager、OperationRepository
	// 與 ProviderClient 的型別：停用／撤銷必須在同一交易內把該 issuer 的未終態 operation
	// 一起轉終態（§7.4 C-8），而憑證驗證是同步等 AMEGO 的唯讀呼叫。
	fx.Provide(NewProviderAccount),
	// SellerComplaint 是 S10 的寫入面（§6.1.11 端點 36）。它與買家側的 Complaint 分開，
	// 理由不是「賣家 vs 買家」而是**歸屬鍵不同**：買家側述詞是 complainant_buyer_id、
	// 賣家側是經 invoice_issuers 解析的 store_id。同一個型別上並存兩種歸屬語意，
	// 正是 Tier A 最容易破的地方。
	fx.Provide(NewSellerComplaint),
	// SellerJob 是端點 62（賣家回報自行開立）。它持有 *AdminJob 並共用 resolveExternally
	// ——§6.1.14 逐字「端點 56 與 62 都要走同一個 transition helper，不得各寫一份 CAS」。
	fx.Provide(NewSellerJob),
	// NotificationResend 是端點 32／49／71 的共用重寄入口（§6.1.10）。三個入口的歸屬鍵
	// 不同、計數器相同 ⇒ 一個 usecase 三支方法，不是三份實作。
	fx.Provide(NewNotificationResend),
	fx.Provide(newManualIssuanceConfig),
	fx.Provide(fx.Annotate(newTaxEvidenceStore, fx.As(new(TaxEvidenceStore)))),
	// 平台代製收據（#3026）：渲染器 ＋ 上傳 port。
	fx.Provide(fx.Annotate(newReceiptRenderer, fx.As(new(ReceiptRenderer)))),
	fx.Provide(fx.Annotate(newReceiptUploader, fx.As(new(ReceiptUploader)))),
)

// newReceiptRenderer 把收據字型讀起來並建渲染器。
//
// 🔴 **fail-closed：字型讀不起來就讓 pod 起不來。** 這裡不是「有更好、沒有也行」——
// 收據全篇中文，字型缺席時 go-pdf/fpdf 產出的是一份**合法但整頁空白**的 PDF：
// `%PDF-` magic bytes 對、HTTP 200、下載得到，只有內容不見了。沒有任何一道下游 gate
// 攔得住那個形狀（§3.10 的「機制看不到窗外」），所以只能在 boot 期擋。
//
// ⚠️ 字型必須是 **TTF**：alpine 的 CJK 套件全是 TTC，而 fpdf 對 TTC 回
// "get metrics Error: not supported"。Dockerfile 的 font stage 抓的是釘住 SHA256 的
// Noto Sans TC TTF，dev 與 prod 兩個 stage 都 COPY 一份。
func newReceiptRenderer(cfg *config.Config, log *zap.Logger) (*domain.ReceiptRenderer, error) {
	font, err := os.ReadFile(cfg.Receipt.FontPath)
	if err != nil {
		return nil, fmt.Errorf("invoice: 收據字型讀取失敗（RECEIPT_FONT_PATH=%s）：%w", cfg.Receipt.FontPath, err)
	}
	r, err := domain.NewReceiptRenderer(font)
	if err != nil {
		return nil, fmt.Errorf("invoice: 收據字型無法載入（RECEIPT_FONT_PATH=%s，必須是 TTF 不是 TTC）：%w",
			cfg.Receipt.FontPath, err)
	}
	log.Info("invoice.receipt.renderer_ready",
		zap.String("font_path", cfg.Receipt.FontPath),
		zap.Int("font_bytes", len(font)),
		zap.String("layout_version", domain.ReceiptLayoutVersion))
	return r, nil
}

// newReceiptUploader 把 media peer client 綁到收據的落檔 port（同 newTaxEvidenceStore 的理由）。
func newReceiptUploader(c *media.Client) *media.Client { return c }

// newTaxEvidenceStore 把 media peer client 綁到本層的換發 port。
//
// 這一層刻意不是 fx.As(new(TaxEvidenceStore)) 直接掛 *media.Client：那會讓
// 「usecase 用到 media 的哪幾支能力」從 fx 圖上看不出來，也會讓測試替身得實作
// 整個 MediaServiceClient。
func newTaxEvidenceStore(c *media.Client) *media.Client { return c }

// newIDGenerator 提供 TSID 產生器。
//
// ⚠️ fx 圖上它的型別就是 `func() int64` —— 若日後有第二個「無參數回 int64」的建構函式
// 被 provide 進來，fx 只會說「重複提供 func() int64」而不會指出是哪兩個。那時的正解是
// 給兩邊各套一個具名型別，不是把其中一個改成回 int64 的值。
func newIDGenerator() func() int64 { return tsid.NewGenerator() }

// newProviderClient 把 amego.Client 綁到 usecase 的 ProviderClient 介面。
//
// 這一層刻意不是 fx.As(new(ProviderClient)) 直接掛 amego.Client：那會讓「usecase 用到
// AMEGO 的哪幾支端點」從 fx 圖上看不出來，而 fake 也得實作全部 16 個方法。
func newProviderClient(c amego.Client) amego.Client { return c }

// newIssuanceConfig 把 config 的排程參數折成 usecase 需要的形狀。
//
// 🔴 MaxAttempts 只在**建列當下**寫進 invoice_operations.max_attempts；worker 判斷
// 「次數用完了沒有」讀的是列上的值不是本值 —— 兩者的 DB DEFAULT 與 §0.2 必須同值（13）。
func newIssuanceConfig(cfg *config.Config) IssuanceConfig {
	return IssuanceConfig{
		MaxAttempts: int32(cfg.Schedule.IssueMaxRetry), //nolint:gosec // 值域由 Config.Validate 守
		BaseBackoff: secondsOf(cfg.Schedule.BackoffBaseSeconds),
		MaxBackoff:  secondsOf(cfg.Schedule.BackoffMaxSeconds),
		IssueLease:  secondsOf(cfg.Schedule.IssueLeaseSeconds),
		// RecheckBase 是 WAITING_CONFIRMATION 的第一段退避（30s，§7.5.3 的階梯起點）。
		// 沿用 BackoffBaseSeconds：兩者在 §0.2 是同一個 30 秒，另開一個 env 只會多一個
		// 沒有人會去對的漂移點。
		RecheckBase: secondsOf(cfg.Schedule.BackoffBaseSeconds),
	}
}

// newPreference 把入口層的載具驗證掛上（#3256）。
//
// 🔴 用**平台**憑證：買家建立常用發票資料時沒有賣家上下文。/json/barcode 是唯讀查詢、
// 不產生任何發票，用平台帳號查完全合適（§6.1.8 端點 25 的「驗證不得產生發票」講的是
// 同一件事的另一面）。
//
// 憑證缺任一半就不掛（nil ⇒ 整段跳過）：本地開發沒有平台 AppKey 時仍要能建 profile，
// 而開票前那一層仍會擋掉錯的載具。⚠️ 2026-09-08 起「不掛」不等於「沒人知道」——
// 只有一半時會 log.Error 並把 invoice_platform_credential_incomplete 設成 1（#3269）。
func newPreference(
	tx database.TxManager,
	prefs domain.BuyerPreferenceRepository,
	idem domain.IdempotencyRepository,
	provider ProviderClient,
	cfg *config.Config,
	newID func() int64,
	log *zap.Logger,
) *Preference {
	u := NewPreference(tx, prefs, idem, newID, log)
	// 🔴 三個分支不是兩個：「兩半都在」與「兩半都缺」都是合法狀態，**只有一半**才是缺陷。
	// 原本寫成一個 if，於是「只有一半」跟「都沒設」走同一條路——而前者是配置錯誤、
	// 後者是功能未啟用，兩者的正確處置完全相反。見 platformCredentialIncomplete 的註解。
	switch {
	case cfg.Amego.PlatformInvoice != "" && cfg.Amego.PlatformAppKey != "":
		u = u.WithBarcodeVerifier(platformBarcodeVerifier{
			provider: provider,
			cred: amego.Credential{
				Invoice: cfg.Amego.PlatformInvoice,
				AppKey:  cfg.Amego.PlatformAppKey,
			},
		})
		platformCredentialIncomplete.Set(0)
		mobileBarcodeVerifierEnabled.Set(1)
	case cfg.Amego.PlatformInvoice == "" && cfg.Amego.PlatformAppKey == "":
		// 平台 issuer 整個未啟用。#3230 把 AMEGO_PLATFORM_INVOICE 由必填改為選填之後，
		// **prod 正好落在這一格**（兩個都沒設）——它從「到不了的分支」變成了常態。
		//
		// 🔴 兩支指標在這裡刻意不同值：platformCredentialIncomplete ＝ 0（這不是配置
		// 錯誤，是還沒開通），而 mobileBarcodeVerifierEnabled ＝ 0（verifier 確實沒掛上，
		// 無效手機條碼會被 HTTP 200 接受）。只看前者會以為一切正常，那正是 M34 存在的理由。
		platformCredentialIncomplete.Set(0)
		mobileBarcodeVerifierEnabled.Set(0)
	default:
		platformCredentialIncomplete.Set(1)
		mobileBarcodeVerifierEnabled.Set(0)
		log.Error("平台 AMEGO 憑證只有一半，手機條碼存在性驗證已靜默停用",
			zap.Bool("has_platform_invoice", cfg.Amego.PlatformInvoice != ""),
			zap.Bool("has_platform_app_key", cfg.Amego.PlatformAppKey != ""),
			zap.String("impact", "買家建立／修改常用發票資料時，無效的手機條碼會被接受（HTTP 200），錯誤要到開票前那一層才會被擋下"),
			zap.String("fix", "backend#3269：把 AMEGO_PLATFORM_APP_KEY 放進 hawkr-amego secret"))
	}
	return u
}

// platformBarcodeVerifier 把 ProviderClient 的三參數簽章折成 usecase 要的一參數窄介面，
// 憑證在建構時綁定。
//
// ⚠️ AppKey 明文只活在這個 struct 內、不進 log：Credential 的 String() 已遮罩，
// 而本型別不實作 Stringer ⇒ %v 印出來走的是 Credential 自己那份。
type platformBarcodeVerifier struct {
	provider ProviderClient
	cred     amego.Credential
}

func (v platformBarcodeVerifier) VerifyMobileBarcode(ctx context.Context, barcode string) error {
	return v.provider.VerifyMobileBarcode(ctx, v.cred, barcode)
}

// newManualIssuanceConfig 把平台統編折成端點 57 需要的形狀。
//
// AMEGO_PLATFORM_INVOICE 是 issuer_tax_id_snapshot 在 issuer=PLATFORM 時的唯一來源：
// invoice_issuers 沒有統編欄，而 PLATFORM 沒有 seller_tax_profiles 那一列。
//
// ⚠️ **它可能是空字串**（#3230 起為選填，空 ＝ 平台 issuer 未啟用）。本層仍然不補預設值
// ——補了就會開出一張統編是空字串的發票。擋下空值的是 admin_issuance.go 的 resolveIssuer
// PLATFORM 分支（§4.5.8 第三層），在**開票當下**回錯誤，而不是在啟動時拒絕啟動。
func newManualIssuanceConfig(cfg *config.Config) ManualIssuanceConfig {
	return ManualIssuanceConfig{PlatformTaxID: cfg.Amego.PlatformInvoice}
}

func secondsOf(v int) time.Duration { return time.Duration(v) * time.Second }

// newTrack 建 INV.trk 的 usecase（§7.5.3）。
//
// TrackObserver 由 worker 那層提供（usecase 不 import promauto，理由見 track.go）。
// warnThreshold 與 growthHeadroom 走 config，值與預設見 §9.3.3
// （INVOICE_TRACK_BALANCE_WARN_THRESHOLD=200／INVOICE_TRACK_GROWTH_HEADROOM=0.5）。
func newTrack(
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	obs TrackObserver,
	ops domain.OperationRepository,
	log *zap.Logger,
	cfg *config.Config,
) *Track {
	return NewTrack(accounts, client, obs, ops, log, time.Now,
		int64(cfg.Schedule.TrackBalanceWarnThresh), cfg.Governance.TrackGrowthHeadroom)
}

// newHealthCheck 建 INV.hc 的 usecase（§7.5.3）。
//
// batch 取 §7.5.3 表上的 FetchBatch（healthCheckBatch = 50，單一來源在 health_check.go）。
func newHealthCheck(
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	log *zap.Logger,
) *HealthCheck {
	return NewHealthCheck(accounts, client, log, time.Now, healthCheckBatch)
}

// newRecheck 建 INV.rc 的 usecase（§7.5.3）。
func newRecheck(
	ops domain.OperationRepository,
	docs domain.DocumentRepository,
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	voids *Voiding,
	allowances *Allowance,
	audit domain.AuditRepository,
	log *zap.Logger,
) *Recheck {
	return NewRecheck(ops, docs, accounts, client, voids, allowances, audit, log, time.Now)
}

// newGovernance 建 INV.gov 的 usecase（§7.5.3）。
//
// counter 與 profiles 是兩個不同的 repository：M20 數的是 operation，
// C4 改的是 seller_tax_profiles。
//
// businessDays／holidays／tx 是行事曆鏡像的每日同步（§5.1.14；#3089 第 17 項）：
// 鏡像的兩個寫入方法在 2026-09-04 之前**零呼叫端**，於是那張表自建立以來一列都沒有
// ⇒ 每一筆申訴的 SLA 都退到 14 自然日的硬上限，而退化是靜默的。
//
// inbox／outboxRows 是 §5.4.6 的兩張保留期清理表（#3261 ①②）：兩者的 SQL、adapter
// 與設計書規範都齊備，而在 2026-09-08 之前**沒有任何東西呼叫它們** —— inbox 的
// payload（含 buyer_id／store_id／金額）永久保留，outbox 的 done 列永不刪除。
func newGovernance(
	profiles domain.SellerTaxProfileRepository,
	ops domain.OperationRepository,
	obs GovernanceObserver,
	businessDays domain.BusinessDayRepository,
	holidays HolidaySource,
	inbox domain.InboxRepository,
	outboxRows domain.OutboxRepository,
	tx database.TxManager,
	log *zap.Logger,
) *Governance {
	return NewGovernance(profiles, ops, obs, businessDays, holidays, tx, log, time.Now).
		WithRetention(inbox, outboxRows)
}

// newPDF 建 INV.pdf 的 usecase（§7.5.3）。
//
// maxBytes 走 config 的 PDFMaxBytes：下載一份沒有上限的檔案會讓一次 claim 的
// 最壞耗時失去上界，而租約算式的前提正是那個上界。
func newPDF(
	docs domain.DocumentRepository,
	accounts domain.ProviderAccountRepository,
	client amego.Client,
	uploader *media.Client,
	log *zap.Logger,
	cfg *config.Config,
) *PDF {
	return NewPDF(docs, accounts, client, uploader, log, time.Now, cfg.Schedule.PDFMaxBytes)
}

// newCorrection 建 B06 的狀態機執行層。
//
// orders 取 OrderSnapshotReviser（寫入面）——讀取面由 CorrectionIntake 自己拿，
// 兩者分開讓「誰會寫 order-service」在型別上說得清楚。
func newCorrection(
	tx database.TxManager,
	corrections domain.CorrectionRequestRepository,
	operations domain.OperationRepository,
	documents domain.DocumentRepository,
	adjustments domain.AdjustmentRepository,
	orders OrderSnapshotReviser,
	audit domain.AuditRepository,
	log *zap.Logger,
	newID func() int64,
	cfg IssuanceConfig,
) *Correction {
	// maxAttempts 沿用開票的重試上限：作廢與開票打的是同一個 provider，
	// 給它們不同的忍耐度沒有理由，而兩份設定必然漂移。
	return NewCorrection(tx, corrections, operations, documents, adjustments, orders, audit,
		log, func() time.Time { return time.Now().UTC() }, newID, cfg.MaxAttempts)
}

// newCorrectionIntake 建 B06 的組裝層。
// tracks 直接吃 amego.Client（同 newHealthCheck 的做法）：TrackLister 是一個
// **單方法的消費端介面**，由使用者宣告、由既有具體型別滿足——不需要也不該為它
// 另建一個 fx binding，那會多一個只有一個實作的間接層。
func newCorrectionIntake(
	documents domain.DocumentRepository,
	operations domain.OperationRepository,
	accounts domain.ProviderAccountRepository,
	prefs domain.BuyerPreferenceRepository,
	corrections domain.CorrectionRequestRepository,
	client amego.Client,
	correction *Correction,
) *CorrectionIntake {
	return NewCorrectionIntake(CorrectionIntakeDeps{
		Documents: documents, Operations: operations, Accounts: accounts,
		Preferences: prefs, Corrections: corrections, Tracks: client,
	}, correction)
}

// newAllowance 組出折讓軸。
//
// b2cSplitTax 來自 INVOICE_ALLOWANCE_B2C_SPLIT_TAX（§5A.9.4 未決 U5，預設 true）：
// B2C 原發票 TaxAmount=0 時折讓單是否仍逐行拆稅，官方範例與 f0401 的規則互相矛盾。
// 預設走官方範例的 A 版，留開關而不是寫死 —— 那個答案只會是二選一，而查證要對 AMEGO
// 送兩版折讓看哪張被拒。
// newRefundAdjustment 建 G01 usecase（§7.2.2）。
func newRefundAdjustment(
	tx database.TxManager,
	adjustments domain.AdjustmentRepository,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	audit domain.AuditRepository,
	newID func() int64,
	cfg IssuanceConfig,
	appCfg *config.Config,
	log *zap.Logger,
) *RefundAdjustment {
	return NewRefundAdjustment(tx, adjustments, docs, ops, audit, newID, cfg,
		appCfg.Governance.AllowanceB2CSplitTax, log, func() time.Time { return time.Now().UTC() })
}

func newAllowance(
	tx database.TxManager,
	docs domain.DocumentRepository,
	ops domain.OperationRepository,
	adjustments domain.AdjustmentRepository,
	accounts domain.ProviderAccountRepository,
	issuers domain.IssuerRepository,
	audit domain.AuditRepository,
	outbox domain.OutboxRepository,
	provider ProviderClient,
	cfg IssuanceConfig,
	appCfg *config.Config,
	log *zap.Logger,
) *Allowance {
	return NewAllowance(tx, docs, ops, adjustments, accounts, issuers, audit, outbox, provider,
		cfg, appCfg.Governance.AllowanceB2CSplitTax, log)
}
