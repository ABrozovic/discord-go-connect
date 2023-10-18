package wshub

// Action defines the action types for WebSocket communication.

type ClientAction string
type ServerAction string

type Action[T ServerAction | ClientAction] string

const (
	ClientHearbeat         Action[ClientAction] = "heartbeat"
	ClientJoin             Action[ClientAction] = "get_guilds"
	ClientLeave            Action[ClientAction] = "leave"
	ClientGuildMessage     Action[ClientAction] = "guild_message"
	ClientSubscribeToGuild Action[ClientAction] = "get_messages"
	ClientDmMessage        Action[ClientAction] = "dm_message"
	ServerHandshake        Action[ServerAction] = "handshake"
	ServerListGuilds       Action[ServerAction] = "guilds"
	ServerListDms          Action[ServerAction] = "list_dms"
)
