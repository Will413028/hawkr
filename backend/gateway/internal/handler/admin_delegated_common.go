package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Will413028/hawkr/gateway/internal/handler/delegation"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)


var delegatedIdempotencyKeyRE = middleware.IdempotencyKeyPattern

var delegatedVersionRE = regexp.MustCompile(`^\d{1,19}$`)

type delegatedMutationData struct {
	AuditLogID string `json:"auditLogId"`
	Version    string `json:"version"`
	Resource   any    `json:"resource"`
}

type delegatedRequest struct {
	Session        *accountpb.OperationSessionSummary
	StoreID        int64
	IdempotencyKey string
	RequestID      string
	CorrelationID  string
	UserAgent      string
}

func resolveDelegatedRequest(c *gin.Context, requireIdempotency bool) (*delegatedRequest, bool) {
	session, ok := middleware.DelegationSession(c)
	if !ok {
		response.BizStatus(c, bizcode.DelegationCredentialInvalid, "代行憑證無效，請重新建立工作階段")
		return nil, false
	}
	storeID, err := strconv.ParseInt(c.Param("store_id"), 10, 64)
	if err != nil || storeID <= 0 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "店家代號格式不正確")
		return nil, false
	}

	req := &delegatedRequest{
		Session: session,
		StoreID: storeID,
		RequestID:     requestTraceValue(c, "request_id", "X-Request-Id"),
		CorrelationID: requestTraceValue(c, "trace_id", "x-trace-id"),
		UserAgent:     c.GetHeader("User-Agent"),
	}

	key := c.GetHeader("Idempotency-Key")
	if requireIdempotency {
		if !delegatedIdempotencyKeyRE.MatchString(key) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "Idempotency-Key 格式不正確")
			return nil, false
		}
	}
	req.IdempotencyKey = key
	return req, true
}

func requestTraceValue(c *gin.Context, contextKey, headerName string) string {
	if v := c.GetString(contextKey); v != "" {
		return v
	}
	return c.GetHeader(headerName)
}

func (h *Holder) orchestrator() *delegation.Orchestrator {
	return &delegation.Orchestrator{Audit: h.Account, Cfg: h.Delegation}
}

func (r *delegatedRequest) readDenial(resourceType, resourceID string, code int32) delegation.ReadDenial {
	return delegation.ReadDenial{
		Session:       r.Session,
		ResourceType:  resourceType,
		ResourceID:    resourceID,
		BizCode:       code,
		RequestID:     r.RequestID,
		CorrelationID: r.CorrelationID,
		UserAgent:     r.UserAgent,
	}
}

func bindDelegatedBody(c *gin.Context, dst any) (payloadHash string, ok bool) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "請求內容無法讀取")
		return "", false
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "請求內容格式不正確")
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}

func validateOperationReason(c *gin.Context, reason string) (string, bool) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "請填寫本次操作的原因")
		return "", false
	}
	if len([]rune(trimmed)) > 100 {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "操作原因不得超過 100 字")
		return "", false
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "操作原因不得包含控制字元")
			return "", false
		}
	}
	return trimmed, true
}

func validateExpectedVersion(c *gin.Context, version string) bool {
	if !delegatedVersionRE.MatchString(version) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "expectedVersion 格式不正確")
		return false
	}
	return true
}

func notifySellerOrDefault(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

func delegatedPaging(c *gin.Context) (page, pageSize int32, ok bool) {
	page, pageSize = 1, delegatedDefaultPageSize

	if raw := c.Query("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "page 必須是大於 0 的整數")
			return 0, 0, false
		}
		page = int32(n)
	}
	if raw := c.Query("page_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < delegatedMinPageSize || n > maxPageSize {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "page_size 必須介於 1 與 50 之間")
			return 0, 0, false
		}
		pageSize = int32(n)
	}
	return page, pageSize, true
}

const delegatedMinPageSize = 1

const delegatedDefaultPageSize = 20

const delegatedKeywordMaxRunes = 255

func delegatedKeyword(c *gin.Context) (string, bool) {
	trimmed := strings.TrimSpace(c.Query("keyword"))
	if len([]rune(trimmed)) > delegatedKeywordMaxRunes {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "keyword 不得超過 255 字")
		return "", false
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "keyword 不得包含控制字元")
			return "", false
		}
	}
	return trimmed, true
}

func delegatedDateRange(c *gin.Context) (from, to string, ok bool) {
	from = c.Query("date_from")
	to = c.Query("date_to")
	var fromTime, toTime time.Time
	if from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "date_from 格式不正確")
			return "", "", false
		}
		fromTime = t
	}
	if to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			response.BizStatus(c, bizcode.DelegationInvalidCommand, "date_to 格式不正確")
			return "", "", false
		}
		toTime = t
	}
	if from != "" && to != "" && fromTime.After(toTime) {
		response.BizStatus(c, bizcode.DelegationInvalidCommand, "date_from 不得晚於 date_to")
		return "", "", false
	}
	return from, to, true
}

func denyDelegated(c *gin.Context, d *delegation.Denial) {
	response.BizStatus(c, d.Code, d.Message)
}

func respondDelegatedMutation(c *gin.Context, result *delegation.Result) {
	response.OK(c, delegatedMutationData{
		AuditLogID: result.AuditLogID,
		Version:    result.Version,
		Resource:   result.Resource,
	})
}

type delegatedProductMutationData struct {
	AuditLogID string `json:"auditLogId"`
	Version    string `json:"version"`
	Resource   any    `json:"resource"`
	CertificationWarnings []CertificationWarningOutput `json:"certificationWarnings"`
}

func respondDelegatedProductMutation(c *gin.Context, result *delegation.Result, warnings []CertificationWarningOutput) {
	if warnings == nil {
		warnings = []CertificationWarningOutput{}
	}
	response.OK(c, delegatedProductMutationData{
		AuditLogID:            result.AuditLogID,
		Version:               result.Version,
		Resource:              result.Resource,
		CertificationWarnings: warnings,
	})
}

func delegationDenialFromCode(code int32, message string) *delegation.Denial {
	if code == 0 {
		return nil
	}
	return delegation.Deny(code, message)
}

func upstreamDenial() *delegation.Denial {
	return delegation.Deny(bizcode.DelegationUpstreamUnavailable, "代行服務暫時不可用，請稍後再試")
}

var delegatedBizMsgRE = regexp.MustCompile(`^\[(\d+)\]\s*(.*)$`)

func denialFromGrpcErr(err error) *delegation.Denial {
	st, ok := grpcstatus.FromError(err)
	if !ok {
		return upstreamDenial()
	}
	switch st.Code() {
	case grpccodes.Unavailable, grpccodes.DeadlineExceeded, grpccodes.Internal, grpccodes.Unknown:
		return upstreamDenial()
	}
	if m := delegatedBizMsgRE.FindStringSubmatch(st.Message()); m != nil {
		if n, convErr := strconv.ParseInt(m[1], 10, 32); convErr == nil {
			return delegation.Deny(int32(n), m[2])
		}
	}
	switch st.Code() {
	case grpccodes.InvalidArgument:
		return delegation.Deny(bizcode.DelegationInvalidCommand, st.Message())
	case grpccodes.PermissionDenied, grpccodes.NotFound:
		return delegation.Deny(bizcode.DelegationResourceNotOwned, st.Message())
	}
	zap.L().Warn("Delegation.upstream.unmapped",
		zap.String("grpcCode", st.Code().String()), zap.String("grpcMessage", st.Message()))
	return upstreamDenial()
}

func mapDomainItemCode(code int32) int32 {
	switch code {
	case 0:
		return 0
	case 400:
		return bizcode.DelegationInvalidCommand
	case 403, 404:
		return bizcode.DelegationResourceNotOwned
	default:
		return code
	}
}

func joinForDisplay(values []string) string {
	return strings.Join(values, "、")
}
