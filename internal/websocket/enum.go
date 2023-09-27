package ws

// Action defines the action types for WebSocket communication.
type Action string

const (
	ActionClientJoin Action = "join"

	ActionServerListGuilds Action = "list_guilds"
	ActionServerListDms    Action = "list_dms"
)
