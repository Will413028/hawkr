package pagination

import (
	"errors"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestPageSize(t *testing.T) {
	cases := []struct{ size, want int32 }{{0, 12}, {5, 5}, {50, 50}, {999, 50}}
	for _, c := range cases {
		got, err := PageSize(c.size, 12, 50)
		if err != nil || got != c.want {
			t.Errorf("PageSize(%d) = %d, %v，預期 %d", c.size, got, err, c.want)
		}
	}
	_, err := PageSize(-1, 12, 50)
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("PageSize(-1) = %v，預期 InvalidInput", err)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || len(ae.FieldViolations()) != 1 || ae.FieldViolations()[0].Field != "PageSize" {
		t.Errorf("負數的錯誤要指到欄位 PageSize：%v", err)
	}
}
