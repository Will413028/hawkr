package domain

// Message represents a chat message exchanged between Gateway and IM.
type Message struct {
	Data    interface{} `json:"data,omitempty"`
	Online  *bool       `json:"online,omitempty"`
	RoomID  string      `json:"roomId,omitempty"`
	Type    string      `json:"type"`
	Content string      `json:"content,omitempty"`
	// from/to/messageId/seqId 皆為 64-bit TSID（> JS 2^53）。本 struct 經 json.Marshal
	// 直接送 WebSocket frame 給 web/Flutter；web JSON.parse 對 > 2^53 的 number 會精度
	// 丟失（去重 key `${conversationId}_${seqId}` 失準），且與 REST 歷史（proto，送字串）
	// 型別不一致。故 int64 id 一律以 ,string 序列化成 JSON 字串（對齊 notification #960、
	// REST chat history）。timestamp/lastSeenAt 是毫秒 timestamp（< 2^53）維持 number。
	From         int64  `json:"from,omitempty,string"`
	To           int64  `json:"to,omitempty,string"`
	MessageID    int64  `json:"messageId,omitempty,string"`
	SeqID        int64  `json:"seqId,omitempty,string"`
	Timestamp    int64  `json:"timestamp"`
	LastSeenAt   int64  `json:"lastSeenAt,omitempty"`
	MsgType      int32  `json:"msgType,omitempty"`
	PeerRole     int32  `json:"peerRole,omitempty"`
	HasMore      bool   `json:"hasMore,omitempty"`
	SenderName   string `json:"senderName,omitempty"`
	SenderAvatar string `json:"avatarKey,omitempty"` // 發送者頭像 objectKey（49 §6.7 S4：只帶 key，前端組 URL）
	// unread_changed event payload. UnreadCount is a pointer so that a value
	// of 0 (the whole point of a mark-read event) serialises as "unreadCount":0
	// instead of being dropped by omitempty. Role: 1=buyer, 2=seller.
	UnreadCount *int64 `json:"unreadCount,omitempty"`
	Role        int32  `json:"role,omitempty"`
}

// Connection is the transport-neutral lifecycle handle owned by a connected
// client. Gateway owns the public WebSocket; IM only needs to close its local
// gRPC stream when a user is disconnected.
type Connection interface {
	Close() error
}
