package domain

type CropPendingReason string

const (
	CropPendingReasonAmbiguous CropPendingReason = "AMBIGUOUS"

	CropPendingReasonUnavailable CropPendingReason = "UNAVAILABLE"
)

type CropDisambiguationOption struct {
	CategoryID    string `json:"categoryId,omitempty"`
	CategoryLevel int    `json:"categoryLevel,omitempty"`

	Label string `json:"label"`
}
