import express from 'express';
import bodyParser from 'body-parser';
import qrcode from 'qrcode-terminal';
import pkg from 'whatsapp-web.js';
const { Client, LocalAuth } = pkg;

let isClientReady = false;

const app = express();
app.use(bodyParser.json());

const client = new Client({
  authStrategy: new LocalAuth(),
  puppeteer: {
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  }
});

client.on('qr', (qr) => {
    // Generate and scan this code with your phone
    // Print QR code to terminal for easy scanning
    qrcode.generate(qr, { small: true });
    console.log('QR RECEIVED', qr);
});


client.on('ready', () => {
  isClientReady = true;
  console.log('Client is ready!');
});

client.on('message', message => {
  console.log('Received message:', message.body);

  if (message.body == '!ping') {
    message.reply('pong');
    //exit
    // return;
  }

  // Forward incoming message to your Go service
  // Example: POST to Go API (replace with your Go service URL)
  fetch('http://go-router:8080/receive-message', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      from: message.from,
      body: message.body,
      timestamp: message.timestamp
    }),
  }).catch(console.error);
});

// API endpoint to send a WhatsApp message
app.post('/send-message', async (req, res) => {
  const { number, message } = req.body;
  if (!number || !message) {
    return res.status(400).json({ error: 'number and message are required' });
  }

  try {
    const chatId = number.includes('@c.us') ? number : `${number}@c.us`;
    await client.sendMessage(chatId, message);
    res.json({ status: 'Message sent' });
  } catch (err) {
    console.error(err);
    res.status(500).json({ error: 'Failed to send message' });
  }
});

app.get('/', (req, res) => {
  if (isClientReady) {
    res.status(200).send('OK');
  } else {
    res.status(503).send('WhatsApp client not ready');
  }
});


const PORT = process.env.PORT || 3000;
app.listen(PORT, () => {
  console.log(`WhatsApp bot API listening on port ${PORT}`);
});

client.initialize();
