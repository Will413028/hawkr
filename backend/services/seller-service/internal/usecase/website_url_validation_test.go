package usecase

import (
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

func TestValidateWebsiteURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty string is valid (clears the field)", raw: "", wantErr: false},
		{name: "valid https url", raw: "https://example.com", wantErr: false},
		{name: "valid http url", raw: "http://example.com/shop", wantErr: false},
		{name: "missing scheme", raw: "example.com", wantErr: true},
		{name: "non-http(s) scheme", raw: "javascript:alert(1)", wantErr: true},
		{name: "scheme without host", raw: "https://", wantErr: true},
		{name: "not a url at all", raw: "not a url", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebsiteURL(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatalf("validateWebsiteURL(%q) = nil, want error", tt.raw)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateWebsiteURL(%q) = %v, want nil", tt.raw, err)
			}
			if tt.wantErr {
				if pkgerrors.GetCode(err) != pkgerrors.ErrorCode(bizcode.SellerWebsiteUrlInvalid) {
					t.Errorf("GetCode(err) = %v, want SellerWebsiteUrlInvalid (%d)", pkgerrors.GetCode(err), bizcode.SellerWebsiteUrlInvalid)
				}
			}
		})
	}
}
