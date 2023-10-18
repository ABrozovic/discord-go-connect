package wshub

import (
	"discord-go-connect/internal/logger"
	"encoding/json"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients      map[*Client]struct{}
	discordBot   *Client
	broadcast    chan WSPayload
	unicast      chan WSPayload
	server       chan WSPayload
	register     chan *Client
	unregister   chan *Client
	logger       *logger.StandardLoggerHandler
	clientLookup sync.Map
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan WSPayload),
		unicast:    make(chan WSPayload),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
		server:     make(chan WSPayload, 10),
		logger:     logger.NewLogger(os.Stderr),
	}
}

func (h *Hub) ListenToWSChannel() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
			h.sendHandshake(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case payload := <-h.broadcast:
			h.broadcastMessage(payload)
		case payload := <-h.unicast:
			h.unicastMessage(payload)
		case payload := <-h.server:
			h.sendPayloadToBot(payload)
		}
	}
}

func (h *Hub) registerClient(c *Client) {
	if c.ClientType == "D-BOT" {
		h.logger.Debug("Registering bot with id: %s", c.ID)
		h.discordBot = c
	} else {
		h.clients[c] = struct{}{}
		h.clientLookup.Store(c.ID, *c)
		h.logger.Debug("Registering client with id: %s", c.ID)
	}
}

func (h *Hub) sendHandshake(client *Client) {
	status := struct {
		Status int `json:"status"`
	}{Status: 200}

	statusMSG, err := json.Marshal(status)
	if err != nil {
		h.logger.Error("error marshaling status message. Error: %v", err)
		return
	}

	wsJSONResponse := WSJSONResponse{
		MessageID: "0",
		Action:    Action[ClientAction](ServerHandshake),
		Message:   string(statusMSG),
	}
	client.SendMessage(&wsJSONResponse)
}

func (h *Hub) unregisterClient(client *Client) {
	if _, ok := h.clients[client]; ok {
		h.logger.Debug("Unregistering %s with id: %s", client.ClientType, client.ID)
		h.clientLookup.Delete(client.ID)
		delete(h.clients, client)
		client.Conn.Close()

	} else {
		if h.discordBot != nil {
			client.Conn.Close()
			h.logger.Debug("Unregistering %s with id: %s", client.ClientType, client.ID)
		}
	}
}

func (h *Hub) broadcastMessage(payload WSPayload) {
	for client := range h.clients {
		message := WSJSONResponse{Action: Action[ClientAction](payload.Action), MessageID: payload.MessageID, Message: payload.Message}
		client.SendMessage(&message)
	}
}

func (h *Hub) unicastMessage(payload WSPayload) {
	if client, ok := h.clientLookup.Load(payload.Receiver); ok {
		message := WSJSONResponse{Action: Action[ClientAction](payload.Action), MessageID: payload.MessageID, Message: payload.Message}

		if c, ok := client.(Client); ok {
			c.SendMessage(&message)
		}
	}
}

func (h *Hub) sendPayloadToBot(payload WSPayload) {
	if h.discordBot == nil {
		return
	}

	if err := h.discordBot.Conn.WriteJSON(payload); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			h.logger.Debug("error: %v", err)
		}
		h.unregister <- h.discordBot
	}
}
