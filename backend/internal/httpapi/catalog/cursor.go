package catalog

import (
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

type cursor struct {
	Key int64     `json:"k"`
	ID  uuid.UUID `json:"i"`
}

func (c cursor) Valid() bool { return c.ID != (uuid.UUID{}) }

func decodeCursor(scope, token string) (*catalog.ListCursor, error) {
	c, err := rpc.ValidCursor[cursor](scope, token)
	if err != nil || c == nil {
		return nil, err
	}
	return &catalog.ListCursor{Key: c.Key, ID: c.ID}, nil
}

func nextToken(scope string, next *catalog.ListCursor) string {
	if next == nil {
		return ""
	}
	return rpc.TokenFor(scope, cursor{Key: next.Key, ID: next.ID})
}

func checkPageSize(n int32) error {
	if n < 0 {
		return fieldError("page_size", "page_size 不得為負數")
	}
	return nil
}

func checkMaxResults(n int32) error {
	if n < 0 {
		return fieldError("max_results", "max_results 不得為負數")
	}
	return nil
}

// DecodeCursor 與 NextToken 給 buyer 的「追蹤中」動態用：同一種游標與 token 編碼，scope 由呼叫端綁定（買家與 procedure）。
func DecodeCursor(scope, token string) (*catalog.ListCursor, error) {
	return decodeCursor(scope, token)
}

// NextToken 見 DecodeCursor。
func NextToken(scope string, next *catalog.ListCursor) string { return nextToken(scope, next) }
