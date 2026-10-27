package domain

// RecommendStrategy defines the recommendation algorithm used.
type RecommendStrategy string

const (
	StrategyPersonalized RecommendStrategy = "personalized"
)

// ModelVersion is the current recommendation model version.
const ModelVersion = "v2.1.0"
