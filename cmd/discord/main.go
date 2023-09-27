package main

import (
	"discord-go-connect/internal"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	go func() {
		http.HandleFunc("/health", healthHandler)
		http.HandleFunc("/ws", internal.WsEndpoint)
		log.Println("Starting WebSocket server on localhost:8080")
		err := http.ListenAndServe(":80", nil)
		if err != nil {
			log.Fatal("Failed to start WebSocket server:", err)
		}
	}()

	log.Println("Starting channel listener")
	go internal.ListenToWsChannel()

	err := godotenv.Load()
	if err != nil {
		log.Fatalln("DISCORD_BOT_TOKEN missing")
	}
	botToken := os.Getenv("DISCORD_BOT_TOKEN")

	bot := internal.NewBot(botToken)

	err = bot.Start()
	if err != nil {
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
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
