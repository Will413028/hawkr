package dto

// SSEEventType represents the type of a Server-Sent Event.
type SSEEventType string

const (
	SSEEventText       SSEEventType = "text"
	SSEEventToolResult SSEEventType = "tool_result"
	SSEEventDone       SSEEventType = "done"
)

// SellerChatSSEEvent is a single SSE event from the seller assistant stream.
type SellerChatSSEEvent struct {
	Type SSEEventType `json:"type"`
	Data string       `json:"data"` // plain text for "text"; JSON string for "tool_result"
}

// OrderListToolResult is the JSON payload for event type "tool_result" with type "order_list".
type OrderListToolResult struct {
	ResultType string            `json:"type"` // "order_list"
	Orders     []OrderItemResult `json:"orders"`
}

// OrderItemResult is a single order in the tool result.
type OrderItemResult struct {
	OrderNo      string `json:"order_no"`
	Status       string `json:"status"`        // human-readable, e.g. "待出貨"
	BuyerName    string `json:"buyer_name"`    // 收件人姓名
	ItemsSummary string `json:"items_summary"` // 商品摘要，如 "A5 和牛燒肉組 x1, 和牛組 x2"
	AmountTWD    int64  `json:"amount_twd"`
	CreateTimeMs int64  `json:"create_time_ms"`
}
