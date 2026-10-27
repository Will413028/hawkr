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

	ResolvedLabel string  `json:"resolvedLabel,omitempty"`
	CropCode      *string `json:"cropCode,omitempty"`
	GroupCode     *string `json:"groupCode,omitempty"`

	FallbackGroupCode *string `json:"fallbackGroupCode,omitempty"`

	EntityCategoryID string `json:"entityCategoryId,omitempty"`
}
