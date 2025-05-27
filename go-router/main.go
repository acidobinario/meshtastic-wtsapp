package main

import (
	"bytes"
	// "context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
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

func main() {
	log.Println("go-router starting, waiting for whatsapp-bot to be ready...")

	// Wait until whatsapp-bot is responding
	waitForWhatsAppBot()

	// Send test message
	err := sendWhatsAppMessage("56977788092@c.us", "Hello from go-router test!")
	if err != nil {
		log.Fatalf("Failed to send test message: %v", err)
	}
	log.Println("Test message sent successfully.")

	// Start HTTP server to receive messages from whatsapp-bot
	http.HandleFunc("/receive-message", receiveMessageHandler)
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
