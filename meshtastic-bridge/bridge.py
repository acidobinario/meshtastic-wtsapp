import time
import json
import meshtastic
import meshtastic.serial_interface
import requests
from pubsub import pub
import os
from flask import Flask, request, jsonify

GO_ROUTER_URL = os.getenv("GO_ROUTER_URL", "http://go-router:8080/send-message")
HEALTH_CHECK_URL = os.getenv("HEALTH_CHECK_URL", "http://go-router:8080/health")

def wait_for_go_router():
    print("Checking go-router health endpoint...")
    while True:
        try:
            resp = requests.get(HEALTH_CHECK_URL)
            if resp.status_code == 200:
                print("go-router is up!")
                return
        except requests.RequestException:
            pass
        print("Waiting for go-router to be ready...")
        time.sleep(3)

def onReceive(packet, interface):
    try:
        decoded = packet.get('decoded')
        if not decoded:
            return

        payload = decoded.get('text')
        if not payload or not payload.startswith('!'):
            return  # Only forward messages starting with '!'

        sender = packet.get('from', 'unknown')
        to = packet.get('to')
        timestamp = int(time.time())

        print(f"[FROM: {sender} TO: {to}] Payload: {payload}")

        # Forward command messages to go-router
        data = {
            "message": payload,
            "timestamp": timestamp,
            "from": sender,
            "to": to,
        }
        
        print(f"Forwarding to go-router: {data}")
        response = requests.post(GO_ROUTER_URL, json=data)
        # Use the go-router's response as the ack
        if response.status_code == 200:
            ack = response.text.strip() or "✅ Message delivered!"
        else:
            ack = "❌ Message could not be delivered."

        interface.sendText(str(sender), ack)

    except Exception as e:
        print(f"Error handling packet: {e}")

def onConnection(interface, topic=pub.AUTO_TOPIC):  # pylint: disable=unused-argument
    print("onconnection called")
    print(interface.myInfo)

app = Flask(__name__)
meshtastic_interface = None

@app.route('/send-message', methods=['POST'])
def send_message():
    data = request.get_json()
    to = data.get('to')
    message = data.get('message')
    print(f"Received request to send message to {to}: {message}")
    if not to or not message:
        return jsonify({'error': 'to and message are required'}), 400

    try:
        meshtastic_interface.sendText(to, message)
        return jsonify({'status': 'sent'})
    except Exception as e:
        return jsonify({'error': str(e)}), 500

def main():
    global meshtastic_interface
    print("Starting Meshtastic bridge...")
    wait_for_go_router()

    dev_path = os.getenv("MESH_DEVICE_PATH", "/dev/ttyACM0")
    meshtastic_interface = meshtastic.serial_interface.SerialInterface(devPath=dev_path)
    pub.subscribe(onReceive, "meshtastic.receive")
    pub.subscribe(onConnection, "meshtastic.connection.established")

    app.run(host="0.0.0.0", port=8080)

if __name__ == "__main__":
    main()
