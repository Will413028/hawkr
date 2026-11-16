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

// 錯誤模型見 ADR 2026-09-25-api-error-model 與 API 設計規範（Connect）§八：code 表示類別，details 用
// google.rpc 的標準型別。ErrorInfo 只在 client 需要分支時帶，給使用者的文字只放在 LocalizedMessage，
// message 給開發者看。

// errorDomain 是 ErrorInfo 的 domain。錯誤只有我們自己的產生者，正式網域上線時才確定。
const errorDomain = "hawkr"

// errorLocale 是 apperr 的文字的語系：apperr 的文字目前都是 zh-TW 字面值。
const errorLocale = uitext.ZhTW

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

// transportError 是傳輸層自己判斷的錯誤。它與模組的 apperr 是 toConnect 僅有的兩種來源：其他錯誤
// （包括下游 client 回的 *connect.Error）一律變成 internal，不把下游的 code 與訊息轉給 client。
type transportError struct {
	code      connect.Code
	message   string         // 給開發者看
	localized uitext.Message // 給使用者看；Text 是空字串表示不帶 LocalizedMessage
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
	return toConnect(context.Background(), userError(code, uitext.Message{Text: text, Locale: errorLocale})).err
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

// userError 是要讓使用者看到文字的傳輸層錯誤（認證、授權、逾時）。
func userError(code connect.Code, m uitext.Message) error {
	return &transportError{code: code, message: m.Text, localized: m}
}

// kindCodes 是模組錯誤的分類對應的 Connect 錯誤碼。
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

// converted 是 toConnect 的結果。internal 為 true 表示這是沒有分類的錯誤：回給 client 的訊息是固定的，
// 細節只進 log。unmapped 是模組回報、但沒有附給 client 的欄位（handler 沒有對應到請求路徑）：
// client 只拿到摘要文字，這是 handler 的 bug，interceptor 以 error 等級記錄。
type converted struct {
	err      *connect.Error
	internal bool
	unmapped []string
}

// toConnect 把 handler 回傳的錯誤轉成 Connect 的錯誤，這是單體唯一的轉換點。
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
	addDetail(ce, LocalizedMessage(uitext.Message{Text: ae.Message, Locale: errorLocale}))
	if ae.RetryAfter > 0 {
		addDetail(ce, &errdetails.RetryInfo{RetryDelay: durationpb.New(ae.RetryAfter)})
	}
	// BadRequest 只屬於 invalid_argument（API 設計規範「八、錯誤」）：欄位只能由 InvalidInputOn、InvalidInputFields
	// 在建立時帶，分類建立後不能改，所以有欄位的錯誤一定是 invalid_argument。
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
			LocalizedMessage: LocalizedMessage(uitext.Message{Text: v.Message, Locale: errorLocale}),
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
		// 只有訊息無法序列化時才會發生；這些都是固定的標準型別。
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

// sameNameField 是沒有明列對應時的自動規則：模組的欄位名是單純的識別字（沒有 []、沒有 .），而且它的 snake_case
// 寫法（PageSize → page_size、RequestID → request_id）是請求訊息的頂層欄位，就對應到那個欄位。明列的 FieldPaths 優先；
// 請求沒有同名欄位時不對應（例如請求欄位叫 max_results 的 PageSize），由 handler 以 WithFieldPaths 列對應。
// 請求訊息未知（request 為 nil：沒有請求訊息可對照的路徑，例如入口中介層的 ConnectError）時不對應。
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

// snakeCase 把 Go 欄位名轉成 proto 欄位名：連續的大寫視為一個詞（RequestID → request_id）。
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

// requestError 讓轉換看得到請求訊息的 descriptor，由 interceptor 在 handler 回錯後附上。
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
		// 整個 repeated 欄位的錯誤：找結尾帶 [] 的那一筆，去掉結尾的 []。
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

// pathExists 檢查以 . 分隔的欄位路徑在訊息裡存在，而且帶 [] 的每一段都是 repeated 欄位；中間的 repeated 段一定要帶 []，
// 只有最後一段可以不帶（整個欄位的錯誤指到 repeated 欄位本身）。
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
