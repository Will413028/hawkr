package dto

type RecommendedSeller struct {
	Name        string
	Logo        string
	CoverKey    string
	Description string
	City        string
	District    string
	StoreID     int64
	StoreCode   int64
	Rating      float64
	OrderCount  int32
	Followers   int32
}
