package relay

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"github.com/tyler-smith/go-bip39"
)

type Client struct {
	ID          string
	Mnemonic    string
	Conn        *websocket.Conn
	RoomID      string
	IP          string
	UseProtobuf bool // Track if client uses protobuf
}

type Room struct {
	ID      string
	Clients map[string]*Client
	Mutex   sync.Mutex
}

type IncomingMessage struct {
	Type              string `json:"type"`
	RoomID            string `json:"roomId,omitempty"`
	ClientID          string `json:"clientId,omitempty"`
	Pub               string `json:"pub,omitempty"`
	IvB64             string `json:"iv_b64,omitempty"`
	DataB64           string `json:"data_b64,omitempty"`
	ChunkData         string `json:"chunk_data,omitempty"`
	ChunkNum          int    `json:"chunk_num,omitempty"`
	EncryptedMetadata string `json:"encrypted_metadata,omitempty"` // Zero-knowledge metadata
	MetadataIV        string `json:"metadata_iv,omitempty"`        // IV for encrypted metadata
}

type OutgoingMessage struct {
	Type              string   `json:"type"`
	From              string   `json:"from,omitempty"`
	Mnemonic          string   `json:"mnemonic,omitempty"`
	RoomID            string   `json:"roomId,omitempty"`
	Pub               string   `json:"pub,omitempty"`
	IvB64             string   `json:"iv_b64,omitempty"`
	DataB64           string   `json:"data_b64,omitempty"`
	ChunkData         string   `json:"chunk_data,omitempty"`
	ChunkNum          int      `json:"chunk_num,omitempty"`
	SelfID            string   `json:"selfId,omitempty"`
	Peers             []string `json:"peers,omitempty"`
	Count             int      `json:"count,omitempty"`
	Error             string   `json:"error,omitempty"`
	EncryptedMetadata string   `json:"encrypted_metadata,omitempty"` // Zero-knowledge metadata
	MetadataIV        string   `json:"metadata_iv,omitempty"`        // IV for encrypted metadata
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	rooms         = make(map[string]*Room)
	roomMux       sync.Mutex
	maxRooms      int
	maxRoomsPerIP int
	// Track room membership counts per IP so we can enforce per-IP limits
	ipRooms   = make(map[string]map[string]int) // IP -> roomID -> connection count
	ipRoomMux sync.Mutex
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

func reserveIPRoom(ip, roomID string) bool {
	if maxRoomsPerIP <= 0 {
		return true
	}

	ipRoomMux.Lock()
	defer ipRoomMux.Unlock()

	roomsForIP, ok := ipRooms[ip]
	if !ok {
		roomsForIP = make(map[string]int)
		ipRooms[ip] = roomsForIP
	}

	if roomsForIP[roomID] > 0 {
		roomsForIP[roomID]++
		return true
	}

	if len(roomsForIP) >= maxRoomsPerIP {
		return false
	}

	roomsForIP[roomID] = 1
	return true
}

func releaseIPRoom(ip, roomID string) {
	if maxRoomsPerIP <= 0 {
		return
	}

	ipRoomMux.Lock()
	defer ipRoomMux.Unlock()

	roomsForIP, ok := ipRooms[ip]
	if !ok {
		return
	}

	count := roomsForIP[roomID]
	switch {
	case count <= 1:
		delete(roomsForIP, roomID)
	default:
		roomsForIP[roomID] = count - 1
	}

	if len(roomsForIP) == 0 {
		delete(ipRooms, ip)
	}
}

func getOrCreateRoom(roomID string) *Room {
	roomMux.Lock()
	defer roomMux.Unlock()

	if rm, ok := rooms[roomID]; ok {
		return rm
	}

	// Enforce room limit
	if maxRooms > 0 && len(rooms) >= maxRooms {
		logger.Warn("Room limit reached", "limit", maxRooms, "current", len(rooms))
		return nil
	}

	rm := &Room{
		ID:      roomID,
		Clients: make(map[string]*Client),
	}
	rooms[roomID] = rm
	logger.Info("Room created", "room", roomID, "total_rooms", len(rooms))
	return rm
}

func removeClientFromRoom(c *Client) {
	roomID := c.RoomID
	if roomID == "" {
		return
	}

	roomMux.Lock()
	room, ok := rooms[roomID]
	roomMux.Unlock()
	if !ok {
		releaseIPRoom(c.IP, roomID)
		c.RoomID = ""
		return
	}

	room.Mutex.Lock()
	delete(room.Clients, c.ID)
	empty := len(room.Clients) == 0
	room.Mutex.Unlock()

	releaseIPRoom(c.IP, roomID)
	c.RoomID = ""

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

	for _, c := range room.Clients {
		sendMessage(c.Conn, &payload, c.UseProtobuf)
	}
}

var logger *slog.Logger

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Upgrade error", "error", err)
		return
	}
	defer conn.Close()

	// Extract IP address, checking for proxy headers
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take first IP in the list
		if idx := strings.Index(forwarded, ","); idx > 0 {
			clientIP = strings.TrimSpace(forwarded[:idx])
		} else {
			clientIP = strings.TrimSpace(forwarded)
		}
	} else if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}
	// Strip port if present
	if idx := strings.LastIndex(clientIP, ":"); idx > 0 {
		clientIP = clientIP[:idx]
	}

	client := &Client{
		ID:   fmt.Sprintf("peer-%p", conn),
		Conn: conn,
		IP:   clientIP,
	}

	logger.Info("New client", "clientId", client.ID, "ip", clientIP)

	// All clients use protobuf
	client.UseProtobuf = true

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Decode protobuf message
		in, err := decodeProtobuf(raw)
		if err != nil {
			logger.Warn("Bad protobuf message", "error", err, "clientId", client.ID)
			continue
		}

		switch in.Type {
		case "join":
			if in.ClientID != "" {
				client.ID = in.ClientID
			}
			client.Mnemonic = GenerateMnemonic(client.ID)

			if client.RoomID == in.RoomID {
				resp := OutgoingMessage{
					Type:     "joined",
					SelfID:   client.ID,
					Mnemonic: client.Mnemonic,
					RoomID:   in.RoomID,
				}
				sendMessage(conn, &resp, client.UseProtobuf)
				continue
			}

			if client.RoomID != "" && client.RoomID != in.RoomID {
				removeClientFromRoom(client)
			}

			if !reserveIPRoom(client.IP, in.RoomID) {
				logger.Warn("Per-IP room limit reached", "ip", client.IP, "limit", maxRoomsPerIP)
				resp := OutgoingMessage{
					Type:  "error",
					Error: "Maximum rooms per IP reached, try again later",
				}
				sendMessage(conn, &resp, client.UseProtobuf)
				conn.Close()
				return
			}

			room := getOrCreateRoom(in.RoomID)

			// Check if room creation was blocked due to limit
			if room == nil {
				releaseIPRoom(client.IP, in.RoomID)
				resp := OutgoingMessage{
					Type:  "error",
					Error: "Maximum rooms reached, try again later",
				}
				sendMessage(conn, &resp, client.UseProtobuf)
				conn.Close()
				return
			}

			room.Mutex.Lock()
			room.Clients[client.ID] = client
			room.Mutex.Unlock()

			client.RoomID = in.RoomID

			logger.Info("Client joined room", "clientId", client.ID, "room", in.RoomID)

			resp := OutgoingMessage{
				Type:     "joined",
				SelfID:   client.ID,
				Mnemonic: client.Mnemonic,
				RoomID:   in.RoomID,
			}
			sendMessage(conn, &resp, client.UseProtobuf)
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

			// Debug logging for file_start messages - but not the sensitive metadata
			if in.Type == "file_start" {
				hasEncrypted := in.EncryptedMetadata != ""
				logger.Debug("Relay forwarding file_start",
					"hasEncryptedMetadata", hasEncrypted)
			}

			out := OutgoingMessage{
				Type:              in.Type,
				From:              client.ID,
				Mnemonic:          client.Mnemonic,
				RoomID:            client.RoomID,
				Pub:               in.Pub,
				IvB64:             in.IvB64,
				DataB64:           in.DataB64,
				ChunkData:         in.ChunkData,
				ChunkNum:          in.ChunkNum,
				EncryptedMetadata: in.EncryptedMetadata, // Zero-knowledge metadata
				MetadataIV:        in.MetadataIV,        // IV for encrypted metadata
			}

			room.Mutex.Lock()
			for _, other := range room.Clients {
				if other.ID != client.ID {
					sendMessage(other.Conn, &out, other.UseProtobuf)
				}
			}
			room.Mutex.Unlock()
		}
	}

	removeClientFromRoom(client)
	logger.Info("Closed connection", "clientId", client.ID)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// spaHandler wraps http.FileServer to handle SPA routing
type spaHandler struct {
	staticFS      http.FileSystem
	installScript []byte
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ua := strings.ToLower(r.UserAgent())
	if strings.Contains(ua, "curl") && len(h.installScript) > 0 {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(h.installScript)
		return
	}

	// Try to serve the requested file
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	f, err := h.staticFS.Open(path)
	if err == nil {
		f.Close()
		http.FileServer(h.staticFS).ServeHTTP(w, r)
		return
	}

	// File not found, serve index.html for client-side routing
	index, err := h.staticFS.Open("/index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer index.Close()

	stat, err := index.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, "index.html", stat.ModTime(), index.(io.ReadSeeker))
}

// Start starts the relay server on the specified port
func Start(port int, maxRoomsLimit int, maxRoomsPerIPLimit int, staticFS embed.FS, log *slog.Logger) {
	logger = log
	maxRooms = maxRoomsLimit
	maxRoomsPerIP = maxRoomsPerIPLimit

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/health", healthHandler)

	installScript, err := staticFS.ReadFile("install.sh")
	if err != nil {
		logger.Warn("Install script missing", "error", err)
	}

	if len(installScript) > 0 {
		mux.HandleFunc("/install.sh", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write(installScript)
		})
	}

	// Serve embedded static files with SPA support
	distFS, err := fs.Sub(staticFS, "web/dist")
	if err != nil {
		logger.Error("Failed to access embedded files", "error", err)
		return
	}
	spaHandlerInstance := spaHandler{staticFS: http.FS(distFS), installScript: installScript}
	mux.Handle("/", newGzipFileHandler(spaHandlerInstance))

	handler := cors.AllowAll().Handler(mux)
	addr := fmt.Sprintf(":%d", port)
	logger.Info("share relay starting", "address", fmt.Sprintf("ws://localhost%s", addr))
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error("Server failed", "error", err)
	}
}
