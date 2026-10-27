package domain

type PriceLayer string

const (
	PriceLayerWholesaleAuction PriceLayer = "wholesale_auction"

	PriceLayerFarmgate PriceLayer = "farmgate"

	PriceLayerLiveAnimal PriceLayer = "live_animal"

	PriceLayerRetail PriceLayer = "retail"

	PriceLayerWholesaleBulk PriceLayer = "wholesale_bulk"
)

func (l PriceLayer) Valid() bool {
	switch l {
	case PriceLayerWholesaleAuction, PriceLayerFarmgate, PriceLayerLiveAnimal,
		PriceLayerRetail, PriceLayerWholesaleBulk:
		return true
	default:
		return false
	}
}

const UnitKilogram = "kg"

const (
	ForecastDerivationModel = "model"

	ForecastDerivationSourcePassthrough = "source_passthrough"
)
