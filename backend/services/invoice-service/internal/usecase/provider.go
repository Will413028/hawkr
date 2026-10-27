package usecase

import (
	"context"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
)

// ProviderClient 是 usecase 對 AMEGO 的唯一出口（§4.2.2「不直接打 AMEGO 的 HTTP」）。
//
// 介面刻意只列本波用得到的方法，而不是照抄 amego.Client 的 16 個：測試要注入的 fake
// 只需要實作真正被呼叫的那些，而「這個 usecase 會打 AMEGO 的哪幾支端點」從簽章上看得出來。
// amego.Client 天然滿足本介面（方法集合是它的子集），fx 直接綁。
//
// 🔴 環境閘門（AMEGO_ALLOW_REAL_INVOICE，§4.5.8）由 amego client 的 guardEnvironment 負責，
// usecase **不得**重複實作：兩份判斷會漂移，而漂移的方向是「對真實公司開出無法回收的真發票」。
type ProviderClient interface {
	IssueInvoice(ctx context.Context, cred amego.Credential, req amego.IssueRequest) (*amego.IssueResult, error)

	// VoidInvoice 打 /json/f0501。與 IssueInvoice 並列是刻意的：兩者是**同一條 lane**
	// （INV.op）的兩個寫入類出口，設計書 §7 表列把 f0401／f0501 放在同一列同一組租約。
	//
	// ⚠️ 上面「刻意不放 IssueInvoice 以外的寫入類方法」那句話指的是**憑證驗證**那一組
	// ——驗證不得產生任何發票。作廢不是驗證，它本來就是寫入類，兩者不衝突。
	VoidInvoice(ctx context.Context, cred amego.Credential, req amego.VoidRequest) (*amego.VoidAccepted, error)

	// IssueAllowance 打 /json/g0401（折讓開立）。與 VoidInvoice 同一條 lane（INV.op）。
	//
	// 🔴 金額欄位一律**未稅**，與 IssueInvoice 的 DetailVat=1 含稅語意相反
	// （§5A.9.3）⇒ 兩套 DTO 不可共用金額欄位。
	IssueAllowance(ctx context.Context, cred amego.Credential, req amego.AllowanceRequest) (*amego.AllowanceResult, error)

	// QueryAllowance 打 /json/allowance_query。唯讀。
	//
	// 折讓軸靠它收斂 WAITING_CONFIRMATION（G13／G14）：g0401 逾時之後**不得重送**
	// （我方自產的單號會重複），只能去問。
	QueryAllowance(ctx context.Context, cred amego.Credential, allowanceNumber string) (*amego.AllowanceDetail, error)

	// QueryInvoiceByNumber 打 /json/invoice_query（type=invoice）。唯讀。
	//
	// 作廢軸靠它確認 invoice_type 從 C0401 變成 C0501——f0501 回 code:0 只代表
	// AMEGO **受理**了作廢請求，憑證本身要等它處理完 wait[] 佇列才真的變。
	// 用號碼查而不是用 order_id 查：一筆訂單在作廢重開之後會有兩張發票，
	// 用 order_id 查回來的是哪一張沒有保證。
	QueryInvoiceByNumber(ctx context.Context, cred amego.Credential, number string) (*amego.InvoiceDetail, error)

	// ── 憑證驗證（§7.3.4 P03 的三項，端點 25／44／66 共用）──
	//
	// 兩支都是**唯讀**端點：驗證不得產生任何發票（§6.1.8 端點 25 逐字），所以這裡刻意
	// 不放 IssueInvoice 以外的任何寫入類方法。
	//
	// QueryCompanyNames 打 /json/ban_query，同時覆蓋 P03 的 ①（簽章）與帳號層三碼
	// （13／19／22 ⇒ PROVIDER_REJECTED）：簽章不過會回 code 16、統編錯回 code 12，
	// 兩者都在回應裡可分辨。
	//
	// ListTracks 打 /json/track_all，覆蓋 P03 的 ③「至少一組可用字軌」。少了這一項，
	// 賣家會通過 S03 的全部檢查、看到綠色「已啟用」，然後每一筆開票都回 3040111
	// 字軌不足——那是重試多久都不會好的錯誤。
	QueryCompanyNames(ctx context.Context, cred amego.Credential, bans []string) (map[string]string, error)
	ListTracks(ctx context.Context, cred amego.Credential, year, period int) ([]amego.Track, error)

	// VerifyMobileBarcode 打 /json/barcode。唯讀，是**唯一**的載具驗證端點且只驗手機條碼。
	//
	// 🔴 放在 INV.op 這條 lane 上是刻意的：§6.3.1 明令 consumer 絕不呼叫 AMEGO，所以
	// 「送出前驗載具」只能發生在真正要送 f0401 的那一刻，與 IssueInvoice 共用同一次
	// 憑證解密。放進 consumer 會同時違反那條紅線並在 broker 上做無退避的外部呼叫。
	//
	// 為什麼非驗不可（設計書 §P12 逐字要求，2026-09-08 才接上）：AMEGO 對載具「接受
	// 任意 6 碼字串」、三選一互斥「完全不擋」⇒ 我方是唯一防線。不驗的後果不是開票失敗
	// 而已——f0401 回 3040131／3040132 之後 operation 落 FAILED_PERMANENT，那一格
	// 沒有自動出口，法定上傳期限一路倒數，而罰的是賣家。
	VerifyMobileBarcode(ctx context.Context, cred amego.Credential, barcode string) error
}
