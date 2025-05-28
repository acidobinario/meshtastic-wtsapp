package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	whatsappBotURL      = "http://whatsapp-bot:3000"
	sendMessageEndpoint = whatsappBotURL + "/send-message"
	healthCheckEndpoint = whatsappBotURL + "/"
	bridgeURL           = "http://meshtastic-bridge:8080/send-message"
	maxMessagesPerMinute = 10 // adjust as needed
)

// QuotedMessage represents a message that can be quoted in WhatsApp.
// It contains the ID of the message, the sender's name, and the body of the message.
type QuotedMessage struct {
	ID   string `json:"id"`
	From string `json:"from"`
	Body string `json:"body"`
}

// WhatsAppMessage represents incoming WhatsApp messages forwarded by the bot.
type WhatsAppMessage struct {
	From      string         `json:"from"`
	Body      string         `json:"body"`
	Timestamp int64          `json:"timestamp"`
	ID        string         `json:"id"`
	Quoted    *QuotedMessage `json:"quoted,omitempty"`
}

// MeshtasticMessage represents incoming messages from meshtastic
type MeshtasticMessage struct {
	To        string `json:"to"`
	From 	  string `json:"from"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

// Map WhatsApp message ID to Meshtastic device ID
var (
	replyMap = make(map[string]string)
	mapMu    sync.Mutex
)

type rateLimiter struct {
    mu        sync.Mutex
    lastReset time.Time
    count     int
}

var deviceLimiters = make(map[string]*rateLimiter)
var deviceLimitersMu sync.Mutex

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

func sendWhatsAppMessage(number, message string) (string, error) {
	payload := map[string]string{
		"number":  number,
		"message": message,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(sendMessageEndpoint, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.ID, nil
}

func allowMessage(deviceID string) bool {
    deviceLimitersMu.Lock()
    rl, exists := deviceLimiters[deviceID]
    if !exists {
        rl = &rateLimiter{lastReset: time.Now(), count: 0}
        deviceLimiters[deviceID] = rl
    }
    deviceLimitersMu.Unlock()

    rl.mu.Lock()
    defer rl.mu.Unlock()
    now := time.Now()
    if now.Sub(rl.lastReset) > time.Minute {
        rl.lastReset = now
        rl.count = 0
    }
    if rl.count >= maxMessagesPerMinute {
        return false
    }
    rl.count++
    return true
}

func receiveMessageHandler(w http.ResponseWriter, r *http.Request) {
	var msg WhatsAppMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("Received WhatsApp message from %s: %s\n", msg.From, msg.Body)

	// If this is a reply to a WhatsApp message, try to route it back to Meshtastic
	if msg.Quoted != nil && msg.Quoted.ID != "" {
		mapMu.Lock()
		deviceID, ok := replyMap[msg.Quoted.ID]
		mapMu.Unlock()
		if ok {
			// Forward reply to Meshtastic device
			forwardToMeshtastic(deviceID, msg.Body)
			log.Printf("Forwarded WhatsApp reply to Meshtastic device %s", deviceID)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
    var msg MeshtasticMessage
    if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Rate limiting per device
    if !allowMessage(msg.From) {
        http.Error(w, "❌ Rate limit exceeded. Please wait before sending more messages.", http.StatusTooManyRequests)
        return
    }

    log.Printf("Received Meshtastic message from %s: %s", msg.From, msg.Message)

    // Handle !help
    if strings.HasPrefix(msg.Message, "!help") {
        helpText := "🤖 Available commands:\n" +
            "!wsp <phone> <message> - Send WhatsApp message\n" +
            "!ping - Check if the bridge is alive\n" +
            "!help - Show this help message"
        w.Write([]byte(helpText))
        return
    }

    // Handle !ping
    if strings.HasPrefix(msg.Message, "!ping") {
        w.Write([]byte("pong"))
        return
    }

    // Handle !clima (weather)
    if strings.HasPrefix(msg.Message, "!clima") {
        parts := strings.Fields(msg.Message)
        city := "Santiago" // Ciudad por defecto
        if len(parts) >= 2 {
            city = strings.Join(parts[1:], " ")
        }
        weatherURL := "https://wttr.in/" + url.QueryEscape(city) +
            "?format=%x+Humedad:%h+Temp:%t+Sensación:%f+Lugar:%l&lang=es"
        resp, err := http.Get(weatherURL)
        if err != nil {
            w.Write([]byte("No se pudo obtener el clima."))
            return
        }
        defer resp.Body.Close()
        weather, _ := io.ReadAll(resp.Body)
        w.Write([]byte(string(weather)))
        return
    }

    // Handle !sismo (earthquake)
    if strings.HasPrefix(msg.Message, "!sismo") {
        // USGS GeoRSS feed for earthquakes
        feedURL := "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/2.5_day.atom"
        resp, err := http.Get(feedURL)
        if err != nil {
            w.Write([]byte("No se pudo obtener información de sismos."))
            return
        }
        defer resp.Body.Close()
        var feed Feed
        if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
            w.Write([]byte("No se pudo leer la información de sismos."))
            return
        }
        // Buscar los sismos en Chile
        found := false
        for _, entry := range feed.Entries {
            matched, _ := regexp.MatchString(`Chile`, entry.Title)
            if matched {
                w.Write([]byte("Último sismo en Chile:\n" + entry.Title))
                found = true
                break
            }
        }
        if !found {
            w.Write([]byte("No hay sismos recientes en Chile."))
        }
        return
    }

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

        // Send WhatsApp message and store mapping
        id, err := sendWhatsAppMessage(phone+"@c.us", fullMessage)
        if err != nil {
            log.Printf("Failed to forward message to WhatsApp: %v", err)
            http.Error(w, "❌ Could not send WhatsApp message.", http.StatusInternalServerError)
            return
        }
        mapMu.Lock()
        replyMap[id] = msg.From // CORRECT: this maps to the original sender
        mapMu.Unlock()
        log.Printf("Mapped WhatsApp msg ID %s to device %s", id, msg.From)
        w.Write([]byte("✅ WhatsApp message sent!"))
        return
    }

    w.Write([]byte("✅ Command received!"))
}

// Forwards a message to a Meshtastic device via the bridge
func forwardToMeshtastic(deviceID, message string) {
	payload := map[string]string{
		"to":      deviceID,
		"message": message,
	}
	body, _ := json.Marshal(payload)
	http.Post(bridgeURL, "application/json", bytes.NewBuffer(body))
}

// For USGS GeoRSS feed (last 24h, M2.5+ worldwide)
type Entry struct {
    Title string `xml:"title"`
    Summary string `xml:"summary"`
}
type Feed struct {
    Entries []Entry `xml:"entry"`
}
