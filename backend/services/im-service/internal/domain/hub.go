package domain

// PresenceKey identifies a user + role pair. Presence is tracked per-role so
// that a buyer chatting with a seller only sees "seller app online", not
// "user's buyer app online".
type PresenceKey struct {
	UserID    int64
	LoginRole int32 // 1=buyer, 2=seller
}

// Client represents a connected Gateway-to-IM chat stream client.
type Client struct {
	Conn               Connection
	Send               chan []byte
	SubscribedPresence map[PresenceKey]struct{} // guarded by hub mutex
	UserID             int64
	LoginRole          int32 // 1=buyer, 2=seller; 0=unknown (backwards compat)
}

// Hub defines the interface for managing Gateway-to-IM streams and message routing.
type Hub interface {
	Run()
	Register(client *Client)
	Unregister(client *Client)
	SendToUser(userID int64, message *Message) bool
	SendToUserRole(userID int64, loginRole int32, message *Message) bool
	Broadcast(message []byte)
	BroadcastExcept(message *Message, exclude map[int64]bool) int
	IsOnline(userID int64) bool
	ConnectionCount() int
	DisconnectUser(userID int64) bool
	SubscribePresence(client *Client, peer PresenceKey)
	UnsubscribePresence(client *Client, peer PresenceKey)
}
