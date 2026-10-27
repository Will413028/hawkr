package domain

const (
	UnitCatty = "catty"

	UnitHectokilogram = "100kg"
)

var kgPerUnit = map[string]float64{
	UnitKilogram:      1,
	UnitCatty:         0.6,
	UnitHectokilogram: 100,
}

func CanonicalPricePerKg(price float64, unit string) (float64, bool) {
	kg, ok := kgPerUnit[unit]
	if !ok || kg <= 0 {
		return 0, false
	}
	return price / kg, true
}

func CanonicalUnitFor(unit string) string {
	if _, ok := kgPerUnit[unit]; !ok {
		return ""
	}
	return UnitKilogram
}

func KgPerUnit(unit string) (float64, bool) {
	kg, ok := kgPerUnit[unit]
	return kg, ok
}
