package realtime

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[string]*Client
}

type Client struct {
	id     string
	roomID string
	conn   *websocket.Conn
	send   chan SignalMessage
	mu     sync.Mutex
	closed bool
	once   sync.Once
}

type SignalMessage struct {
	Type    string          `json:"type"`
	PeerID  string          `json:"peer_id,omitempty"`
	From    string          `json:"from,omitempty"`
	To      string          `json:"to,omitempty"`
	Muted   *bool           `json:"muted,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Peers   []PeerInfo      `json:"peers,omitempty"`
}

type PeerInfo struct {
	PeerID string `json:"peer_id"`
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[string]*Client)}
}

func (h *Hub) Join(ctx context.Context, conn *websocket.Conn, roomID string) {
	client := &Client{
		id:     uuid.NewString(),
		roomID: roomID,
		conn:   conn,
		send:   make(chan SignalMessage, 32),
	}
	peers := h.add(client)
	defer h.remove(client)

	client.enqueue(SignalMessage{Type: "joined", PeerID: client.id, Peers: peers})
	h.broadcast(roomID, SignalMessage{Type: "peer_joined", PeerID: client.id}, client.id)

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		client.write(ctx)
	}()

	for {
		var msg SignalMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			break
		}
		msg.From = client.id
		switch msg.Type {
		case "offer", "answer", "ice_candidate":
			if msg.To != "" {
				h.sendTo(roomID, msg.To, msg)
			}
		case "mute":
			h.broadcast(roomID, msg, client.id)
		case "leave":
			client.close(websocket.StatusNormalClosure, "leaving")
			<-writeDone
			return
		}
	}
	client.close(websocket.StatusNormalClosure, "closed")
	<-writeDone
}

func (h *Hub) add(client *Client) []PeerInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	room := h.rooms[client.roomID]
	if room == nil {
		room = make(map[string]*Client)
		h.rooms[client.roomID] = room
	}
	peers := make([]PeerInfo, 0, len(room))
	for id := range room {
		peers = append(peers, PeerInfo{PeerID: id})
	}
	room[client.id] = client
	return peers
}

func (h *Hub) remove(client *Client) {
	h.mu.Lock()
	room := h.rooms[client.roomID]
	if room != nil {
		delete(room, client.id)
		if len(room) == 0 {
			delete(h.rooms, client.roomID)
		}
	}
	h.mu.Unlock()

	client.closeSend()
	h.broadcast(client.roomID, SignalMessage{Type: "peer_left", PeerID: client.id}, client.id)
}

func (h *Hub) sendTo(roomID, peerID string, msg SignalMessage) {
	h.mu.Lock()
	client := h.rooms[roomID][peerID]
	h.mu.Unlock()
	if client != nil {
		client.enqueue(msg)
	}
}

func (h *Hub) broadcast(roomID string, msg SignalMessage, exceptPeerID string) {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.rooms[roomID]))
	for id, client := range h.rooms[roomID] {
		if id != exceptPeerID {
			clients = append(clients, client)
		}
	}
	h.mu.Unlock()
	for _, client := range clients {
		client.enqueue(msg)
	}
}

func (c *Client) enqueue(msg SignalMessage) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	defer c.mu.Unlock()
	select {
	case c.send <- msg:
	default:
		c.close(websocket.StatusPolicyViolation, "client too slow")
	}
}

func (c *Client) write(ctx context.Context) {
	for msg := range c.send {
		if err := wsjson.Write(ctx, c.conn, msg); err != nil {
			return
		}
	}
}

func (c *Client) close(code websocket.StatusCode, reason string) {
	_ = c.conn.Close(code, reason)
}

func (c *Client) closeSend() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		close(c.send)
		c.mu.Unlock()
	})
}
