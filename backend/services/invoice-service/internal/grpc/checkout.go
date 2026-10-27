package grpc

import (
	"context"
	"strconv"

	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InvoiceServer 實作 hawkr.invoice.InvoiceService。
//
// 本波只落地結帳鏈的兩支內部 RPC（§6.2.4 order-service 的 ②③）；其餘 RPC 由
// UnimplementedInvoiceServiceServer 承接，回 codes.Unimplemented。
//
// ⚠️ 那個 embed 是**必要的**（proto 契約要求 mustEmbed），但它也意味著漏實作一支 RPC
// 不會編譯錯、只會在執行期回 Unimplemented。「這支到底做了沒」不能靠編譯器答，
// 要靠 §6.1.0 的端點表逐列對。
type InvoiceServer struct {
	pb.UnimplementedInvoiceServiceServer

	checkout *usecase.Checkout
	query    *usecase.Query
	// prefs 是 B08 的寫入面（§6.1.1 端點 2／3／4／5）。它與 query 分開是因為只有它
	// 需要 TxManager——把寫入掛進 query 會讓「唯讀查詢面」這句話變成假的。
	prefs *usecase.Preference
	// complaints 是 B07 的寫入面（§6.1.6 端點 13／63／64／65）。與 query 分開的理由
	// 同 prefs：只有它需要 TxManager 與 outbox。
	complaints *usecase.Complaint
	// taxProfiles 是 M3 稅務資格狀態機的寫入面（§6.1.7 端點 18／19／20／22 ＋
	// §6.1.12 端點 40／41）。與 query 分開的理由同 prefs／complaints：只有它需要
	// TxManager、outbox 與媒體換發。
	taxProfiles *usecase.TaxProfile
	// adminJobs／adminIssuance／adminComplaints 是 ADM 面的三個寫入 usecase
	// （§6.1.14 端點 52／53／56、§6.1.15 端點 57、§6.1.16 端點 60）。它們與 query
	// 分開的理由同上：只有它們需要 TxManager、outbox 與稽核寫入。
	adminJobs       *usecase.AdminJob
	adminIssuance   *usecase.AdminIssuance
	adminComplaints *usecase.AdminComplaint
	// providerAccounts 是 M4「AMEGO 帳戶」狀態機的寫入面（§6.1.8 端點 24／25／26／27 ＋
	// §6.1.13 端點 44／45／66／67）。與 query 分開的理由同上：只有它需要 TxManager、
	// ADM 稽核與 ProviderClient。
	providerAccounts *usecase.ProviderAccount
	// sellerComplaints 是 S10 賣家回覆申訴（§6.1.11 端點 36）。與買家側的 complaints
	// 分開是因為**歸屬鍵不同**（store_id vs complainant_buyer_id），不是因為角色不同。
	sellerComplaints *usecase.SellerComplaint
	// sellerJobs 是端點 62（賣家回報自行開立）。它與 adminJobs 共用同一個 transition
	// helper（§6.1.14），差別只在歸屬鍵與 actor。
	sellerJobs *usecase.SellerJob
	// resends 是端點 32／49／71 的重寄入口。三支端點的歸屬鍵不同、計數器相同
	// （key ＝ document_id，不是 document_id × actor）⇒ 共用一個 usecase。
	resends *usecase.NotificationResend
	// correctionIntake 是 B06 的組裝層（§6.1 端點 11）。它與 query 分開的理由同上：
	// 只有它需要 TxManager、跨服務 client 與五個讀取面。
	correctionIntake *usecase.CorrectionIntake
	// files 是 §6.1.4 的 PDF 授權下載。端點 10／70 走這裡拿中介資料；
	// bytes 由 10042 的檔案面直接送，**不經 gRPC**（bytes 不該經 gRPC，§4.3.1）。
	files *usecase.DocumentFile
	// sellerNames 只服務 ADM 工作台的識別欄（#3017）。它可以是 nil：
	// 店名取不到時退回 store_id，而那不影響任何一筆資料的正確性。
	sellerNames SellerNameResolver
	// shipmentStatuses 只服務 S06／A12 的出貨狀態欄（#3019）。它可以是 nil：
	// 取不到時退回空字串，而那不影響任何一筆發票資料的正確性。
	// ⚠️ 但「取不到」必須傳達出去（shipment_status_degraded），理由見
	// shipment_status.go 的檔頭——空字串在這一欄會被讀成「還沒出貨」。
	shipmentStatuses ShipmentStatusResolver
	// buyerNames 只服務申訴畫面的「是誰提的」欄（#3017 後半）。它可以是 nil：
	// 取不到時退回空字串，那不影響任何一筆申訴資料的正確性。
	// ⚠️ resolveBuyerNames 回的**已經是遮罩後的值**，本層拿不到明碼。
	buyerNames BuyerNameResolver
	// evidenceMeta 只服務稅籍佐證清單的中介資料（#3031）。可以是 nil：
	// 取不到時退回「只有檔案路徑」，那正是修這件事之前的畫面。
	evidenceMeta EvidenceMetadataResolver
	log          *zap.Logger
}

// NewInvoiceServer 組出 gRPC handler。
func NewInvoiceServer(
	checkout *usecase.Checkout,
	query *usecase.Query,
	prefs *usecase.Preference,
	complaints *usecase.Complaint,
	taxProfiles *usecase.TaxProfile,
	adminJobs *usecase.AdminJob,
	adminIssuance *usecase.AdminIssuance,
	adminComplaints *usecase.AdminComplaint,
	providerAccounts *usecase.ProviderAccount,
	sellerComplaints *usecase.SellerComplaint,
	sellerJobs *usecase.SellerJob,
	resends *usecase.NotificationResend,
	correctionIntake *usecase.CorrectionIntake,
	files *usecase.DocumentFile,
	sellerNames SellerNameResolver,
	shipmentStatuses ShipmentStatusResolver,
	buyerNames BuyerNameResolver,
	evidenceMeta EvidenceMetadataResolver,
	log *zap.Logger,
) *InvoiceServer {
	return &InvoiceServer{
		checkout: checkout, query: query, prefs: prefs, complaints: complaints,
		taxProfiles: taxProfiles, adminJobs: adminJobs, adminIssuance: adminIssuance,
		adminComplaints: adminComplaints, providerAccounts: providerAccounts,
		sellerComplaints: sellerComplaints, sellerJobs: sellerJobs, resends: resends,
		correctionIntake: correctionIntake, files: files,
		sellerNames: sellerNames, shipmentStatuses: shipmentStatuses,
		buyerNames:   buyerNames,
		evidenceMeta: evidenceMeta, log: log,
	}
}

// ResolveOrderInvoiceModeInternal 是 checkout preview 的發票區塊（§6.1.2(a)）。
//
// 這支決定 B01 能不能讓買家付款：回不出可判定的 invoice_mode，前端就進 `invalid` 並阻擋付款
// ⇒ 失敗時回 gRPC error 而不是「一個看起來成功但 mode 為空的回應」，
// 後者會讓前端顯示一張沒有任何提示的空卡片。
func (s *InvoiceServer) ResolveOrderInvoiceModeInternal(
	ctx context.Context, req *pb.ResolveOrderInvoiceModeInternalRequest,
) (*pb.ResolveOrderInvoiceModeInternalResponse, error) {
	if err := requireInternalCall(ctx); err != nil {
		return nil, err
	}

	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	// buyer_id 允許缺席（訪客結帳）：0 表示「沒有買家可查常用發票資料」，
	// 與「查了但沒有預設」是兩件事，usecase 兩者都回 defaultProfile=null 但不猜。
	buyerID, err := parseOptionalID(req.GetBuyerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "buyer_id: %v", err)
	}
	product, err := parseAmount(req.GetProductAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "product_amount: %v", err)
	}
	shipping, err := parseAmount(req.GetShippingAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "shipping_amount: %v", err)
	}
	discount, err := parseAmount(req.GetDiscountAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "discount_amount: %v", err)
	}

	out, err := s.checkout.ResolveMode(ctx, usecase.ResolveModeInput{
		StoreID:        storeID,
		BuyerID:        buyerID,
		ProductAmount:  product,
		ShippingAmount: shipping,
		DiscountAmount: discount,
		CheckoutID:     req.GetCheckoutId(),
	})
	if err != nil {
		s.logger().Error("invoice.checkout.resolve_mode.failed",
			zap.Int64("store_id", storeID), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "resolve order invoice mode: %v", err)
	}

	return &pb.ResolveOrderInvoiceModeInternalResponse{
		Code:    0,
		Message: "success",
		Data: &pb.ResolveOrderInvoiceModeInternalData{
			InvoiceMode: out.InvoiceMode,
			// 🔴 snapshot_version 刻意留空。唯一寫入者是 order-service（I-50：
			// 「repository 不暴露任何遞增 snapshot_version 的方法」），而本 request
			// 也沒有帶進來可回聲的值 ⇒ 本服務在這裡填任何數字都是憑空造一個第二寫入者。
			// order-service 把這個區塊併進 CheckoutPreviewResponse 時用自己的值覆蓋。
			SnapshotVersion:  "",
			PreviewExpiresAt: timeutil.FormatISO8601(out.PreviewExpiresAt),
			Amount:           toAmountBreakdown(out.Amount),
			Disclosure:       toDisclosure(out.Disclosure),
			DefaultProfile:   toProfileInfo(out.DefaultProfile),
			IntakeLimit:      toIntakeLimit(out.IntakeLimit),
		},
	}, nil
}

// ValidateOrderInvoiceInputInternal 是 checkout confirm 的發票驗證（§6.1.2(b)）。
//
// 驗證失敗是**業務結果不是故障**：回 code≠0 ＋ valid=false，讓 order-service 把它翻成
// 前端的 409／200＋preserveData。回 gRPC error 會讓那六個分支全部塌縮成同一個 424。
func (s *InvoiceServer) ValidateOrderInvoiceInputInternal(
	ctx context.Context, req *pb.ValidateOrderInvoiceInputInternalRequest,
) (*pb.ValidateOrderInvoiceInputInternalResponse, error) {
	if err := requireInternalCall(ctx); err != nil {
		return nil, err
	}

	storeID, err := parseID(req.GetStoreId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "store_id: %v", err)
	}
	buyerID, err := parseOptionalID(req.GetBuyerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "buyer_id: %v", err)
	}
	amount, err := fromAmountBreakdown(req.GetAmount())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "amount: %v", err)
	}
	spec, err := fromInvoiceInputSpec(req.GetInvoice())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invoice: %v", err)
	}
	snapshotVersion, err := parseSnapshotVersion(req.GetSnapshotVersion())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "snapshot_version: %v", err)
	}

	out, err := s.checkout.ValidateInput(ctx, usecase.ValidateInvoiceInput{
		StoreID:         storeID,
		BuyerID:         buyerID,
		OrderNo:         req.GetOrderNo(),
		Invoice:         spec,
		Amount:          amount,
		SnapshotVersion: snapshotVersion,
		IdempotencyKey:  req.GetIdempotencyKey(),
	})
	if err != nil {
		s.logger().Error("invoice.checkout.validate_input.failed",
			zap.Int64("store_id", storeID), zap.String("order_no", req.GetOrderNo()), zap.Error(err))
		return nil, status.Errorf(codes.Internal, "validate order invoice input: %v", err)
	}

	// 🔴 非 0 業務碼一樣帶 data：§6.1.2 對 9326 明文要求 `data.intakeLimit` 帶現況。
	// 回 code≠0 ＋ 空 data 會讓「上限還差多少」永遠到不了前端。
	return &pb.ValidateOrderInvoiceInputInternalResponse{
		Code:    out.BizCode,
		Message: out.Message,
		Data: &pb.ValidateOrderInvoiceInputInternalData{
			Valid:                out.Valid,
			SnapshotJson:         string(out.SnapshotJSON),
			ResolvedCarrierType:  out.ResolvedCarrierType,
			ResolvedDisplayValue: out.ResolvedDisplayValue,
			InvoiceMode:          out.InvoiceMode,
			IntakeLimit:          toIntakeLimit(out.IntakeLimit),
		},
	}, nil
}

func (s *InvoiceServer) logger() *zap.Logger {
	if s.log == nil {
		return zap.NewNop()
	}
	return s.log
}

// ---------------------------------------------------------------------------
// 型別邊界（§4.2.2 Layer 5：對外 ID／金額一律 string）
// ---------------------------------------------------------------------------

// parseID 解析必填的 id 字串。空字串是錯誤而不是 0：呼叫端沒帶 store_id 時，
// 靜默當成 0 會讓決策樹回一個「這位賣家還沒送審」的合理答案，而問題其實在契約。
func parseID(s string) (int64, error) {
	if s == "" {
		return 0, errEmptyID
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, errNonPositiveID
	}
	return v, nil
}

// parseOptionalID 解析可缺席的 id 字串（空字串 ⇒ 0）。
func parseOptionalID(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return parseID(s)
}

// parseAmount 解析金額字串（元，整數）。空字串 ⇒ 0（proto3 沒帶就是零值）。
// 🔴 金額禁止浮點：ParseInt 而不是 ParseFloat 再取整。
func parseAmount(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseSnapshotVersion(s string) (int32, error) {
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(v), nil
}

func toAmountBreakdown(a usecase.CheckoutAmount) *pb.InvoiceAmountBreakdown {
	return &pb.InvoiceAmountBreakdown{
		ProductAmount:  strconv.FormatInt(a.ProductAmount, 10),
		ShippingAmount: strconv.FormatInt(a.ShippingAmount, 10),
		DiscountAmount: strconv.FormatInt(a.DiscountAmount, 10),
		InvoiceAmount:  strconv.FormatInt(a.InvoiceAmount, 10),
		// tax_category 的 wire 值是 CodeMeta 的 Value（"1"/"2"/"3"），不是 Code
		// （TAXABLE/…）——InvoiceTaxCategory 是全書唯一 Code != Value 的 enum。
		TaxCategory: strconv.Itoa(int(a.TaxCategory)),
	}
}

func fromAmountBreakdown(a *pb.InvoiceAmountBreakdown) (usecase.CheckoutAmount, error) {
	var out usecase.CheckoutAmount
	if a == nil {
		return out, errMissingAmount
	}
	var err error
	if out.ProductAmount, err = parseAmount(a.GetProductAmount()); err != nil {
		return out, err
	}
	if out.ShippingAmount, err = parseAmount(a.GetShippingAmount()); err != nil {
		return out, err
	}
	if out.DiscountAmount, err = parseAmount(a.GetDiscountAmount()); err != nil {
		return out, err
	}
	// 🔴 invoice_amount 與 tax_category **不從 request 採信**：兩者都由本服務依
	// product/shipping/discount ＋ 賣家的 default_tax_category 重算（usecase 的
	// buildCheckoutAmount）。採信呼叫端送來的總額，等於讓上游決定要開多少錢的發票。
	return out, nil
}

func fromInvoiceInputSpec(s *pb.InvoiceInputSpec) (usecase.CheckoutInvoiceSpec, error) {
	if s == nil {
		// 整包省略是合法形狀（§6.1.2(b)：已上架 App 直接呼叫，改必填會打爛線上結帳）。
		return usecase.CheckoutInvoiceSpec{Present: false}, nil
	}
	profileID, err := parseOptionalID(s.GetProfileId())
	if err != nil {
		return usecase.CheckoutInvoiceSpec{}, err
	}
	return usecase.CheckoutInvoiceSpec{
		Present:       true,
		ProfileID:     profileID,
		CarrierType:   s.GetCarrierType(),
		Value:         s.GetValue(),
		BuyerTaxID:    s.GetBuyerTaxId(),
		BuyerName:     s.GetBuyerName(),
		Email:         s.GetEmail(),
		SaveAsDefault: s.GetSaveAsDefault(),
	}, nil
}

func toDisclosure(d usecase.CheckoutDisclosure) *pb.InvoiceDisclosure {
	return &pb.InvoiceDisclosure{
		Required:     d.Required,
		TermsVersion: d.TermsVersion,
		Title:        d.Title,
		Body:         d.Body,
	}
}

// toProfileInfo 把預設常用發票資料攤成 wire message。
//
// 沒有預設時回 nil（proto3 的缺席 message ＝ JSON null），對應 §6.1.2(a)
// 「買家沒有任何 is_default = true 的存活列時一律回 null。**不再有合成列**」。
// 回一個所有欄位都空的 message 會讓前端看到一張「有載具但什麼都沒填」的卡片。
func toProfileInfo(p usecase.CheckoutProfile) *pb.InvoiceProfileInfo {
	if !p.Found {
		return nil
	}
	return &pb.InvoiceProfileInfo{
		ProfileId:    strconv.FormatInt(p.ProfileID, 10),
		CarrierType:  p.CarrierType,
		DisplayValue: p.DisplayValue,
		BuyerTaxId:   p.BuyerTaxID,
		BuyerName:    p.BuyerName,
		IsDefault:    p.IsDefault,
		Label:        p.Label,
		CreatedAt:    timeutil.FormatISO8601(p.CreatedAt),
		UpdatedAt:    timeutil.FormatISO8601(p.UpdatedAt),
	}
}

// toIntakeLimit 把待審接單上限攤成 wire message。
//
// 非待審期間回 nil（§6.1.2(a)「`intakeLimit` 非待審期間為 null」）：回一組全 0 會被前端
// 讀成「上限是 0 筆」⇒ 每個賣家都顯示已達上限。
func toIntakeLimit(l usecase.CheckoutIntakeLimit) *pb.SellerIntakeLimit {
	if !l.Applicable {
		return nil
	}
	return &pb.SellerIntakeLimit{
		OrderCountUsed:  strconv.FormatInt(int64(l.OrderCountUsed), 10),
		OrderCountLimit: strconv.FormatInt(int64(l.OrderCountLimit), 10),
		AmountUsed:      strconv.FormatInt(l.AmountUsed, 10),
		AmountLimit:     strconv.FormatInt(l.AmountLimit, 10),
		Blocked:         l.Blocked,
	}
}

// parseVersion 解析樂觀鎖版本字串。
//
// 與 parseID 的差別是**允許 0**：version 從 0 起算（chk_invcomp_version_nonneg），
// 一件從未被更新過的案件版本就是 "0"。空字串仍是錯誤——必填欄缺席是契約破了。
func parseVersion(s string) (int64, error) {
	if s == "" {
		return 0, errEmptyID
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		return 0, errNonPositiveID
	}
	return v, nil
}
