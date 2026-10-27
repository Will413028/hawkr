package httputil

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestParseAccountID(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantID int64
		wantOK bool
	}{
		{"正常 ID", "12345", 12345, true},
		{"空字串", "", 0, false},
		{"零值", "0", 0, false},
		{"非數字", "abc", 0, false},
		{"負數", "-1", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ParseAccountID(tt.input)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("ParseAccountID(%q) = (%d, %v), want (%d, %v)", tt.input, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestParseAccountType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int32
	}{
		{"BUYER", "1", 1},
		{"SELLER", "2", 2},
		{"SELLER_AND_BUYER", "3", 3},
		{"ADMIN", "4", 4},
		{"空字串", "", 0},
		{"非數字", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAccountType(tt.input)
			if got != tt.want {
				t.Errorf("ParseAccountType(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseStoreID(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   identity.StoreID
		wantOK bool
	}{
		{"valid", "8001", 8001, true},
		{"zero is not a valid store", "0", 0, false},
		{"empty", "", 0, false},
		{"garbage", "abc", 0, false},
		{"negative is still parsed", "-1", -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseStoreID(tc.in)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseStoreID(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseBuyerID(t *testing.T) {
	got, ok := ParseBuyerID("55321")
	if got != identity.BuyerID(55321) || !ok {
		t.Errorf("ParseBuyerID(\"55321\") = (%d, %v), want (55321, true)", got, ok)
	}
	if _, ok := ParseBuyerID("0"); ok {
		t.Error("ParseBuyerID(\"0\") 應回 ok=false")
	}
}
