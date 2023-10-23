package main

import (
	"database/sql"
	"discord-go-connect/internal/db"
	"discord-go-connect/internal/discord"
	"discord-go-connect/internal/wshub"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"encoding/json"
	"log"
	"net/http"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	hub := wshub.NewHub()
	dbManager, err := db.NewDBManager()

	if err != nil {
		log.Fatal("Error connecting to the database:", err)
		return
	}

	defer dbManager.Close()

	go func() {
		http.HandleFunc("/health", healthHandler)
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			wshub.WSHandler(hub, w, r)
		})

		http.HandleFunc("/api/channel", func(w http.ResponseWriter, r *http.Request) {
			selectMessages := `
				SELECT 
					Message.id, 
					Message.channel_id, 
					Message.guild_id,
					Message.author_id,
					Message.pinned,
					Message.type AS message_type,
					Message.content,
					Message.timestamp AS message_timestamp,
					Message.edited_timestamp,
					Author.username,
					Author.avatar,
					Author.bot,
					Member.nick,
					Member.avatar 
				FROM Message
				JOIN Author ON Message.author_id = Author.id
				JOIN Member ON Message.member_id = Member.id
				WHERE Message.channel_id = ?
				ORDER BY Message.timestamp DESC
				LIMIT ? OFFSET ?;
			`
			w.Header().Set("Content-Type", "application/json")
			if r.Method == "GET" {
				channelID := r.URL.Query().Get("channelId")

				page := r.URL.Query().Get("page")
				pageSize := 20
				pageNum, err := strconv.Atoi(page)

				if err != nil {
					http.Error(w, "Invalid page number", http.StatusBadRequest)
					return
				}

				offset := (pageNum - 1) * pageSize
				rows, err := dbManager.Query(selectMessages, channelID, pageSize+1, offset)
				if err != nil {
					log.Println("Failed to fetch messages:", err)
					http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)
					return
				}
				defer rows.Close()

				var messages []discordgo.Message
				for rows.Next() {
					var message discordgo.Message
					message.Author = &discordgo.User{}
					message.Member = &discordgo.Member{}
					var timestamp string
					var editedTimestamp sql.NullTime
					err = rows.Scan(
						&message.ID,
						&message.ChannelID,
						&message.GuildID,
						&message.Author.ID,
						&message.Pinned,
						&message.Type,
						&message.Content,
						&timestamp,
						&editedTimestamp,
						&message.Author.Username,
						&message.Author.Avatar,
						&message.Author.Bot,
						&message.Member.Nick,
						&message.Member.Avatar,
					)
					if err != nil {
						log.Println("Error scanning channel row:", err)
						http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
						return
					}
					layout := "2006-01-02 15:04:05.000"
					message.Timestamp, _ = time.Parse(layout, timestamp)
					if editedTimestamp.Valid {
						message.EditedTimestamp = &editedTimestamp.Time
					} else {
						message.EditedTimestamp = nil
					}
					messages = append(messages, message)
				}
				if err = rows.Err(); err != nil {
					log.Println("Error iterating over channels rows:", err)
					http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
					return
				}

				var cursorChan int
				if len(messages) == pageSize+1 {
					messages = messages[:len(messages)-1]
					cursorChan = pageNum + 1
				} else {
					cursorChan = 0
				}

				type dunno struct {
					Data       []discordgo.Message `json:"data"`
					NextCursor int                 `json:"nextCursor"`
				}
				data := dunno{
					Data:       messages,
					NextCursor: cursorChan,
				}

				jsonBytes, err := json.Marshal(data)
				if err != nil {
					log.Println("Error marshaling channels to JSON:", err)
					http.Error(w, "Failed to fetch channels", http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(jsonBytes); err != nil {
					log.Println("api endpoint error:", err)
				}
			}
		})

		log.Println("Starting WebSocket server on localhost:8080")

		corsHandler := cors.Default().Handler(http.DefaultServeMux)
		server := &http.Server{
			Addr:              ":80",
			Handler:           corsHandler,
			ReadHeaderTimeout: 3 * time.Second,
		}

		err = server.ListenAndServe()
		if err != nil {
			log.Fatal("Failed to start WebSocket server:", err)
		}
	}()

	log.Println("Starting channel listener")

	go hub.ListenToWSChannel()

	err = godotenv.Load()
	if err != nil {
		log.Println("DISCORD_BOT_TOKEN missing")
		return
	}

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	bot := discord.NewBot(botToken, dbManager)

	if err = bot.Start(); err != nil {
		log.Println("Failed to start the bot:", err)
		return
	}

	log.Println("Bot is now running. Press Ctrl+C to stop.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	err = bot.Stop()
	if err != nil {
		log.Println("Failed to stop the bot:", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	healthResponse := struct {
		Status string `json:"status"`
	}{
		Status: "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(healthResponse)

	if err != nil {
		log.Printf("Health check encoding error: %v", err)
	}
}
