package client

import (
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/schollz/share/src/relay"
)

// ProtobufMessage is an alias for relay.OutgoingMessage
type ProtobufMessage = relay.OutgoingMessage

// ConnectionType indicates whether a connection is local or internet-based
type ConnectionType string

const (
	ConnectionTypeLocal    ConnectionType = "local"
	ConnectionTypeInternet ConnectionType = "internet"
)

// RelayConnection represents a single connection to a relay server
type RelayConnection struct {
	Conn           *websocket.Conn
	Type           ConnectionType
	URL            string
	IsActive       bool
	mutex          sync.Mutex
	messageHandler func(*ProtobufMessage) error
}

// ConnectionManager manages multiple relay connections and prefers local connections
type ConnectionManager struct {
	Connections      []*RelayConnection
	PreferredConn    *RelayConnection
	mutex            sync.RWMutex
	messageChannel   chan *ProtobufMessage
	closeChannel     chan bool
	onMessageHandler func(*ProtobufMessage)
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		Connections:    make([]*RelayConnection, 0),
		messageChannel: make(chan *ProtobufMessage, 100),
		closeChannel:   make(chan bool),
	}
}

// AddConnection adds a new relay connection to the manager
func (cm *ConnectionManager) AddConnection(conn *websocket.Conn, connType ConnectionType, url string) *RelayConnection {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	rc := &RelayConnection{
		Conn:     conn,
		Type:     connType,
		URL:      url,
		IsActive: true,
	}

	cm.Connections = append(cm.Connections, rc)

	// If this is a local connection, prefer it immediately
	if connType == ConnectionTypeLocal {
		cm.PreferredConn = rc
		log.Printf("Using local relay connection: %s", url)
	} else if cm.PreferredConn == nil {
		cm.PreferredConn = rc
		log.Printf("Using internet relay connection: %s", url)
	}

	// Start listening to this connection
	go cm.listenToConnection(rc)

	return rc
}

// listenToConnection listens for messages from a specific connection
func (cm *ConnectionManager) listenToConnection(rc *RelayConnection) {
	defer func() {
		rc.mutex.Lock()
		rc.IsActive = false
		rc.mutex.Unlock()
		rc.Conn.Close()
	}()

	for {
		msg, err := receiveProtobufMessage(rc.Conn)
		if err != nil {
			// Connection closed or error
			return
		}

		// Forward message to the main channel
		select {
		case cm.messageChannel <- msg:
		case <-cm.closeChannel:
			return
		}
	}
}

// SendMessage sends a message through the preferred connection
func (cm *ConnectionManager) SendMessage(msg map[string]interface{}) error {
	cm.mutex.RLock()
	preferred := cm.PreferredConn
	cm.mutex.RUnlock()

	if preferred == nil {
		return fmt.Errorf("no active connection available")
	}

	preferred.mutex.Lock()
	defer preferred.mutex.Unlock()

	if !preferred.IsActive {
		return fmt.Errorf("preferred connection is not active")
	}

	return sendProtobufMessage(preferred.Conn, msg)
}

// BroadcastMessage sends a message through all active connections
func (cm *ConnectionManager) BroadcastMessage(msg map[string]interface{}) {
	cm.mutex.RLock()
	connections := make([]*RelayConnection, len(cm.Connections))
	copy(connections, cm.Connections)
	cm.mutex.RUnlock()

	for _, rc := range connections {
		rc.mutex.Lock()
		if rc.IsActive {
			sendProtobufMessage(rc.Conn, msg)
		}
		rc.mutex.Unlock()
	}
}

// ReceiveMessage waits for and returns the next message from any connection
func (cm *ConnectionManager) ReceiveMessage() (*ProtobufMessage, error) {
	select {
	case msg := <-cm.messageChannel:
		return msg, nil
	case <-cm.closeChannel:
		return nil, fmt.Errorf("connection manager closed")
	}
}

// GetPreferredConnectionType returns the type of the current preferred connection
func (cm *ConnectionManager) GetPreferredConnectionType() ConnectionType {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if cm.PreferredConn != nil {
		return cm.PreferredConn.Type
	}
	return ""
}

// Close closes all connections and stops the connection manager
func (cm *ConnectionManager) Close() {
	close(cm.closeChannel)

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	for _, rc := range cm.Connections {
		rc.mutex.Lock()
		rc.IsActive = false
		rc.Conn.Close()
		rc.mutex.Unlock()
	}
}

// ConnectWithTimeout attempts to connect to a relay with a timeout
func ConnectWithTimeout(serverURL string, timeout time.Duration) (*websocket.Conn, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/ws"

	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
	}

	conn, _, err := dialer.Dial(u.String(), nil)
	return conn, err
}
