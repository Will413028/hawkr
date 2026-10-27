package handler

import "time"

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

// WebSocket 緩衝區大小
const wsBufferSize = 1024

// Public chat WebSocket liveness. The Gateway is the public WebSocket edge and
// owns the single client-side ping/pong deadline while the private IM leg is a
// typed gRPC stream.
const (
	wsProxyWriteWait  = 10 * time.Second
	wsProxyPongWait   = 60 * time.Second
	wsProxyPingPeriod = 25 * time.Second
)
