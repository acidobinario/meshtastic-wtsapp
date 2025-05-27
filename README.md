# Meshtastic WhatsApp Bot

This project provides a simple WhatsApp bot service that connects WhatsApp messages to a Go-based backend, enabling integration between WhatsApp and your Meshtastic network or other services.

## Features

- **WhatsApp Web Integration:** Uses [whatsapp-web.js](https://github.com/pedroslopez/whatsapp-web.js) for WhatsApp connectivity.
- **QR Code Login:** Displays a QR code in the terminal for easy authentication.
- **REST API:** Exposes an HTTP API to send WhatsApp messages programmatically.
- **Message Forwarding:** Forwards incoming WhatsApp messages to a Go service for further processing.
- **Ping Command:** Responds to `!ping` messages with `pong` for health checks.

## Getting Started

### Prerequisites

- Node.js (v16+ recommended)
- npm
- A running Go service (optional, for message forwarding)
- Google Chrome or Chromium (required by Puppeteer)

### Installation

1. **Clone the repository:**
   ```sh
   git clone https://github.com/yourusername/meshtastic-wtsapp.git
   cd meshtastic-wtsapp/whatsapp-bot
   ```

2. **Install dependencies:**
   ```sh
   npm install
   ```

3. **Start the bot:**
   ```sh
   npm start
   ```

4. **Scan the QR code:**  
   When prompted, scan the QR code in your terminal with your WhatsApp mobile app.

### API Usage

#### Health Check

- **GET /**  
  Returns `200 OK` if the WhatsApp client is ready, otherwise `503 Service Unavailable`.

#### Send a WhatsApp Message

- **POST /send-message**
  - **Body:**  
    ```json
    {
      "number": "1234567890",
      "message": "Hello from API!"
    }
    ```
  - **Response:**  
    ```json
    { "status": "Message sent" }
    ```

### Message Forwarding

Incoming WhatsApp messages are automatically forwarded to your Go service at `http://go-router:8080/receive-message` as a JSON POST request.

## Configuration

- **Go Service URL:**  
  Change the URL in `index.js` if your Go service runs elsewhere.

## Development

- **Node modules and lock files** are excluded from version control via `.gitignore`.

## License

MIT

---

**Note:** This project is not affiliated with WhatsApp or Facebook.