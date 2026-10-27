package ricecity

import (
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/sourcemarket"
)

var (
	ErrDigestMismatch = sourcemarket.ErrDigestMismatch
	ErrUnknownMarket  = sourcemarket.ErrUnknownMarket
)

type Registry struct{ sourcemarket.Registry }

func Load(csvPath, manifestPath string) (Registry, error) {
	r, err := sourcemarket.Load("糧價縣市", csvPath, manifestPath)
	if err != nil {
		return Registry{}, err
	}
	return Registry{Registry: r}, nil
}
