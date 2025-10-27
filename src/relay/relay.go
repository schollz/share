package relay

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"github.com/tyler-smith/go-bip39"
)

type Client struct {
	ID       string
	Mnemonic string
	Conn     *websocket.Conn
	RoomID   string
}

type Room struct {
	ID      string
	Clients map[string]*Client
	Mutex   sync.Mutex
}

type IncomingMessage struct {
	Type     string `json:"type"`
	RoomID   string `json:"roomId,omitempty"`
	ClientID string `json:"clientId,omitempty"`
	Pub      string `json:"pub,omitempty"`
	Name     string `json:"name,omitempty"`
	IvB64    string `json:"iv_b64,omitempty"`
	DataB64  string `json:"data_b64,omitempty"`
}

type OutgoingMessage struct {
	Type     string   `json:"type"`
	From     string   `json:"from,omitempty"`
	Mnemonic string   `json:"mnemonic,omitempty"`
	RoomID   string   `json:"roomId,omitempty"`
	Pub      string   `json:"pub,omitempty"`
	Name     string   `json:"name,omitempty"`
	IvB64    string   `json:"iv_b64,omitempty"`
	DataB64  string   `json:"data_b64,omitempty"`
	SelfID   string   `json:"selfId,omitempty"`
	Peers    []string `json:"peers,omitempty"`
	Count    int      `json:"count,omitempty"`
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	rooms   = make(map[string]*Room)
	roomMux sync.Mutex
)

// GenerateMnemonic generates a 2-word BIP39 mnemonic from a client ID
func GenerateMnemonic(clientID string) string {
	hash := sha256.Sum256([]byte(clientID))
	entropy := hash[:16]

	mnemonic, err := bip39.NewMnemonic(entropy[:])
	if err != nil {
		return clientID
	}

	words := strings.Split(mnemonic, " ")
	if len(words) >= 2 {
		return words[0] + "-" + words[1]
	}
	return mnemonic
}

func getOrCreateRoom(roomID string) *Room {
	roomMux.Lock()
	defer roomMux.Unlock()

	if rm, ok := rooms[roomID]; ok {
		return rm
	}
	rm := &Room{
		ID:      roomID,
		Clients: make(map[string]*Client),
	}
	rooms[roomID] = rm
	return rm
}

func removeClientFromRoom(c *Client) {
	if c.RoomID == "" {
		return
	}
	roomMux.Lock()
	room, ok := rooms[c.RoomID]
	roomMux.Unlock()
	if !ok {
		return
	}

	room.Mutex.Lock()
	delete(room.Clients, c.ID)
	empty := len(room.Clients) == 0
	room.Mutex.Unlock()

	broadcastPeers(room)

	if empty {
		roomMux.Lock()
		delete(rooms, room.ID)
		roomMux.Unlock()
	}
}

func broadcastPeers(room *Room) {
	room.Mutex.Lock()
	defer room.Mutex.Unlock()

	peerList := make([]string, 0, len(room.Clients))
	for id := range room.Clients {
		peerList = append(peerList, id)
	}

	payload := OutgoingMessage{
		Type:   "peers",
		Peers:  peerList,
		Count:  len(peerList),
		RoomID: room.ID,
	}

	data, _ := json.Marshal(payload)
	for _, c := range room.Clients {
		c.Conn.WriteMessage(websocket.TextMessage, data)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	client := &Client{
		ID:   fmt.Sprintf("peer-%p", conn),
		Conn: conn,
	}

	log.Println("New client", client.ID)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var in IncomingMessage
		if err := json.Unmarshal(raw, &in); err != nil {
			log.Println("Bad JSON:", err)
			continue
		}

		switch in.Type {
		case "join":
			if in.ClientID != "" {
				client.ID = in.ClientID
			}
			client.Mnemonic = GenerateMnemonic(client.ID)

			room := getOrCreateRoom(in.RoomID)

			room.Mutex.Lock()
			room.Clients[client.ID] = client
			room.Mutex.Unlock()

			client.RoomID = in.RoomID

			resp := OutgoingMessage{
				Type:     "joined",
				SelfID:   client.ID,
				Mnemonic: client.Mnemonic,
				RoomID:   in.RoomID,
			}
			respBytes, _ := json.Marshal(resp)
			conn.WriteMessage(websocket.TextMessage, respBytes)
			broadcastPeers(room)

		default:
			if client.RoomID == "" {
				continue
			}

			roomMux.Lock()
			room := rooms[client.RoomID]
			roomMux.Unlock()
			if room == nil {
				continue
			}

			out := OutgoingMessage{
				Type:     in.Type,
				From:     client.ID,
				Mnemonic: client.Mnemonic,
				RoomID:   client.RoomID,
				Pub:      in.Pub,
				Name:     in.Name,
				IvB64:    in.IvB64,
				DataB64:  in.DataB64,
			}

			data, _ := json.Marshal(out)

			room.Mutex.Lock()
			for _, other := range room.Clients {
				if other.ID != client.ID {
					other.Conn.WriteMessage(websocket.TextMessage, data)
				}
			}
			room.Mutex.Unlock()
		}
	}

	removeClientFromRoom(client)
	log.Println("Closed connection", client.ID)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// Start starts the relay server on the specified port
func Start(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/", healthHandler)

	handler := cors.AllowAll().Handler(mux)
	addr := fmt.Sprintf(":%d", port)
	log.Printf("🔌 SecureDrop relay on ws://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
