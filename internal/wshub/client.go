package wshub

import (
	"discord-go-connect/internal/logger"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	hub        *Hub
	Conn       *websocket.Conn
	logger     *logger.StandardLoggerHandler
	ID         string
	ClientType string
}

const (
	pongWait        = 60 * time.Second
	readBufferSize  = 1024
	writeBufferSize = 1024
)

var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: writeBufferSize,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// WSJSONResponse defines the response sent back from WebSocket.
type WSJSONResponse struct {
	Action    Action[ClientAction] `json:"action"`
	MessageID string               `json:"message_id"`
	Message   string               `json:"message"`
}

// WSPayload represents the payload sent over WebSocket.
type WSPayload struct {
	Action    Action[ServerAction] `json:"action"`
	MessageID string               `json:"message_id"`
	Message   string               `json:"message"`
	Receiver  string               `json:"receiver"`
	Client    Client               `json:"-"`
}

// WSHandler is the HTTP handler for WebSocket connections.
func WSHandler(h *Hub, w http.ResponseWriter, r *http.Request) {
	clientType := r.URL.Query().Get("type")

	ws, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("%v", err)
		return
	}

	client := Client{Conn: ws, hub: h, ID: uuid.NewString(), ClientType: clientType, logger: h.logger}
	h.register <- &client

	go client.ReadWS()
}

// ReadWS listens for WebSocket messages.
func (c *Client) ReadWS() {
	var err error
	defer func() {
		if err != nil {
			c.logger.Debug("Error %v", err)
		}

		c.hub.server <- &WSPayload{Action: Action[ServerAction](ClientLeave), Message: c.ID}
		c.hub.unregister <- c
	}()

	if err = c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		c.logger.Error("%v", err)
	}

	c.Conn.SetPongHandler(func(string) error {
		if err = c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			c.logger.Error("%v", err)
		}
		return nil
	})

	var payload WSPayload

	for {
		err = c.Conn.ReadJSON(&payload)

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Debug("error: %v", err)
			}

			break
		}

		if payload.Action == Action[ServerAction](ClientHearbeat) {
			if err = c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

			continue
		}

		if _, ok := c.hub.clients[c]; ok && c.hub.discordBot != nil {
			payload.Receiver = c.ID
			c.hub.server <- &payload
		} else {
			if len(payload.Receiver) > 0 {
				c.hub.unicast <- &payload
			} else {
				c.hub.broadcast <- &payload
			}
		}
	}
}

func (c *Client) SendMessage(wsJSONMessage *WSJSONResponse) {
	if err := c.Conn.WriteJSON(wsJSONMessage); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			c.logger.Debug("error: %v", err)
		}
		c.hub.unregister <- c
	}
}
