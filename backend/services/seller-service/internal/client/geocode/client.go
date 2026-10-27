// services/seller-service/internal/client/geocode/client.go
package geocode

import (
	"context"

	pkggeocode "github.com/Will413028/hawkr/go-pkg/geocode"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// Client adapts pkg/geocode's TGOSClient to usecase.GeocodeClient.
type Client struct {
	inner *pkggeocode.TGOSClient
}

// NewClient wires a TGOS geocode client into the usecase-facing adapter.
func NewClient(tgos *pkggeocode.TGOSClient) *Client {
	return &Client{inner: tgos}
}

func (c *Client) Suggest(ctx context.Context, county, district, query string) ([]usecase.Suggestion, error) {
	items, err := c.inner.Suggest(ctx, county, district, query)
	if err != nil {
		return nil, err
	}
	out := make([]usecase.Suggestion, len(items))
	for i, it := range items {
		out[i] = usecase.Suggestion{FullAddress: it.FullAddress, County: it.County, District: it.District, Detail: it.Detail}
	}
	return out, nil
}

func (c *Client) Validate(ctx context.Context, address string) (*usecase.GeocodeValidation, error) {
	r, err := c.inner.Validate(ctx, address)
	if err != nil {
		return nil, err
	}
	return &usecase.GeocodeValidation{
		FullAddress: r.FullAddress,
		County:      r.County,
		District:    r.District,
		Detail:      r.Detail,
		Lat:         r.Lat,
		Lng:         r.Lng,
		Valid:       r.Valid,
	}, nil
}
