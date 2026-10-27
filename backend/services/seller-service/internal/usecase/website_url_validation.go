package usecase

import (
	"net/url"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// validateWebsiteURL checks the seller-supplied website URL format. Empty
// string is valid (clears the field, falls back to the platform storefront
// page). Non-empty must be an absolute http(s) URL with a host.
func validateWebsiteURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return pkgerrors.NewBusiness(bizcode.SellerWebsiteUrlInvalid, "網站網址格式不正確")
	}
	return nil
}
