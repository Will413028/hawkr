package dto

// RecommendedProduct is a single product recommendation.
type RecommendedProduct struct {
	Name       string  `json:"name,omitempty"`
	ImageURL   string  `json:"image_url,omitempty"`
	SellerName string  `json:"seller_name,omitempty"`
	SellerLogo string  `json:"seller_logo,omitempty"`
	Reason     string  `json:"reason"`
	ProductID  int64   `json:"product_id"`
	PriceMin   int64   `json:"price_min,omitempty"`
	PriceMax   int64   `json:"price_max,omitempty"`
	Rating     float64 `json:"rating,omitempty"`
	Score      float64 `json:"score"`
}

// PersonalizedSection represents a section of personalized recommendations.
type PersonalizedSection struct {
	Title    string               `json:"title"`
	Type     string               `json:"type"`
	Products []RecommendedProduct `json:"products"`
}

// PersonalizedResponse is the response for personalized recommendations.
type PersonalizedResponse struct {
	ModelVersion string                `json:"model_version"`
	Sections     []PersonalizedSection `json:"sections"`
}
