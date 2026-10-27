// Package media 是 invoice-service 對 media-service 的出站 peer client（58 §6.1.7）。
//
// 它只做一件事：把賣家上傳稅籍證明拿到的 **staged token** 換成 objectKey，並替換出來的
// 資產建立 asset_reference。順序契約逐字取自 §6.1.7「換發」表：
//
//	① FinalizeImages（致命）→ ② LinkImages（致命）→ ③ 業務列寫入（致命）
//	→ ④ ReleaseReferences（best-effort）
//
// 🔴 ③ 之前不呼叫 ReleaseReferences：顛倒順序會把**正在被引用**的檔案交給 media GC，
// 連同 R2 物件實體刪除，而 evidence_object_keys 還指著它 ⇒ ADM 複審點下去才 404。
//
// 🔴 漏掉 ② 的症狀更隱蔽：資產沒有 asset_reference ⇒ media GC 第 2 階段在 7 天後刪掉
// 它，而業務列與畫面在那 7 天內完全正常（§6.1.7「漏了會怎樣」）。
package media

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	pkgmedia "github.com/Will413028/hawkr/go-pkg/media"
	pb "github.com/Will413028/hawkr/go-pkg/proto/media"

	"github.com/Will413028/hawkr/go-pkg/grpcclient"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

// RefType 是稅籍證明的引用類型：既有的 AssetReferenceType.CERTIFICATION。
//
// 🔴 **不新增 refType 碼值**（§6.1.7 換發表逐字）。副作用是 objectKey 落在
// `secure/certification/...`，前綴仍是 `secure/` ⇒ 下載通道不受影響。
const RefType = pkgmedia.RefTypeCertification

// DefaultDeadline 是本 client 對外那一整段換發的時間預算。
//
// pkg/media 內部對 finalize（10s）與 link（5s × 3 次）各有自己的 timeout，本值是
// 蓋在它們之上的 backstop：換發是端點 18 請求路徑的一部分，賣家在等，
// 不能讓 media-service 卡住時整個 handler 無限期掛住。
const DefaultDeadline = 30 * time.Second

// Client 包住 MediaService stub。
type Client struct {
	stub     pb.MediaServiceClient
	deadline time.Duration
}

// NewClient 撥號 media-service。
//
// 🔴 **forwardAccountHeaders 必須是 true，這一支與其他 peer client 不同。**
//
// media-service 的 FinalizeImages 用 `GrpcAuthContext.requireAccountId()` 取
// **使用者**身分，再拿它與 staged token 的 `account_id` 比對；不一致就回
// 「資產不存在」（防枚舉，故意與真的不存在同碼）。⇒ 不轉發 x-account-id 時，
// 每一次 FinalizeEvidence 都必定失敗，而錯誤訊息看起來像「檔案不見了」。
//
// 這條路 2026-09-01 dev 實測前**從來沒有成功過**：端點 18（存草稿）一定會在
// finalize 稅籍證明時 500，於是賣家從零走不到可送審的 DRAFT——那正是整條發票鏈
// 的第一步。症狀完全誤導：upload_session 是 COMPLETED、物件也真的上傳了，
// 只有 media_asset 那張表一列都沒有。
//
// ⚠️ 其他三支 peer client（order／seller／...）維持 false 是對的：它們的對端
// 授權的是**服務**身分，不是使用者身分。這個差異來自對端怎麼授權，不是慣例。
func NewClient(lc fx.Lifecycle, addr string) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("invoice: media peer addr is empty (MEDIA_SERVICE_HOST)")
	}
	conn, err := grpcclient.Dial(addr,
		grpc.WithUnaryInterceptor(interceptor.InternalAuthClientInterceptor("invoice-service", true)),
		grpc.WithStatsHandler(interceptor.OTelClientStatsHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("invoice: dial media-service %s: %w", addr, err)
	}
	lc.Append(fx.Hook{OnStop: func(context.Context) error { return conn.Close() }})
	return NewClientWithStub(pb.NewMediaServiceClient(conn), DefaultDeadline), nil
}

// NewClientWithStub 讓測試注入 stub 與較短的 deadline。
func NewClientWithStub(stub pb.MediaServiceClient, deadline time.Duration) *Client {
	if deadline <= 0 {
		deadline = DefaultDeadline
	}
	return &Client{stub: stub, deadline: deadline}
}

// FinalizeEvidence 把 staged token 換成 objectKey，並替它們建立 asset_reference。
//
// refID 是 **seller_tax_profiles.id**，不是 store_id（§6.1.7 換發表的紅字）：
// `7111` 的 ref_id 空間與 seller-service 的商家認證共用，而 ReleaseReferences 依
// (ref_type, ref_id) 批次釋放 ⇒ 用一個兩張表都可能命中的鍵，會把別人的引用一起打掉。
//
// 回 (objectKeys, assetIDs)：assetIDs 是 ReleaseReferences 的 keep 名單所需，
// 呼叫端必須把它與「保留清單原本就有的資產」合起來——但保留清單存的是 objectKey 不是
// assetId，那個缺口寫在 ReleaseEvidence 的註解。
func (c *Client) FinalizeEvidence(ctx context.Context, stagedTokens []string, taxProfileID int64) ([]string, []string, error) {
	if len(stagedTokens) == 0 {
		return nil, nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	refID := strconv.FormatInt(taxProfileID, 10)
	keys := make([]string, 0, len(stagedTokens))
	assetIDs := make([]string, 0, len(stagedTokens))
	for _, token := range stagedTokens {
		// 一顆一顆換：pkg/media.FinalizeAndLink 是「finalize 一顆 ＋ link 一顆」的組合，
		// 而 link 帶三次重試。批次一次送進 FinalizeImages 可以少幾次往返，但那條路上
		// 沒有現成的「哪一顆失敗了」回報 ⇒ 部分成功會變成整批看起來失敗，而已轉正的
		// 那幾顆已經有 asset_reference、不會被 GC 回收。
		info, err := pkgmedia.FinalizeAndLink(ctx, c.stub, token, RefType, refID)
		if err != nil {
			return nil, nil, fmt.Errorf("invoice: finalize tax evidence: %w", err)
		}
		if info.ObjectKey == "" {
			// 空 objectKey 寫進 evidence_object_keys 會讓 T02 的守衛（至少一項）成立，
			// 而那一項下載不到任何東西 ⇒ 當成失敗，不要放行。
			return nil, nil, fmt.Errorf("invoice: media returned empty object_key for staged token")
		}
		keys = append(keys, info.ObjectKey)
		assetIDs = append(assetIDs, info.AssetId)
	}
	return keys, assetIDs, nil
}

// FinalizeUnlockEvidence 把 A4 人工解鎖的取消證據（staged token）換成 objectKey（#3108）。
//
// 🔴 **refType 用 InvoiceDocumentRefType 而不是 RefType**（＝ CERTIFICATION）：
// 後者的 ref_id 空間與 seller-service 的商家認證共用，而 ReleaseReferences 依
// (ref_type, ref_id) 批次釋放 ⇒ 借用它會讓某次稅籍佐證的回收把解鎖證據一起打掉。
// 這一條與 FinalizeEvidence 註解裡的紅字是同一個陷阱，方向相反。
//
// refID 用 **invoice_operations.id**：證據屬於那一件工作，不屬於文件或賣家。
//
// 空 token 回空字串而不是錯誤：證據檔是選填的（kind ＋ 查證說明已構成 §6.4 的證據），
// 呼叫端不必為「沒附檔」寫一條分支。
func (c *Client) FinalizeUnlockEvidence(ctx context.Context, stagedToken string, operationID int64) (string, error) {
	if stagedToken == "" {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	info, err := pkgmedia.FinalizeAndLink(ctx, c.stub, stagedToken,
		InvoiceDocumentRefType, strconv.FormatInt(operationID, 10))
	if err != nil {
		return "", fmt.Errorf("invoice: finalize unlock evidence (operation %d): %w", operationID, err)
	}
	if info.ObjectKey == "" {
		// 空 objectKey 落進 cancel_evidence_object_key 會讓覆核者看到一個
		// 下載不到任何東西的證據——那比沒有證據更糟（它看起來有）。
		return "", fmt.Errorf("invoice: media returned empty object_key for unlock evidence (operation %d)", operationID)
	}
	return info.ObjectKey, nil
}

// ReadUnlockEvidence 讀回 A4 取消證據（#3108）。
//
// refID 必須與 FinalizeUnlockEvidence 當初寫入時一致（operation.id）——
// media 的 ServerReadSecureAsset 用 (objectKey, refID) 一起授權，
// 傳錯 refID 的症狀是「資產不存在」而不是「無權存取」。
func (c *Client) ReadUnlockEvidence(ctx context.Context, objectKey string, operationID int64) ([]byte, string, string, error) {
	return c.ReadSecure(ctx, objectKey, strconv.FormatInt(operationID, 10))
}

// ReleaseEvidence 回收「這次沒有被留下」的資產（順序契約的第 ④ 步，best-effort）。
//
// ⚠️ **keepAssetIDs 目前只含本次新換發的資產**：保留清單（request 的 evidenceObjectKeys）
// 存的是 objectKey，而 media 的 keep 名單認的是 assetId，本服務沒有 objectKey → assetId
// 的反查（media proto 沒有這支查詢）。⇒ 本函式只在「本次確實有新換發」時呼叫，否則
// 一次純刪除的草稿存檔會把所有既有佐證的引用一起釋放掉。缺口登記在交付回報。
func (c *Client) ReleaseEvidence(ctx context.Context, taxProfileID int64, keepAssetIDs []string) error {
	if len(keepAssetIDs) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()
	return pkgmedia.ReleaseReferences(ctx, c.stub, RefType,
		[]string{strconv.FormatInt(taxProfileID, 10)}, keepAssetIDs)
}

// InvoiceDocumentRefType 是發票 PDF 的引用類型（CodeMeta AssetReferenceType 7120）。
//
// 🔴 與稅籍證明的 RefType（CERTIFICATION）**不同**：兩者的 releaseReferences 是依
// (ref_type, ref_id) 批次釋放的，混用會讓釋放其一時打掉另一批。
const InvoiceDocumentRefType = pkgmedia.RefTypeInvoiceDocument

// UploadSecure 把一份機敏檔（發票 PDF）直傳 media 並在同一次呼叫內建立 asset_reference。
//
// 🔴 走 ServerUploadSecureAndFinalize 而**不是** ServerUploadAndFinalize（58 §165）：
// 後者硬寫 public/ 前綴、is_secure=false、uploaderAccountId=NULL ⇒ 授權述詞兩分支都不通。
//
// 🔴 refID 用 invoice_documents.id **不用 order_no**（§8.6 T-04）：releaseReferences 依
// (ref_type, ref_id) 批次釋放，用 order_no 會在任一份文件要釋放時，把原發票＋重開票＋
// 所有折讓的引用一起打掉。
//
// 回 objectKey。**只存 key 不存 URL**（§25／49 號設計書）。
func (c *Client) UploadSecure(ctx context.Context, data []byte, filename, contentType, refID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()
	resp, err := c.stub.ServerUploadSecureAndFinalize(ctx, &pb.ServerUploadSecureAndFinalizeRequest{
		FileData:    data,
		Filename:    filename,
		ContentType: contentType,
		RefType:     InvoiceDocumentRefType,
		RefId:       refID,
	})
	if err != nil {
		return "", err
	}
	if resp.GetCode() != 0 {
		return "", fmt.Errorf("media: ServerUploadSecureAndFinalize code=%d msg=%s", resp.GetCode(), resp.GetMessage())
	}
	if resp.GetObjectKey() == "" {
		// code=0 但沒有 key ＝ 對端契約破了。回錯誤而不是存一個空 key——後者會讓
		// pdf_object_key 變成空字串，而「有值但下載不到」比「沒有值」更難診斷。
		return "", fmt.Errorf("media: ServerUploadSecureAndFinalize 回 code=0 但 object_key 為空")
	}
	return resp.GetObjectKey(), nil
}

// ErrAssetNotFound 代表 media 明確回「這份資產不存在」（回應 body 的 code 403／404）。
//
// 它與一般的讀取失敗必須分開：本服務用它遞增 M15
// （invoice_documents_pdf_missing_total），而那支指標守的是「§8.1.5(e) 的
// asset_reference 到底有沒有真的建立」——沒建的話 media GC 會在 7 天後把 R2 物件
// 連同紀錄刪掉，**而 AMEGO 的 invoice_file 只能查 180 天內**，超過就再也抓不回來。
// 把它折進一般錯誤，那條時效很硬的線索就永遠不會出現。
var ErrAssetNotFound = errors.New("media: secure asset not found")

// ErrAssetNotSecure 代表 media 回「此資產不是機敏檔」（body code 400）。它不是缺件，不遞增 M15。
var ErrAssetNotSecure = errors.New("media: asset is not secure")

// ReadSecure 把先前以服務身分上傳的機敏檔讀回原始位元組（58 §6.1.4）。
//
// 🔴 為什麼不能直接讀 OSS：機敏資產在儲存上是**加密的**，金鑰只有 media 拿得到；
// 而「儲存 SDK 只准在 media／product-import」是 §25 的硬規則。
//
// 🔴 為什麼不能走 media 的 `secure_download`：那支的授權述詞是「**使用者**是上傳者
// 或有 asset_reference」。發票 PDF 由本服務以服務身分上傳，合法讀者卻是買家／賣家／ADM
// 三個真人 ⇒ 三個人都不符合那條述詞。誰能看是**本服務**的事（order_no × buyer_id ×
// document_id），media 只負責把 bytes 還給上傳它的那個服務。
//
// refID 與 UploadSecure 當時傳的必須是同一個（invoice_documents.id），
// 它與 objectKey 合起來就是對端的授權述詞——只給 objectKey 是不夠的。
func (c *Client) ReadSecure(ctx context.Context, objectKey, refID string) (data []byte, contentType, filename string, err error) {
	return c.readSecure(ctx, objectKey, InvoiceDocumentRefType, refID)
}

// ReadTaxEvidence 讀回稅籍佐證檔（後台稅籍審核頁的下載，plan 8f-1）。
//
// 引用是 FinalizeEvidence 建的那一筆：refType ＝ RefType（CERTIFICATION），
// refID ＝ seller_tax_profiles.id。傳成 InvoiceDocumentRefType 的症狀是「資產不存在」。
// objectKey 必須來自該 profile 的 evidence_object_keys：誰能讀由呼叫端（invoice）決定，
// media 只驗 (objectKey, refType, refId) 三元組。
func (c *Client) ReadTaxEvidence(ctx context.Context, objectKey string, taxProfileID int64) ([]byte, string, string, error) {
	return c.readSecure(ctx, objectKey, RefType, strconv.FormatInt(taxProfileID, 10))
}

func (c *Client) readSecure(ctx context.Context, objectKey, refType, refID string) (data []byte, contentType, filename string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()
	resp, err := c.stub.ServerReadSecureAsset(ctx, &pb.ServerReadSecureAssetRequest{
		ObjectKey: objectKey,
		RefType:   refType,
		RefId:     refID,
	})
	if err != nil {
		return nil, "", "", err
	}
	// 🔴 media 把業務失敗放在**回應的 code**（gRPC 狀態是 OK）：MediaServiceException 被
	// MediaGrpcService 接住後回 code=404／400，不會是 gRPC NOT_FOUND。判斷必須看 body code。
	// 「物件不存在」與「media 掛了」對呼叫端是兩件事：前者代表 asset_reference 沒建成、
	// GC 已經刪掉實體（§8.1.5(e)），是 M15 的定義；後者是暫時性故障。
	// 查無與三元組不符對端回同一個碼（403／404），與單體 media adapter 同一個映射；
	// 不是機敏資產（400）是另一件事——資料錯置而不是缺件。
	switch code := resp.GetCode(); code {
	case 0:
	case 403, 404:
		return nil, "", "", fmt.Errorf("%w: object_key=%s: media code=%d msg=%s", ErrAssetNotFound, objectKey, code, resp.GetMessage())
	case 400:
		return nil, "", "", fmt.Errorf("%w: object_key=%s: media msg=%s", ErrAssetNotSecure, objectKey, resp.GetMessage())
	default:
		return nil, "", "", fmt.Errorf("media: ServerReadSecureAsset code=%d msg=%s", code, resp.GetMessage())
	}
	if len(resp.GetFileData()) == 0 {
		// code=0 但零位元組 ＝ 對端契約破了。回錯誤而不是把空檔往下送——
		// 後者的症狀是「買家下載到一個 0 bytes 的 PDF」，比 5xx 難診斷得多。
		return nil, "", "", fmt.Errorf("media: ServerReadSecureAsset 回 code=0 但 file_data 為空")
	}
	return resp.GetFileData(), resp.GetContentType(), resp.GetFilename(), nil
}

// AssetMetadata 是一份稅籍佐證的中介資料（#3031）。
type AssetMetadata struct {
	ObjectKey  string
	AssetID    string
	FileName   string
	SizeBytes  string
	UploadedAt string
	MimeType   string
}

// BatchGetEvidenceMetadata 取一批稅籍佐證的中介資料（#3031）。
//
// 🔴 **asset_id 是下載路徑的唯一輸入**（Gateway 組
// `/api/v1/app/media/secure_download/{id}`）。在這支 RPC 之前，本服務手上只有
// evidence_object_keys 的 objectKey，組不出下載連結 ⇒ S05／A02 的佐證清單只畫得出
// 一個點不下去的檔名。
//
// 回傳 map 只含**通過授權且存在**的資產；查無與越權在對端是同一個處置（都不出現），
// 所以這裡也分不出來——那是刻意的（否則這支 RPC 會變成可枚舉的 oracle）。
//
// 🔴 錯誤往上拋，不在這一層降級：降不降級是呼叫端的決定。
func (c *Client) BatchGetEvidenceMetadata(ctx context.Context, objectKeys []string, taxProfileID int64) (map[string]AssetMetadata, error) {
	if len(objectKeys) == 0 {
		return map[string]AssetMetadata{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.deadline)
	defer cancel()

	resp, err := c.stub.BatchGetAssetMetadata(ctx, &pb.BatchGetAssetMetadataRequest{
		RefType:    RefType,
		RefId:      strconv.FormatInt(taxProfileID, 10),
		ObjectKeys: objectKeys,
	})
	if err != nil {
		return nil, fmt.Errorf("invoice: batch get evidence metadata (%d keys): %w", len(objectKeys), err)
	}
	if code := resp.GetCode(); code != 0 {
		return nil, fmt.Errorf("invoice: batch get evidence metadata: downstream code=%d message=%q", code, resp.GetMessage())
	}

	out := make(map[string]AssetMetadata, len(resp.GetData()))
	for _, m := range resp.GetData() {
		if m.GetObjectKey() == "" {
			continue
		}
		out[m.GetObjectKey()] = AssetMetadata{
			ObjectKey:  m.GetObjectKey(),
			AssetID:    m.GetAssetId(),
			FileName:   m.GetFileName(),
			SizeBytes:  m.GetSizeBytes(),
			UploadedAt: m.GetUploadedAt(),
			MimeType:   m.GetMimeType(),
		}
	}
	return out, nil
}
