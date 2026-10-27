// pkg/identity/identity_test.go
package identity_test

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestIDTypesAreDistinctAndConvertible(t *testing.T) {
	var a identity.AccountID = 100
	var b identity.BuyerID = 200
	var s identity.StoreID = 300

	if int64(a) != 100 || int64(b) != 200 || int64(s) != 300 {
		t.Fatalf("underlying int64 conversion failed: a=%d b=%d s=%d", a, b, s)
	}

	// 零值語意：resolver fail-open/no-current 情境會回傳零值，呼叫端要能安全比較。
	var zero identity.StoreID
	if zero != 0 {
		t.Fatalf("zero value should equal untyped 0, got %v", zero)
	}
}
