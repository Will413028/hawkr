//go:build integration

package social_test

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/reads"
	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/visibility"
)

// rowFields 攤平 row struct（內嵌的 struct 展開）成「pgx 對應用的欄位名 → 型別」，規則同 pgx.RowToStructByName：有 db tag 用 tag，
// 沒有就用欄位名去掉底線後不分大小寫比對。
type rowField struct {
	key string
	typ reflect.Type
}

func rowFields(t reflect.Type) []rowField {
	var out []rowField
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct {
			out = append(out, rowFields(sf.Type)...)
			continue
		}
		if sf.PkgPath != "" {
			continue
		}
		name := sf.Name
		if tag, ok := sf.Tag.Lookup("db"); ok {
			name = tag
		}
		out = append(out, rowField{key: normalizeColumn(name), typ: sf.Type})
	}
	return out
}

func normalizeColumn(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }

// scansInto：Postgres 欄位型別（OID）可以掃進 Go 型別（指標算它指向的型別；Describe 看不出欄位可不可以是 NULL，所以指標或非指標都接受）。
func scansInto(oid uint32, t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch oid {
	case pgtype.TextOID, pgtype.VarcharOID:
		return t.Kind() == reflect.String
	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID:
		return slices.Contains([]reflect.Kind{reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int}, t.Kind())
	case pgtype.BoolOID:
		return t.Kind() == reflect.Bool
	case pgtype.UUIDOID:
		return t == reflect.TypeOf(uuid.UUID{})
	case pgtype.TimestamptzOID:
		return t == reflect.TypeOf(time.Time{})
	}
	return false
}

// 結果的形狀（sqlc 在生成時給的保證，改成 go-jet 後由這個測試在 go test 時給）：每支登記的 query 對測試資料庫 Describe（只問 Postgres 這句
// 回什麼欄位，不執行、不需要資料），欄位名稱與型別必須和 row struct 一一對應。pgx 的 RowToStructByName 在零列時不會檢查，
// 所以這裡不靠執行。
func TestRegisteredQueriesDescribeTheirRowStructs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	conn, err := f.db.Pool().Acquire(ctx)
	must(t, err)
	defer conn.Release()
	for _, e := range reads.Registry() {
		for _, v := range []visibility.Viewer{visibility.Anonymous, {BuyerID: 42}} {
			q, _ := e.Build(v).Sql()
			sd, err := conn.Conn().PgConn().Prepare(ctx, "", q, nil)
			if err != nil {
				t.Errorf("%s：Postgres 不接受這句 SQL：%v", e.Name, err)
				continue
			}
			fields := rowFields(reflect.TypeOf(e.Row))
			byKey := map[string]rowField{}
			for _, fl := range fields {
				byKey[fl.key] = fl
			}
			seen := map[string]bool{}
			for _, col := range sd.Fields {
				key := normalizeColumn(col.Name)
				fl, ok := byKey[key]
				switch {
				case !ok:
					t.Errorf("%s：欄位 %q 在 %T 裡沒有對應的欄位", e.Name, col.Name, e.Row)
				case !scansInto(col.DataTypeOID, fl.typ):
					t.Errorf("%s：欄位 %q（OID %d）掃不進 %s", e.Name, col.Name, col.DataTypeOID, fl.typ)
				}
				seen[key] = true
			}
			for _, fl := range fields {
				if !seen[fl.key] {
					t.Errorf("%s：%T 的欄位 %q 沒有對應的查詢欄位", e.Name, e.Row, fl.key)
				}
			}
			if len(sd.Fields) != len(fields) {
				t.Errorf("%s：查詢 %d 個欄位，%T 有 %d 個", e.Name, len(sd.Fields), e.Row, len(fields))
			}
		}
	}
}
