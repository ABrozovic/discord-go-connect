package internal

import (
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

	go handleWebSocketMessages(ws.Conn, b)

}

func handleWebSocketMessages(conn *websocket.Conn, b *Bot) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message from WebSocket:", err)
			return
		}
		fmt.Println(string(message))
		var wsResponse WsJsonResponse
		err = json.Unmarshal(message, &wsResponse)
		if err != nil {
			log.Println("Error decoding JSON message:", err)
			continue
		}

		switch wsResponse.Action {
		case ActionJoin:
			// Send Bot.dms with Action "UPSERT_DMS"
			dmsJSON, err := json.Marshal(map[string]interface{}{
				"action": "UPSERT_DMS",
				"dms":    b.dms,
			})
			if err != nil {
				log.Println("Error marshaling JSON for UPSERT_DMS:", err)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, dmsJSON)
			if err != nil {
				log.Println("Error sending UPSERT_DMS:", err)
				continue
			}

			// Send Bot.guilds with Action "UPSERT_GUILDS"
			guildsJSON, err := json.Marshal(map[string]interface{}{
				"action": "UPSERT_GUILDS",
				"guilds": b.guilds,
			})
			if err != nil {
				log.Println("Error marshaling JSON for UPSERT_GUILDS:", err)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, guildsJSON)
			if err != nil {
				log.Println("Error sending UPSERT_GUILDS:", err)
				continue
			}
		}

	}
}
