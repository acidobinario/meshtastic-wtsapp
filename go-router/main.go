package main

import (
	"bytes"
	// "context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"strings"
)

const (
	whatsappBotURL      = "http://whatsapp-bot:3000"
	sendMessageEndpoint = whatsappBotURL + "/send-message"
	healthCheckEndpoint = whatsappBotURL + "/"
)

// WhatsAppMessage represents incoming WhatsApp messages forwarded by the bot.
type WhatsAppMessage struct {
	From      string `json:"from"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

// MeshtastiMessage represents incoming messages from meshtastic
type MeshtasticMessage struct {
	Message string `json:"message"`
	Timestamp int64 `json:"timestamp"`
}

func main() {
	log.Println("go-router starting, waiting for whatsapp-bot to be ready...")

	// Wait until whatsapp-bot is responding
	waitForWhatsAppBot()

	// Start HTTP server to receive messages from whatsapp-bot
	http.HandleFunc("/receive-message", receiveMessageHandler)
	// HTTP handler to receive message from meshtastic-bridge and send it
	http.HandleFunc("/send-message", sendMessageHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("go-router listening on :8080 for incoming messages...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func waitForWhatsAppBot() {
	client := &http.Client{Timeout: 10 * time.Second}
	for {
		resp, err := client.Get(healthCheckEndpoint)
		if err == nil && resp.StatusCode == 200 {
			_ = resp.Body.Close()
			break
		}
		log.Println("Waiting for whatsapp-bot to be ready...")
		time.Sleep(3 * time.Second)
	}
}

func sendWhatsAppMessage(number, message string) error {
	payload := map[string]string{
		"number":  number,
		"message": message,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := http.Post(sendMessageEndpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("post send-message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("send-message returned status: %s", resp.Status)
	}
	return nil
}

func receiveMessageHandler(w http.ResponseWriter, r *http.Request) {
	var msg WhatsAppMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Received WhatsApp message from %s: %s\n", msg.From, msg.Body)
	w.WriteHeader(http.StatusOK)
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	var msg MeshtasticMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Received Meshtastic message: %s", msg.Message)

	// Command parsing
	if strings.HasPrefix(msg.Message, "!wsp") {
		parts := strings.Fields(msg.Message)
		if len(parts) < 3 {
			http.Error(w, "Invalid !wsp command. Format: !wsp <phone> <message>", http.StatusBadRequest)
			return
		}

		// Extract phone number and message
		phone := strings.TrimPrefix(parts[1], "+") // remove + sign if exists
		fullMessage := strings.Join(parts[2:], " ")

		err := sendWhatsAppMessage(phone+"@c.us", fullMessage)
		if err != nil {
			log.Printf("Failed to forward message to WhatsApp: %v", err)
			http.Error(w, "Failed to forward message", http.StatusInternalServerError)
			return
		}

		log.Printf("Forwarded to WhatsApp: %s → %s", phone, fullMessage)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Message forwarded to WhatsApp"))
		return
	}

	if strings.HasPrefix(msg.Message, "!ping") {
		w.Write([]byte("pong"))
		return
	}

	if strings.HasPrefix(msg.Message, "!help") {
		help := `
Available commands:
!wsp <phone> <message>  - Send a WhatsApp message
!ping                   - Check if service is alive
!help                   - Show this message
`
		w.Write([]byte(help))
		return
	}

	// If not a known command
	log.Printf("Unknown command or plain message: %s", msg.Message)
	w.WriteHeader(http.StatusOK)
}