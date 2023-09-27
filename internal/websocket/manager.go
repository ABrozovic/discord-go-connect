package ws

import (
	"fmt"
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

// WsJsonResponse defines the response sent back from websocket
type WsJsonResponse struct {
	Sender    string `json:"sender"`
	Action    Action `json:"action"`
	MessageID string `json:"message_id"`
	Message   string `json:"message"`
}

// WsPayload represents the payload sent over WebSocket.
type WsPayload struct {
	Action    string              `json:"action"`
	MessageID string              `json:"message_id"`
	Message   string              `json:"message"`
	Conn      WebSocketConnection `json:"-"`
}

// WsEndpoint Upgrades connection to websocket.
func WsEndpoint(w http.ResponseWriter, r *http.Request) {
	sender := r.URL.Query().Get("type")
	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Client connected to endpoint")

	var response WsJsonResponse
	response.Message = "Connected to server"

	conn := WebSocketConnection{Conn: ws, isClient: sender == "CLIENT"}
	clients.Store(&conn, conn.RemoteAddr().Network())

	err = ws.WriteJSON(response)
	if err != nil {
		log.Println(err)
	}

	go ListenForWs(&conn, sender)
}

// ListenForWs listens for WebSocket messages.
func ListenForWs(conn *WebSocketConnection, sender string) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()

	var payload WsPayload

	for {
		err := conn.ReadJSON(&payload)
		if err != nil {

		} else {
			payload.Conn = *conn
			wsChan <- payload
		}
	}
}

// ListenToWsChannel listens to the WebSocket channel and handles incoming events.
func ListenToWsChannel() {
	var response WsJsonResponse
	for {
		event := <-wsChan
		if event.Conn.isClient {
			response.Sender = "CLIENT"
		} else {
			response.Sender = "SERVER"
		}

		response.Action = Action(event.Action)
		response.Message = event.Message
		response.MessageID = event.MessageID

		switch response.Sender {
		case "CLIENT":
			SendJSONToServer(&response)
		case "SERVER":
			BroadcastJsonToClients(&response)
		}
	}
}

// BroadcastJsonToClients sends a JSON response to all clients in the clients map.
func BroadcastJsonToClients(response *WsJsonResponse) {
	clients.Range(func(key, value interface{}) bool {
		conn := key.(*WebSocketConnection)

		if conn.isClient {
			err := conn.WriteJSON(*response)
			if err != nil {
				log.Println("websocket err:", err)
				conn.Close()
				clients.Delete(conn)
			}
		}
		return true
	})
}

// SendJSONToServer sends a JSON response to all non-client connections in the clients map.
func SendJSONToServer(response *WsJsonResponse) {
	clients.Range(func(key, value interface{}) bool {
		conn := key.(*WebSocketConnection)
		if !conn.isClient {
			err := conn.WriteJSON(*response)
			if err != nil {
				log.Println("websocket err:", err)
				conn.Close()
				clients.Delete(conn)
			}
		}
		return true
	})
}
