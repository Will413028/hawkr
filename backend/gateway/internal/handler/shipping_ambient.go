package handler

const ambientShippingTemperature = "3201"

func isAmbientOrUntyped(temperature string) bool {
	return temperature == "" || temperature == ambientShippingTemperature
}
