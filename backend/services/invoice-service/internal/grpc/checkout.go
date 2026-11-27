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
	prefs *usecase.Preference
	complaints *usecase.Complaint
	taxProfiles *usecase.TaxProfile
	adminJobs       *usecase.AdminJob
	adminIssuance   *usecase.AdminIssuance
	adminComplaints *usecase.AdminComplaint
	providerAccounts *usecase.ProviderAccount
	sellerComplaints *usecase.SellerComplaint
	sellerJobs *usecase.SellerJob
	resends *usecase.NotificationResend
	correctionIntake *usecase.CorrectionIntake
	files *usecase.DocumentFile
	sellerNames SellerNameResolver
	shipmentStatuses ShipmentStatusResolver
	buyerNames BuyerNameResolver
	evidenceMeta EvidenceMetadataResolver
	calendar *usecase.BusinessCalendarMirror
	log      *zap.Logger
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

func parseOptionalID(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	return parseID(s)
}

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
	return out, nil
}

func fromInvoiceInputSpec(s *pb.InvoiceInputSpec) (usecase.CheckoutInvoiceSpec, error) {
	if s == nil {
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
