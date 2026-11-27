package rpc

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)


const errorDomain = "hawkr"

// LocalizedMessage 回傳給使用者看的文字；locale 一律取自文字本身（Message.Locale 是實際使用的語系，缺字退回 zh-TW 時就是 zh-TW，
// ADR 2026-10-17-multi-locale-architecture D3），不接受另外傳入的語系。錯誤的 detail 與回應裡非錯誤的使用者文字（例如動作不能做的原因）用同一個型別。
func LocalizedMessage(m uitext.Message) *errdetails.LocalizedMessage {
	return &errdetails.LocalizedMessage{Locale: string(m.Locale), Message: m.Text}
}

// CommonText 回 Connect code 的預設使用者文字：共用文案目錄的 common.error.<CODE>，語系取自 ctx 的請求語系（uitext.LocaleFrom）。
// 傳輸層自己判斷的錯誤（未登入、無權操作、服務暫時不可用、系統錯誤）的文字只有這一個來源。
func CommonText(ctx context.Context, code connect.Code) uitext.Message {
	m, _ := uitext.For(uitext.LocaleFrom(ctx)).CommonError(strings.ToUpper(code.String()))
	return m
}

// CommonError 回帶預設文字（CommonText）的 Connect 錯誤；給入口中介層自己寫回應時用。
func CommonError(ctx context.Context, code connect.Code) *connect.Error {
	return toConnect(ctx, userError(code, CommonText(ctx, code))).err
}

// CommonFailure 回帶預設文字（CommonText）的錯誤值，給 handler 以 return 回傳，由 interceptor 轉成 Connect 錯誤；
// 語系取自 ctx。與 CommonError 不同：CommonError 回的 *connect.Error 不是 toConnect 認得的來源。
func CommonFailure(ctx context.Context, code connect.Code) error {
	return userError(code, CommonText(ctx, code))
}

type transportError struct {
	code      connect.Code
	message   string
	localized uitext.Message
}

func (e *transportError) Error() string { return e.code.String() + ": " + e.message }

// Error 回傳傳輸層判斷的錯誤，例如 ID 格式錯、page token 無法解讀、修改沒帶 etag。這些是 client 的錯，
// message 只給開發者看，不帶 LocalizedMessage：client 顯示該 code 的預設文字。
func Error(code connect.Code, format string, args ...any) error {
	return &transportError{code: code, message: fmt.Sprintf(format, args...)}
}

// UserError 回傳 Connect 的錯誤，帶給使用者看的文字（LocalizedMessage）；給 interceptor 之外、自己寫回應的
// 中介層用（例如入口在解析 session 失敗時回 unavailable）。
func UserError(code connect.Code, text string) *connect.Error {
	return toConnect(context.Background(), userError(code, uitext.Message{Text: text, Locale: uitext.ZhTW})).err
}

// ConnectError 把模組的錯誤（apperr）轉成 Connect 的錯誤，與 interceptor 的轉換相同；給入口中介層自己寫回應、
// 又需要帶 ErrorInfo 的 reason 時用。這條路徑在 handler 之外，沒有請求訊息，所以欄位錯誤只有明列的 FieldPaths 能對應
// （沒有自動的同名對應）；入口中介層的錯誤（session 解析）不是欄位錯誤，沒有缺口。
func ConnectError(ctx context.Context, err error) *connect.Error {
	return toConnect(ctx, err).err
}

// RetryLater 回傳 resource_exhausted，帶共用文案目錄的預設文字（CommonText，語系取自 ctx）與 RetryInfo（wait 之後再試）；給入口的限流用。
func RetryLater(ctx context.Context, wait time.Duration) *connect.Error {
	ce := CommonError(ctx, connect.CodeResourceExhausted)
	addDetail(ce, &errdetails.RetryInfo{RetryDelay: durationpb.New(wait)})
	return ce
}

func userError(code connect.Code, m uitext.Message) error {
	return &transportError{code: code, message: m.Text, localized: m}
}

var kindCodes = map[apperr.Kind]connect.Code{
	apperr.KindInvalidInput:       connect.CodeInvalidArgument,
	apperr.KindNotFound:           connect.CodeNotFound,
	apperr.KindVersionConflict:    connect.CodeAborted,
	apperr.KindAlreadyExists:      connect.CodeAlreadyExists,
	apperr.KindFailedPrecondition: connect.CodeFailedPrecondition,
	apperr.KindUnauthenticated:    connect.CodeUnauthenticated,
	apperr.KindPermissionDenied:   connect.CodePermissionDenied,
	apperr.KindUnavailable:        connect.CodeUnavailable,
	apperr.KindResourceExhausted:  connect.CodeResourceExhausted,
}

type converted struct {
	err      *connect.Error
	internal bool
	unmapped []string
}

func toConnect(ctx context.Context, err error) converted {
	if err == nil {
		return converted{}
	}
	var te *transportError
	if errors.As(err, &te) {
		ce := connect.NewError(te.code, errors.New(te.message))
		if te.localized.Text != "" {
			addDetail(ce, LocalizedMessage(te.localized))
		}
		return converted{err: ce}
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		if code, ok := kindCodes[ae.Kind()]; ok {
			var fp *fieldPathsError
			var paths FieldPaths
			if errors.As(err, &fp) {
				paths = fp.paths
			}
			var re *requestError
			var desc protoreflect.MessageDescriptor
			if errors.As(err, &re) {
				desc = re.desc
			}
			ce, unmapped := fromAppErr(code, ae, paths, desc)
			return converted{err: ce, unmapped: unmapped}
		}
	}
	switch {
	case errors.Is(err, context.Canceled):
		return converted{err: connect.NewError(connect.CodeCanceled, err)}
	case errors.Is(err, context.DeadlineExceeded):
		m := CommonText(ctx, connect.CodeDeadlineExceeded)
		ce := connect.NewError(connect.CodeDeadlineExceeded, errors.New(m.Text))
		addDetail(ce, LocalizedMessage(m))
		return converted{err: ce}
	}
	return converted{err: connect.NewError(connect.CodeInternal, errors.New(CommonText(ctx, connect.CodeInternal).Text)), internal: true}
}

func fromAppErr(code connect.Code, ae *apperr.Error, paths FieldPaths, request protoreflect.MessageDescriptor) (*connect.Error, []string) {
	ce := connect.NewError(code, errors.New(ae.Message))
	if ae.Reason != (apperr.Reason{}) {
		addDetail(ce, &errdetails.ErrorInfo{Reason: ae.Reason.String(), Domain: errorDomain, Metadata: ae.Metadata})
	}
	addDetail(ce, LocalizedMessage(uitext.Message{Text: ae.Message, Locale: ae.Locale()}))
	if ae.RetryAfter > 0 {
		addDetail(ce, &errdetails.RetryInfo{RetryDelay: durationpb.New(ae.RetryAfter)})
	}
	var violations []*errdetails.BadRequest_FieldViolation
	var unmapped []string
	for _, v := range ae.FieldViolations() {
		path, ok := paths.path(v.Field)
		if !ok {
			path, ok = sameNameField(request, v.Field)
		}
		if !ok {
			unmapped = append(unmapped, v.Field)
			continue
		}
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:            path,
			Description:      v.Message,
			LocalizedMessage: LocalizedMessage(uitext.Message{Text: v.Message, Locale: ae.Locale()}),
		})
	}
	if len(violations) > 0 {
		addDetail(ce, &errdetails.BadRequest{FieldViolations: violations})
	}
	return ce, unmapped
}

func addDetail(ce *connect.Error, m proto.Message) {
	d, err := connect.NewErrorDetail(m)
	if err != nil {
		panic(err)
	}
	ce.AddDetail(d)
}

// FieldPaths 把模組回報的欄位（apperr.FieldViolation.Field）對應到請求訊息的路徑（ADR
// 2026-09-25-api-error-model D4）。key 是模組的欄位名、索引寫成 []；value 是請求裡的 proto 路徑，
// 其中的 [] 依序填入模組回報的索引，例如 "Banners[].Title" → "banners[].title"。repeated 欄位只列一筆結尾帶 []
// 的對應："EvidenceTokens[]" → "evidence_staged_tokens[]" 同時涵蓋單一元素的錯誤（"EvidenceTokens[2]" →
// "evidence_staged_tokens[2]"）與整個欄位的錯誤（"EvidenceTokens" → "evidence_staged_tokens"）。單值欄位被回報成索引欄位
// （"Reason[2]"）或沒有對應的欄位對應不到，會被當成沒有對應的欄位記錄。
type FieldPaths map[string]string

func sameNameField(request protoreflect.MessageDescriptor, field string) (string, bool) {
	if request == nil || field == "" {
		return "", false
	}
	for _, r := range field {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", false
		}
	}
	name := snakeCase(field)
	if request.Fields().ByName(protoreflect.Name(name)) == nil {
		return "", false
	}
	return name, true
}

func snakeCase(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		upper := r >= 'A' && r <= 'Z'
		if upper && i > 0 {
			prev := rs[i-1]
			prevLowerOrDigit := prev >= 'a' && prev <= 'z' || prev >= '0' && prev <= '9'
			nextLower := i+1 < len(rs) && rs[i+1] >= 'a' && rs[i+1] <= 'z'
			if prevLowerOrDigit || (prev >= 'A' && prev <= 'Z' && nextLower) {
				b.WriteByte('_')
			}
		}
		if upper {
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

type requestError struct {
	err  error
	desc protoreflect.MessageDescriptor
}

func (e *requestError) Error() string { return e.err.Error() }
func (e *requestError) Unwrap() error { return e.err }

func withRequest(err error, req any) error {
	if err == nil {
		return nil
	}
	if m, ok := req.(proto.Message); ok {
		return &requestError{err: err, desc: m.ProtoReflect().Descriptor()}
	}
	return err
}

// WithFieldPaths 讓 err 裡的欄位錯誤以 paths 對應到請求的路徑。會回欄位錯誤的模組呼叫，handler 都要
// 這樣包；對應不到的欄位不放進 BadRequest，interceptor 以 error 等級記錄。
func WithFieldPaths(err error, paths FieldPaths) error {
	if err == nil {
		return nil
	}
	return &fieldPathsError{err: err, paths: paths}
}

type fieldPathsError struct {
	err   error
	paths FieldPaths
}

func (e *fieldPathsError) Error() string { return e.err.Error() }
func (e *fieldPathsError) Unwrap() error { return e.err }

var fieldIndex = regexp.MustCompile(`\[\d+\]`)

func (p FieldPaths) path(field string) (string, bool) {
	indexes := fieldIndex.FindAllString(field, -1)
	key := fieldIndex.ReplaceAllString(field, "[]")
	path, ok := p[key]
	if !ok && !strings.HasSuffix(key, "[]") {
		if elem, found := p[key+"[]"]; found {
			path, ok = strings.CutSuffix(elem, "[]")
		}
	}
	if !ok || strings.Count(path, "[]") != len(indexes) {
		return "", false
	}
	for _, i := range indexes {
		path = strings.Replace(path, "[]", i, 1)
	}
	return path, true
}

// CheckFieldPaths 回傳 paths 裡在請求訊息 desc 不存在的路徑，給 handler 的測試用：proto 改名時對應表
// 要跟著改。
func CheckFieldPaths(desc protoreflect.MessageDescriptor, paths FieldPaths) []string {
	var missing []string
	for _, path := range paths {
		if !pathExists(desc, path) {
			missing = append(missing, path)
		}
	}
	slices.Sort(missing)
	return missing
}

func pathExists(desc protoreflect.MessageDescriptor, path string) bool {
	parts := strings.Split(path, ".")
	for i, p := range parts {
		name, indexed := strings.CutSuffix(p, "[]")
		f := desc.Fields().ByName(protoreflect.Name(name))
		if f == nil || (indexed && !f.IsList()) {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		if f.IsList() && !indexed {
			return false
		}
		if f.Kind() != protoreflect.MessageKind {
			return false
		}
		desc = f.Message()
	}
	return false
}
