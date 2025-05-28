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
        print(f"Received packet: {packet}")
        decoded = packet.get('decoded')
        if not decoded:
            print("No decoded field in packet.")
            return

        payload = decoded.get('text')
        print(f"Decoded payload: {payload!r}")
        if not payload or not payload.startswith('!'):
            print("Payload missing or does not start with '!'. Not forwarding.")
            return  # Only forward messages starting with '!'

        sender = packet.get('from', 'unknown')
        to = packet.get('to')
        timestamp = int(time.time())

        print(f"Sender: {sender} (type: {type(sender)}), To: {to} (type: {type(to)})")
        print(f"[FROM: {sender} TO: {to}] Payload: {payload}")

        # Forward command messages to go-router
        data = {
            "message": payload,
            "timestamp": timestamp,
            "from": str(sender),
            "to": str(to),
        }
        
        print(f"Forwarding to go-router: {data}")
        try:
            response = requests.post(GO_ROUTER_URL, json=data)
            print(f"Go-router response status: {response.status_code}")
            print(f"Go-router response text: {response.text!r}")
        except Exception as e:
            print(f"Error sending to go-router: {e}")
            interface.sendText(str(sender), "❌ Could not contact server.")
            return

        # Use the go-router's response as the ack
        if response.status_code == 200:
            ack = response.text.strip() or "✅ Message delivered!"
        else:
            ack = f"❌ Message could not be delivered. (Status: {response.status_code})"

        print(f"Sending ack to device {sender}: {ack!r}")
        try:
            # Ensure sender is an int for destinationId
            dest_id = int(sender) if not isinstance(sender, int) else sender
            interface.sendText(ack, dest_id)
        except Exception as e:
            print(f"Error sending ack to device: {e}")

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
        dest_id = int(to) if not isinstance(to, int) else to
        meshtastic_interface.sendText(message, dest_id)
        return jsonify({'status': 'sent'})
    except Exception as e:
        print(f"Error in sendText: {e}")
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
