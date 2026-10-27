package domain

type QuoteLane string

const (
	QuoteLanePlatform QuoteLane = "PLATFORM"
	QuoteLaneExternal QuoteLane = "EXTERNAL"
)

var QuoteRequiredOptionalSlots = map[SlotKey]bool{
	SlotQuantity:     true,
	SlotDeliveryDate: true,
}
