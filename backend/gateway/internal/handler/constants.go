package handler

// 分頁預設值（字串，用於 c.DefaultQuery）
const (
	defaultPageStr     = "1"
	defaultPageSizeStr = "10"
)

// 分頁驗證上下限
const (
	maxPageSize     = 50
	defaultPageSize = 10
	defaultLimit    = 10
)
