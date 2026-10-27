package discovery

import "github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/ranking"

// 模組擁有的 port（形狀由 discovery 定義，見 internal/ranking/ports.go）。

// Orders 是訂單統計的 port；OrderStats 是它的回傳。
type Orders = ranking.Orders

// OrderStats 是一家店的近期訂單統計。
type OrderStats = ranking.OrderStats

// Products 是「有公開商品的店家」的 port。
type Products = ranking.Products

// Reviews 是評價統計的批次 port（一次最多 MaxRatingBatch 家，回傳只含有評價的店）；RatingStats 是它的回傳。
type Reviews = ranking.Reviews

// MaxRatingBatch 是 Reviews.StoreRatingStats 一次最多接受的店家數。
const MaxRatingBatch = ranking.MaxRatingBatch

// RatingStats 是一家有評價的店的評價統計。
type RatingStats = ranking.RatingStats

// Stores 是店家曝光資格的 port，*seller.Module 直接滿足。
type Stores = ranking.Stores
