import express from 'express';
import bodyParser from 'body-parser';
import qrcode from 'qrcode-terminal';
import pkg from 'whatsapp-web.js';
const { Client, LocalAuth } = pkg;

let isClientReady = false;

console.log('Starting whatsapp-bot...');

const app = express();
app.use(bodyParser.json());

const client = new Client({
  authStrategy: new LocalAuth(),
  puppeteer: {
    executablePath: '/usr/bin/chromium',
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-gpu',
      '--single-process',
      '--no-zygote',
    ],
  }
});

client.on('qr', (qr) => {
  console.log('QR RECEIVED', qr);
  qrcode.generate(qr, { small: true });
});

client.on('ready', () => {
  isClientReady = true;
  console.log('Client is ready!');
});

client.on('message', async message => {
  let quoted = null;
  if (message.hasQuotedMsg) {
    const quotedMsg = await message.getQuotedMessage();
    quoted = {
      id: quotedMsg.id._serialized,
      from: quotedMsg.from,
      body: quotedMsg.body,
    };
  }

  // Forward everything to Go router
  fetch('http://go-router:8080/receive-message', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      from: message.from,
      body: message.body,
      timestamp: message.timestamp,
      id: message.id._serialized,
      quoted,
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
    const sentMsg = await client.sendMessage(chatId, message);
    res.json({ status: 'Message sent', id: sentMsg.id._serialized });
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
