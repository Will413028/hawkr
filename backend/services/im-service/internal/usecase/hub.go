package usecase

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/Will413028/hawkr/im-service/internal/domain"
	"go.uber.org/zap"
)

// MessageHub manages all Gateway-to-IM chat streams.
// One userID can have multiple connections (e.g. buyer + seller app).
type MessageHub struct {
	clients      map[int64]map[*domain.Client]struct{}
	presenceSubs map[domain.PresenceKey]map[*domain.Client]struct{}
	lastSeen     map[domain.PresenceKey]time.Time
	broadcast    chan []byte
	register     chan *domain.Client
	unregister   chan *domain.Client
	mu           sync.RWMutex
}

// NewMessageHub creates a new MessageHub instance.
func NewMessageHub() *MessageHub {
	return &MessageHub{
		clients:      make(map[int64]map[*domain.Client]struct{}),
		presenceSubs: make(map[domain.PresenceKey]map[*domain.Client]struct{}),
		lastSeen:     make(map[domain.PresenceKey]time.Time),
		broadcast:    make(chan []byte, 256),
		register:     make(chan *domain.Client),
		unregister:   make(chan *domain.Client),
	}
}

// Run starts the hub's main loop.
func (h *MessageHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			conns := h.clients[client.UserID]
			if conns == nil {
				conns = make(map[*domain.Client]struct{})
				h.clients[client.UserID] = conns
			}
			// First connection with this (userID, loginRole) → emit online after unlock.
			firstForRole := client.LoginRole > 0
			if firstForRole {
				for c := range conns {
					if c.LoginRole == client.LoginRole {
						firstForRole = false
						break
					}
				}
			}
			conns[client] = struct{}{}
			h.mu.Unlock()
			zap.L().Info("Client registered", zap.Int64("userID", client.UserID), zap.Int("conns", len(conns)), zap.Int("totalUsers", len(h.clients)))
			if firstForRole {
				h.publishPresence(domain.PresenceKey{UserID: client.UserID, LoginRole: client.LoginRole}, true, time.Time{})
			}

		case client := <-h.unregister:
			h.mu.Lock()
			lastForRole := false
			var lastSeenAt time.Time
			if conns, ok := h.clients[client.UserID]; ok {
				if _, exists := conns[client]; exists {
					// drop presence subscriptions held by this client
					for key := range client.SubscribedPresence {
						if subs, ok2 := h.presenceSubs[key]; ok2 {
							delete(subs, client)
							if len(subs) == 0 {
								delete(h.presenceSubs, key)
							}
						}
					}
					client.SubscribedPresence = nil
					delete(conns, client)
					close(client.Send)
					if client.LoginRole > 0 {
						lastForRole = true
						for c := range conns {
							if c.LoginRole == client.LoginRole {
								lastForRole = false
								break
							}
						}
						if lastForRole {
							lastSeenAt = time.Now()
							h.lastSeen[domain.PresenceKey{UserID: client.UserID, LoginRole: client.LoginRole}] = lastSeenAt
						}
					}
					if len(conns) == 0 {
						delete(h.clients, client.UserID)
					}
				}
			}
			h.mu.Unlock()
			zap.L().Info("Client unregistered", zap.Int64("userID", client.UserID), zap.Int("totalUsers", len(h.clients)))
			if lastForRole {
				h.publishPresence(domain.PresenceKey{UserID: client.UserID, LoginRole: client.LoginRole}, false, lastSeenAt)
			}

		case message := <-h.broadcast:
			// RLock + snapshot 避免 block 其他讀操作（SendToUser、IsOnline）
			h.mu.RLock()
			allClients := make([]*domain.Client, 0)
			for _, conns := range h.clients {
				for client := range conns {
					allClients = append(allClients, client)
				}
			}
			h.mu.RUnlock()
			for _, client := range allClients {
				select {
				case client.Send <- message:
				default:
					zap.L().Warn("chat stream broadcast dropped", zap.Int64("userID", client.UserID))
				}
			}
		}
	}
}

// Register adds a new client.
func (h *MessageHub) Register(client *domain.Client) {
	h.register <- client
}

// Unregister removes a client.
func (h *MessageHub) Unregister(client *domain.Client) {
	h.unregister <- client
}

// Broadcast sends a message to all clients.
func (h *MessageHub) Broadcast(message []byte) {
	h.broadcast <- message
}

// SendToUser sends a message to a specific user (all connections).
func (h *MessageHub) SendToUser(userID int64, message *domain.Message) bool {
	return h.sendToClients(userID, 0, message)
}

// SendToUserRole sends a message only to connections matching the given loginRole.
// loginRole=0 means send to all connections (backwards compat).
func (h *MessageHub) SendToUserRole(userID int64, loginRole int32, message *domain.Message) bool {
	return h.sendToClients(userID, loginRole, message)
}

func (h *MessageHub) sendToClients(userID int64, loginRole int32, message *domain.Message) bool {
	h.mu.RLock()
	conns, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return false
	}
	// Snapshot to avoid holding lock during send
	clients := make([]*domain.Client, 0, len(conns))
	for c := range conns {
		if loginRole == 0 || c.LoginRole == 0 || c.LoginRole == loginRole {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()

	data, err := json.Marshal(message)
	if err != nil {
		return false
	}

	sent := false
	for _, client := range clients {
		select {
		case client.Send <- data:
			sent = true
		default:
			zap.L().Warn("chat stream send channel full, message dropped", zap.Int64("userID", client.UserID))
		}
	}
	return sent
}

// IsOnline checks if a user is online.
func (h *MessageHub) IsOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.clients[userID]
	return ok && len(conns) > 0
}

// ConnectionCount returns the number of active connections.
func (h *MessageHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, conns := range h.clients {
		count += len(conns)
	}
	return count
}

// BroadcastExcept sends a message to all clients except excluded ones.
func (h *MessageHub) BroadcastExcept(message *domain.Message, exclude map[int64]bool) int {
	data, err := json.Marshal(message)
	if err != nil {
		return 0
	}

	h.mu.RLock()
	var targets []*domain.Client
	for userID, conns := range h.clients {
		if exclude[userID] {
			continue
		}
		for client := range conns {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()

	sentCount := 0
	for _, client := range targets {
		select {
		case client.Send <- data:
			sentCount++
		default:
			zap.L().Warn("chat stream broadcast dropped", zap.Int64("userID", client.UserID))
		}
	}
	return sentCount
}

// SubscribePresence registers client as a listener for the given (userID, loginRole)
// and immediately pushes the current presence snapshot to the client.
func (h *MessageHub) SubscribePresence(client *domain.Client, peer domain.PresenceKey) {
	h.mu.Lock()
	subs := h.presenceSubs[peer]
	if subs == nil {
		subs = make(map[*domain.Client]struct{})
		h.presenceSubs[peer] = subs
	}
	subs[client] = struct{}{}
	if client.SubscribedPresence == nil {
		client.SubscribedPresence = make(map[domain.PresenceKey]struct{})
	}
	client.SubscribedPresence[peer] = struct{}{}

	online := false
	if conns, ok := h.clients[peer.UserID]; ok {
		for c := range conns {
			if c.LoginRole == peer.LoginRole {
				online = true
				break
			}
		}
	}
	lastSeenAt := h.lastSeen[peer]
	h.mu.Unlock()

	sendPresenceTo(client, peer, online, lastSeenAt)
}

// UnsubscribePresence removes client from the listener set for the given peer.
func (h *MessageHub) UnsubscribePresence(client *domain.Client, peer domain.PresenceKey) {
	h.mu.Lock()
	if subs, ok := h.presenceSubs[peer]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(h.presenceSubs, peer)
		}
	}
	if client.SubscribedPresence != nil {
		delete(client.SubscribedPresence, peer)
	}
	h.mu.Unlock()
}

// publishPresence fan-outs a presence event to every subscriber of the given peer.
func (h *MessageHub) publishPresence(peer domain.PresenceKey, online bool, lastSeenAt time.Time) {
	h.mu.RLock()
	subs := h.presenceSubs[peer]
	targets := make([]*domain.Client, 0, len(subs))
	for c := range subs {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	data, err := json.Marshal(buildPresenceMessage(peer, online, lastSeenAt))
	if err != nil {
		return
	}
	for _, c := range targets {
		select {
		case c.Send <- data:
		default:
			zap.L().Warn("presence event dropped", zap.Int64("subscriber", c.UserID))
		}
	}
}

func sendPresenceTo(client *domain.Client, peer domain.PresenceKey, online bool, lastSeenAt time.Time) {
	data, err := json.Marshal(buildPresenceMessage(peer, online, lastSeenAt))
	if err != nil {
		return
	}
	select {
	case client.Send <- data:
	default:
		zap.L().Warn("presence snapshot dropped", zap.Int64("subscriber", client.UserID))
	}
}

func buildPresenceMessage(peer domain.PresenceKey, online bool, lastSeenAt time.Time) *domain.Message {
	o := online
	msg := &domain.Message{
		Type:      "presence",
		To:        peer.UserID,
		PeerRole:  peer.LoginRole,
		Online:    &o,
		Timestamp: time.Now().UnixMilli(),
	}
	if !lastSeenAt.IsZero() {
		msg.LastSeenAt = lastSeenAt.UnixMilli()
	}
	return msg
}

// DisconnectUser forcefully disconnects all connections of a user.
func (h *MessageHub) DisconnectUser(userID int64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.clients[userID]
	if !ok {
		return false
	}

	for client := range conns {
		close(client.Send)
		_ = client.Conn.Close()
	}
	delete(h.clients, userID)
	zap.L().Info("User forcefully disconnected", zap.Int64("userID", userID))
	return true
}
