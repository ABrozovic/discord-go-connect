package discord

import (
	ws "discord-go-connect/internal/websocket"
	"encoding/json"
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"github.com/recws-org/recws"
)

type Bot struct {
	session *discordgo.Session
	token   string
	guilds  map[string]*discordgo.Guild
	dms     map[string]*discordgo.Channel
	conn    *websocket.Conn
}

func NewBot(token string) *Bot {
	return &Bot{
		token:  token,
		guilds: make(map[string]*discordgo.Guild),
		dms:    make(map[string]*discordgo.Channel),
	}
}

func (b *Bot) Start() error {
	session, err := discordgo.New("Bot " + b.token)
	if err != nil {
		return err
	}

	session.AddHandler(b.onReady)

	err = session.Open()
	if err != nil {
		return err
	}

	b.session = session
	return nil
}

func (b *Bot) Stop() error {
	if b.session != nil {
		err := b.session.Close()
		if err != nil {
			log.Println("Failed to close Discord session:", err)
			return err
		}
	}

	return nil
}

func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Println("Bot is ready!")
	if len(event.Guilds) > 1 {
		for _, guild := range event.Guilds {
			guildData, _ := s.Guild(guild.ID)
			b.guilds[guild.ID] = guildData
		}
	}
	if len(event.PrivateChannels) > 1 {
		for _, channel := range event.PrivateChannels {
			b.dms[channel.ID] = channel
		}
	}

	go subscribeToWebSocket(b)
}

func subscribeToWebSocket(b *Bot) {

	ws := recws.RecConn{
		KeepAliveTimeout: 0,
	}

	ws.Dial("ws://127.0.0.1/ws?type=SERVER", nil)

	b.conn = ws.Conn

	go handleWebSocketMessages(b)

}

func handleWebSocketMessages(b *Bot) {
	for {
		_, message, err := b.conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message from WebSocket:", err)
			return
		}
		fmt.Println(string(message))
		var wsResponse ws.WsJsonResponse
		err = json.Unmarshal(message, &wsResponse)
		if err != nil {
			log.Println("Error decoding JSON message:", err)
			continue
		}

		switch wsResponse.Action {
		case ws.ActionClientJoin:
			b.sendJsonReponse(b.dms, ws.ActionServerListDms)
			b.sendJsonReponse(b.guilds, ws.ActionServerListGuilds)
		}

	}
}

func (b *Bot) sendJsonReponse(toMarshal interface{}, action ws.Action) {
	dmsJSON, err := json.Marshal(toMarshal)
	if err != nil {
		log.Printf("Error marshaling for %s. Error: %v:", action, err)
		return
	}

	response, err := json.Marshal(ws.WsJsonResponse{
		Action:    action,
		Message:   string(dmsJSON),
		MessageID: "0",
	})
	if err != nil {
		log.Printf("Error marshaling %s. Error: %v", action, err)

	}
	err = b.conn.WriteMessage(websocket.TextMessage, response)
	if err != nil {
		log.Printf("Error sending %s. Error: %v", action, err)
	}
}
