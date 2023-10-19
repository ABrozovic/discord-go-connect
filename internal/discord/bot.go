package discord

import (
	"discord-go-connect/internal/db"
	"discord-go-connect/internal/logger"
	"discord-go-connect/internal/wshub"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
)

type Bot struct {
	db             *db.DBManager
	session        *discordgo.Session
	conn           *websocket.Conn
	logger         *logger.StandardLoggerHandler
	writer         *messageWriter
	writeInterval  time.Duration
	guilds         map[string]*discordgo.Guild
	dms            map[string]*discordgo.Channel
	subscribers    map[string]string
	onClose        chan struct{}
	token          string
	maxBufferCount int
}

func NewBot(token string, db *db.DBManager) *Bot {
	b := &Bot{
		db:             db,
		token:          token,
		guilds:         make(map[string]*discordgo.Guild),
		dms:            make(map[string]*discordgo.Channel),
		subscribers:    make(map[string]string),
		logger:         logger.NewLogger(os.Stderr),
		onClose:        make(chan struct{}),
		writeInterval:  300 * time.Second,
		maxBufferCount: 100,
	}
	writer := newMessageWriter(b)
	b.writer = writer
	writer.start()
	return b
}

func (b *Bot) Start() error {
	session, err := discordgo.New("Bot " + b.token)
	if err != nil {
		return err
	}

	session.AddHandler(b.onReady)
	session.AddHandler(b.onMessage)

	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsGuildVoiceStates

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
			b.logger.Error("failed to close Discord session: %v", err)
			return err
		}
	}

	return nil
}

func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	b.logger.Debug("Bot is ready!")

	if len(event.Guilds) > 1 {
		for _, guild := range event.Guilds {
			guildData, _ := s.Guild(guild.ID)
			channels, _ := s.GuildChannels(guild.ID)

			b.guilds[guild.ID] = guildData
			b.guilds[guild.ID].Channels = channels
		}
		go func() {
			if err := b.CreateOrUpdateGuildsAndChannels(); err != nil {
				b.logger.Error("%v", err)
			}
		}()

	}

	if len(event.PrivateChannels) > 1 {
		for _, channel := range event.PrivateChannels {
			b.dms[channel.ID] = channel
		}
	}

	go b.subscribeToWebSocket()
}

func (b *Bot) onMessage(_ *discordgo.Session, msg *discordgo.MessageCreate) {
	if len(msg.Content) == 0 {
		return
	}

	b.writer.AddMessage(msg)

	for receiver, guildID := range b.subscribers {
		b.logger.Debug("listening to guild %s, and with user id %s", msg.GuildID, receiver)
		if guildID == msg.GuildID {
			b.sendJSONReponse(msg, &wshub.WSPayload{Action: "messages", MessageID: msg.ChannelID, Receiver: receiver})
		}
	}
}

func (b *Bot) sendMessageToChannel(channelID, message string) {
	channel, err := b.session.Channel(channelID)

	if err != nil {
		b.logger.Error("sendMessageToChannel channel not found. Error: %v", err)
	}

	_, err = b.session.ChannelMessageSend(channel.ID, message)

	if err != nil {
		b.logger.Error("sendMessageToChannel error: %v", err)
	}
}

func (b *Bot) sendDM(userId, message string) {
	channel, err := b.session.UserChannelCreate(userId)

	if err != nil {
		b.logger.Error("sendDM couldn't create channel. Error: %v", err)
	}

	_, err = b.session.ChannelMessageSend(channel.ID, message)

	if err != nil {
		b.logger.Error("sendDM error: %v", err)
	}
}

func (b *Bot) subscribeToWebSocket() {
	for {
		conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1/ws?type=D-BOT", nil)
		if err != nil {
			b.logger.Info("WebSocket connection error: %v", err)

			reconnectInterval := time.Second * 5
			time.Sleep(reconnectInterval)

			continue
		}

		b.conn = conn

		go b.handleWebSocketMessages()
		go b.heartbeat()

		<-b.onClose

		reconnectInterval := time.Second * 5
		time.Sleep(reconnectInterval)
	}
}

func (b *Bot) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		b.conn.Close()
	}()

	for range ticker.C {
		if err := b.conn.WriteJSON(&wshub.WSPayload{Action: wshub.Action[wshub.ServerAction](wshub.ClientHearbeat)}); err != nil {
			return
		}
	}
}

func (b *Bot) handleWebSocketMessages() {
	for {
		var wsPayload wshub.WSPayload

		_, message, err := b.conn.ReadMessage()

		if err != nil {
			b.logger.Error("error reading message from WebSocket: %v", err)

			b.conn.Close()
			b.onClose <- struct{}{}

			return
		}

		if err = json.Unmarshal(message, &wsPayload); err != nil {
			b.logger.Error("error decoding JSON message: %v", err)
			continue
		}

		b.logger.Info("bot received %v", wsPayload)

		action := wshub.Action[wshub.ClientAction](wsPayload.Action)

		switch action {
		case wshub.ClientJoin:
			// b.sendJSONReponse(b.dms, &wshub.WSPayload{Action: wshub.ServerListDms})
			b.sendJSONReponse(b.guilds, &wshub.WSPayload{Action: wshub.ServerListGuilds})
		case wshub.ClientGuildMessage:
			b.sendMessageToChannel(wsPayload.MessageID, wsPayload.Message)
		case wshub.ClientSubscribeToGuild:
			b.logger.Debug("%d, message %v", len(b.writer.WriteBuffer), wsPayload)
			msgs := make([]*discordgo.MessageCreate, 0)
			for _, msg := range b.writer.WriteBuffer {
				if msg.GuildID == wsPayload.Message {
					msgs = append(msgs, msg)
				}
			}
			b.sendJSONReponse(msgs, &wshub.WSPayload{Action: "messages", Receiver: wsPayload.Receiver})
			b.subscribers[wsPayload.Receiver] = wsPayload.Message
		case wshub.ClientLeave:
			delete(b.subscribers, wsPayload.Message)
		case wshub.ClientDmMessage:
			b.sendDM(wsPayload.MessageID, wsPayload.Message)
		}
	}
}

func (b *Bot) sendJSONReponse(toMarshal interface{}, wsReponse *wshub.WSPayload) {
	log.Println("Payload", wsReponse.Action)
	message, err := json.Marshal(toMarshal)
	if err != nil {
		b.logger.Error("eror marshaling for %s. Error: %v:", wsReponse.Action, err)
		return
	}

	err = b.conn.WriteJSON(wshub.WSPayload{
		Action:    wsReponse.Action,
		Message:   string(message),
		MessageID: wsReponse.MessageID,
		Receiver:  wsReponse.Receiver,
	})

	if err != nil {
		b.logger.Error("error sending %s. Error: %v", wsReponse.Action, err)
	}
}
