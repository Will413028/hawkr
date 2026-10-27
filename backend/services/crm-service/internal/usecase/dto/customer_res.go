package dto

type Customer struct {
	Nickname           string
	Avatar             string
	Phone              string
	Address            string
	Email              string
	CustomerType       string
	Tags               []*Tag
	FrequentCategories []string
	ID                 int64
	StoreID            int64
	BuyerID            int64
	TotalSpent         int64
	OrderCount         int
	CouponCount        int32
}

type PaginatedCustomers struct {
	List  []*Customer
	Total int64
}

type MatchedCustomer struct {
	Nickname string
	Phone    string
	Avatar   string
	BuyerID  int64
}

type SearchCustomersByTagsResult struct {
	Customers []*MatchedCustomer
	Total     int32
}
