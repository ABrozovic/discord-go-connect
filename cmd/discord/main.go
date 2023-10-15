package main

import (
	"discord-go-connect/internal/db"
	"discord-go-connect/internal/discord"
	"discord-go-connect/internal/wshub"
	"os"
	"os/signal"
	"syscall"
	"time"

	"encoding/json"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	hub := wshub.NewHub()
	db, err := db.NewDBManager()

	if err != nil {
		log.Fatal("Error connecting to the database:", err)
		return
	}

	defer db.Close()

	go func() {
		http.HandleFunc("/health", healthHandler)
		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			wshub.WSHandler(hub, w, r)
		})

		log.Println("Starting WebSocket server on localhost:8080")

		server := &http.Server{
			Addr:              ":80",
			ReadHeaderTimeout: 3 * time.Second,
		}

		err := server.ListenAndServe()
		if err != nil {
			log.Fatal("Failed to start WebSocket server:", err)
		}
	}()

	log.Println("Starting channel listener")

	go hub.ListenToWSChannel()

	err = godotenv.Load()
	if err != nil {
		log.Fatalln("DISCORD_BOT_TOKEN missing")
	}

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	bot := discord.NewBot(botToken, db)

	if err = bot.Start(); err != nil {
		log.Fatal("Failed to start the bot:", err)
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
