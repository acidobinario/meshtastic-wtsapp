import time
import json
import meshtastic
import meshtastic.serial_interface
import requests
from pubsub import pub
import os

GO_ROUTER_URL = os.getenv("GO_ROUTER_URL", "http://go-router:8080/send-message")
HEALTH_CHECK_URL = os.getenv("HEALTH_CHECK_URL", "http://go-router:8080/health")
COMMAND_WHITELIST = ["!wsp", "!ping", "!help"]

def is_whitelisted(message):
    return any(message.startswith(cmd) for cmd in COMMAND_WHITELIST)

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
        if not payload:
            return

        sender = packet.get('from', 'unknown')
        to = packet.get('to')
        timestamp = int(time.time())

        print(f"[FROM: {sender} TO: {to}] Payload: {payload}")

        if not is_whitelisted(payload):
            print("Command not whitelisted, ignoring.")
            return

        data = {
            "message": payload,
            "timestamp": timestamp,
            "from": sender
        }

        response = requests.post(GO_ROUTER_URL, json=data)
        print(f"Forwarded to Go router: {response.status_code} {response.text}")

    except Exception as e:
        print(f"Error handling packet: {e}")

def onConnection(interface, topic=pub.AUTO_TOPIC):  # pylint: disable=unused-argument
    """This is called when we (re)connect to the radio."""
    print("onconnection called")
    print(interface.myInfo)

def main():
    print("Starting Meshtastic bridge...")
    wait_for_go_router()

    dev_path = os.getenv("MESH_DEVICE_PATH", "/dev/ttyACM0")
    interface = meshtastic.serial_interface.SerialInterface(devPath=dev_path)
    pub.subscribe(onReceive, "meshtastic.receive")
    pub.subscribe(onConnection, "meshtastic.connection.established")

    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        print("Stopping bridge...")
        interface.close()

if __name__ == "__main__":
    main()
