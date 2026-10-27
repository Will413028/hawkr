package payuni

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Will413028/hawkr/go-pkg/payuni"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

const (
	payuniMerType       = 2
	payuniMerMCC        = 3
	payuniGoodsRate     = "100,0,0,0"
	payuniPayment       = "CREDIT,ApplePay,GooglePay,ATM"
	payuniDeliverTime   = 7
	payuniEscrowType    = 1
	payuniMerUnitPrice  = 1000
	payuniSellType      = 1
	payuniStatusSuccess = "SUCCESS"

	maxQPS = 2
)

type Client struct {
	client     *payuni.Client
	webBaseURL string
	limiter    *rateLimiter
}

func NewClient(cfg *config.Config) *Client {
	client := payuni.NewClient(cfg.Payuni.AgentID, cfg.Payuni.HashKey, cfg.Payuni.HashIV, cfg.Payuni.IsSandbox)
	limiter := newRateLimiter(maxQPS, nil)
	limiter.onThrottle = func() { payuniClientThrottledTotal.Inc() }
	limiter.onDegrade = func() { payuniClientDegradedTotal.Inc() }
	return &Client{client: client, webBaseURL: cfg.WebBaseUrl, limiter: limiter}
}

func call[T any](ctx context.Context, c *Client, fn func() (T, error)) (T, error) {
	var zero T
	if err := c.limiter.Wait(ctx); err != nil {
		return zero, fmt.Errorf("payuni rate limiter wait: %w", err)
	}
	result, err := fn()
	if looksRateLimited(err) {
		c.limiter.Degrade()
	}
	return result, err
}

func merchantStoreURL(webBaseURL, sellerID string) string {
	return fmt.Sprintf("%s/sellers/%s", strings.TrimRight(webBaseURL, "/"), sellerID)
}

func resolveMerURL(websiteURL, webBaseURL, storeID string) string {
	if websiteURL != "" {
		return websiteURL
	}
	return merchantStoreURL(webBaseURL, storeID)
}

func (c *Client) BindSeller(ctx context.Context, req *usecase.BindSellerRequest) (*usecase.PayuniResponse, error) {
	merURL := resolveMerURL(req.WebsiteURL, c.webBaseURL, req.StoreCode)

	merAddr := firstNonEmpty(req.CompanyAddress, req.Address, "台灣")

	postalCode := req.PostalCode
	if postalCode == "" {
		postalCode = "100"
	} else if len(postalCode) > 3 {
		postalCode = postalCode[:3]
	}

	payuniReq := &payuni.BindSellerRequest{
		MerID:         req.SellerID,
		MerType:       payuniMerType,
		MerMCC:        payuniMerMCC,
		GoodsRate:     payuniGoodsRate,
		Payment:       payuniPayment,
		DeliverTime:   payuniDeliverTime,
		EscrowType:    payuniEscrowType,
		MerUnitPrice:  payuniMerUnitPrice,
		SellType:      payuniSellType,
		MerName:       firstNonEmpty(req.StoreName, req.CompanyName, req.RealName),
		MerEngName:    req.EnglishName,
		MerEmail:      req.Email,
		MerURL:        merURL,
		MerAddr:       merAddr,
		MerAddrPost:   postalCode,
		MerDesc:       firstNonEmpty(req.StoreDescription, "Hawkr 生鮮食品"),
		ContactName:   req.RealName,
		ContactMobile: req.ContactPhone,
	}
	if req.SellerType == "business" {
		payuniReq.Name = req.CompanyName
		payuniReq.UBN = req.TaxID
		payuniReq.Email = req.Email
		payuniReq.ContactTel = req.CompanyPhone
		payuniReq.MBCName = req.RealName
		payuniReq.MBCTel = req.CompanyPhone
		payuniReq.MBCMail = req.Email
		payuniReq.MCName = req.RealName
		payuniReq.MCID = req.IDCardNo
	} else {
		payuniReq.Name = req.RealName
		payuniReq.UBN = req.IDCardNo
		payuniReq.Email = req.Email
	}
	resp, err := call(ctx, c, func() (*payuni.AgentApiResponse, error) {
		return c.client.BindSeller(ctx, payuniReq)
	})
	if err != nil {
		return &usecase.PayuniResponse{Success: false, ErrorMsg: err.Error()}, nil
	}
	rawResp, _ := json.Marshal(resp)
	success := resp.Status == payuniStatusSuccess
	out := &usecase.PayuniResponse{
		Success:     success,
		Message:     resp.Message,
		MerID:       req.SellerID,
		RawResponse: rawResp,
	}
	if !success {
		out.ErrorCode = resp.Status
	}
	return out, nil
}

func (c *Client) UpdateMerchant(ctx context.Context, req *usecase.UpdateMerchantRequest) (*usecase.PayuniResponse, error) {
	payuniReq := &payuni.UpdateMerchantRequest{
		MerID:         req.MerID,
		ContactName:   req.ContactName,
		ContactMobile: req.ContactMobile,
		ContactTel:    req.ContactTel,
		MerEmail:      req.MerEmail,
		MerAddr:       req.MerAddr,
		MerAddrPost:   req.MerAddrPost,
		MerTel:        req.MerTel,
	}
	resp, err := call(ctx, c, func() (*payuni.AgentApiResponse, error) {
		return c.client.UpdateMerchant(ctx, payuniReq)
	})
	if err != nil {
		return &usecase.PayuniResponse{Success: false, ErrorMsg: err.Error()}, nil
	}
	rawResp, _ := json.Marshal(resp)
	success := resp.Status == payuniStatusSuccess
	out := &usecase.PayuniResponse{Success: success, Message: resp.Message, MerID: req.MerID, RawResponse: rawResp}
	if !success {
		out.ErrorCode = resp.Status
	}
	return out, nil
}

func (c *Client) GetSellerStatus(ctx context.Context, merID string) (*usecase.SellerStatusResponse, error) {
	resp, err := call(ctx, c, func() (*payuni.SellerStatusResponse, error) {
		return c.client.GetSellerStatus(ctx, merID)
	})
	if err != nil {
		return nil, err
	}
	return &usecase.SellerStatusResponse{
		MerID: resp.MerID, MerStatus: resp.MerStatus, Reason: resp.Reason,
		MemberStatus:    resp.MemberStatus,
		CreditStatus:    resp.CreditStatus,
		ApplePayStatus:  resp.ApplePayStatus,
		GooglePayStatus: resp.GooglePayStatus,
		ATMStatus:       resp.ATMStatus,
		RawResponse:     resp.Raw,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
