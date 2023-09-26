package internal

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var wsChan = make(chan WsPayload)
var clients sync.Map

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WebSocketConnection represents a WebSocket connection.
type WebSocketConnection struct {
	*websocket.Conn
	isClient bool
}

// Action defines the action types for WebSocket communication.
type Action string

const (
	ActionJoin  Action = "join"
	ActionLeave Action = "leave"
	ActionError Action = "error"
)

// WsJsonResponse defines the response sent back from WebSocket.
type WsJsonResponse struct {
	Sender      string `json:"sender"`
	Action      Action `json:"action"`
	Message     string `json:"message"`
	MessageType string `json:"message_type"`
}

// WsPayload represents the payload sent over WebSocket.
type WsPayload struct {
	Action  string              `json:"action"`
	Message string              `json:"message"`
	Conn    WebSocketConnection `json:"-"`
}

// WsEndpoint upgrades connection to WebSocket.
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	sender := r.URL.Query().Get("type")
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}

	log.Println("Client connected to endpoint")

	var response WsJsonResponse
	response.Message = "Connected to server"

	conn := WebSocketConnection{Conn: ws, isClient: sender == "CLIENT"}
	clients.Store(&conn, conn.RemoteAddr().Network())

	if err := ws.WriteJSON(response); err != nil {
		log.Println("Error writing JSON response:", err)
	}

	go ListenForWs(&conn, sender)
}

// ListenForWs listens for WebSocket messages.
func ListenForWs(conn *WebSocketConnection, sender string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error:", r)
		}
	}()

	var payload WsPayload

	for {
		err := conn.ReadJSON(&payload)
		if err != nil {
			log.Println("Error reading JSON:", err)
		} else {
			payload.Conn = *conn
			wsChan <- payload
		}
	}
}

// ListenToWsChannel listens to the WebSocket channel and handles incoming events.
func ListenToWsChannel() {
	for event := range wsChan {
		var response WsJsonResponse
		if event.Conn.isClient {
			response.Sender = "CLIENT"
		} else {
			response.Sender = "SERVER"
		}

		response.Action = Action(event.Action)
		response.Message = event.Message

		switch response.Sender {
		case "CLIENT":
			SendJSONToServer(&response)
		case "SERVER":
			BroadcastJSONToClients(&response)
		}
	}
}

// BroadcastJSONToClients sends a JSON response to all clients in the clients map.
func BroadcastJSONToClients(response *WsJsonResponse) {
	clients.Range(func(key, value interface{}) bool {
		conn, ok := key.(WebSocketConnection)
		if !ok {
			return true
		}

		if conn.isClient {
			if err := conn.WriteJSON(*response); err != nil {
				log.Println("WebSocket error:", err)
				conn.Close()
				clients.Delete(key)
			}
		}
		return true
	})
}

// SendJSONToServer sends a JSON response to all non-client connections in the clients map.
func SendJSONToServer(response *WsJsonResponse) {
	clients.Range(func(key, value interface{}) bool {
		conn, ok := key.(WebSocketConnection)
		if !ok || conn.isClient {
			return true
		}

		if err := conn.WriteJSON(*response); err != nil {
			log.Println("WebSocket error:", err)
			conn.Close()
			clients.Delete(key)
		}
		return true
	})
}