package characterize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// REST 以舊 gateway 的 REST API（統一 envelope，經單體的入口）發請求，是模組 REST driver 的基底。身分以 session
// 的 Bearer token 帶入（DevAuth、EnvTokens）。
type REST struct {
	Base   string
	Tokens map[Caller]string
	HTTP   *http.Client
	// Business 把模組自己的業務碼對應到錯誤的分類（回 nil 表示不是這個模組的業務碼，照 RESTKind 分類）。
	Business func(code int32) error
}

type envelope struct {
	Code    int32           `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Do 送出請求，把 envelope 的 data 解到 out（out 為 nil 時不解）。失敗時的錯誤以 RESTKind 分類。
func (r *REST) Do(ctx context.Context, c Caller, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.Base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	Bearer(r.Tokens)(c, req.Header)
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s %s：回應不是 envelope（HTTP %d）：%s", method, path, resp.StatusCode, raw)
	}
	if resp.StatusCode >= 400 || env.Code != 0 {
		kind := RESTKind(resp.StatusCode, env.Code)
		if r.Business != nil {
			if k := r.Business(env.Code); k != nil {
				kind = k
			}
		}
		return fmt.Errorf("%w：%s %s → HTTP %d、code %d：%s", kind, method, path, resp.StatusCode, env.Code, env.Message)
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// RESTKind 依 envelope 的 code 與 HTTP status 分類錯誤（CodeMeta 的通用碼與 HTTP 同號）。模組的業務碼
// 由模組的 REST driver 自己先對應。
func RESTKind(status int, code int32) error {
	for _, c := range []int{int(code), status} {
		switch c {
		case http.StatusUnauthorized:
			return ErrUnauthenticated
		case http.StatusForbidden:
			return ErrPermission
		case http.StatusNotFound:
			return ErrNotFound
		case http.StatusBadRequest:
			return ErrInvalid
		case http.StatusConflict:
			return ErrConflict
		}
	}
	return fmt.Errorf("未分類的錯誤")
}
