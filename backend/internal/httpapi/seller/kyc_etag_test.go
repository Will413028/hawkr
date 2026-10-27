package seller

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestKYCAdminEtagRoundTrips(t *testing.T) {
	for _, c := range [][2]int64{{1, 0}, {3, 2}, {12, 345}} {
		v, s, err := parseKYCAdminEtag(kycAdminEtag(c[0], c[1]), "etag")
		if err != nil || v != c[0] || s != c[1] {
			t.Errorf("%v → (%d, %d, %v)", c, v, s, err)
		}
	}
}

func TestParseKYCAdminEtagRejectsMalformedValuesAsInvalidArgument(t *testing.T) {
	for _, bad := range []string{"", "x", "3", "3.", ".1", "0.0", "-1.0", "3.-1", "3.1.1", "03.1", "3.01", "+3.1", " 3.1", "3.1 ", "a.b", "3,1"} {
		_, _, err := parseKYCAdminEtag(bad, "etag")
		if apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%q → %v（要 invalid_argument，不是版本衝突）", bad, err)
		}
	}
}
