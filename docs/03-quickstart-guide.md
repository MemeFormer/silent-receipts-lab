# Part 2: Quickstart Guide & Hands-on Lab

This guide walks you through pairing your first `whatsmeow` client, observing live protocol receipts, and running the latency timing lab.

---

## 🚀 Quickstart in 4 Commands

Once you have completed the [Environment Setup Guide (Part 1)](01-environment-setup.md), open your Terminal and run:

```bash
# 1. Enter the project directory
cd ~/silent-receipts-lab

# 2. Run the preflight environment diagnostic check
./scripts/check-env.sh

# 3. Download Go module dependencies (first time only)
go mod tidy

# 4. Launch the minimal client
go run ./cmd/minimal
```

*(Alternatively, if you have `make` installed: `make check`, `make tidy`, `make run`)*

---

## 📱 Pairing Your WhatsApp Account (Terminal QR Code)

When running `cmd/minimal` for the first time:

1. The client detects that `store/session.db` does not contain an existing session.
2. It generates a cryptographic device keypair and requests a pairing token from WhatsApp's multi-device servers.
3. A pairing QR code will be rendered **directly in your Terminal**:

```
  ▄▄▄▄▄▄▄ ▄ ▄  ▄▄ ▄▄▄▄▄▄▄
  █ ▄▄▄ █ █▀█ █ █ █ ▄▄▄ █
  █ ███ █ █▄█ █▀  █ ███ █
  █▀▀▀▀▀█ █ ▄ █ ▄ █▀▀▀▀▀█
  ...
```

4. **On your phone:**
   - Open **WhatsApp**.
   - Tap **Settings** (or the three dots menu on Android) $\to$ **Linked Devices**.
   - Tap **Link a Device**.
   - Point your phone's camera at the QR code in your Terminal.

5. **Upon successful scan:**
   - The terminal will print:
     ```text
     [✅ SUCCESS] Pairing successful! Session credentials saved to store/session.db
     [✅ SUCCESS] Linked JID: 15551234567@s.whatsapp.net
     [🟢 STATUS] Connected to WhatsApp multi-device websocket servers!
     ```

---

## 🔄 Reconnecting (Subsequent Runs)

The cryptographic session keys are saved locally in `store/session.db` (which is safely protected by `.gitignore`).

When you launch the client again:

```bash
go run ./cmd/minimal
```

It will **automatically resume** the existing session:
```text
[🔑 SESSION] Found existing session for account: 15551234567
[🔑 SESSION] Connecting directly without QR code...
[🟢 STATUS] Connected to WhatsApp multi-device websocket servers!
[✅ READY] Client connected and listening for events. Press Ctrl+C to exit.
```

No need to re-scan the QR code!

---

## 🔬 Observing Live Delivery Receipts & Messages

While `cmd/minimal` is running:

1. Send a message to yourself or receive a message from a friend.
2. Look at the terminal output:
   - When a message arrives:
     ```text
     [📩 MESSAGE RECEIVED] from 15559876543 (Chat: 15559876543@s.whatsapp.net)
        Content: Hello from WhatsApp!
        Message ID: 3EB0A1B2C3D4E5F6 | Timestamp: 2026-09-18T16:00:00Z
     ```
   - When a delivery receipt or read receipt arrives:
     ```text
     [⚡ RECEIPT RECEIVED]
        Type:       delivery (device ack)
        Source:     15559876543@s.whatsapp.net
        Timestamp:  2026-09-18T16:00:01Z
        Target IDs: [3EB0A1B2C3D4E5F6]
     ```

Notice how the receipt arrives almost immediately after the message, acknowledging receipt at the device level.

---

## 🧪 Running the Receipt & Timing Lab (`cmd/receipts`)

Once paired, you can run the specialized Careless Whisper analysis tool:

```bash
go run ./cmd/receipts
```

You will see the interactive lab console:

```text
==========================================================
   silent-receipts-lab: Delivery Receipt & Timing Lab     
   Exploring 'Careless Whisper' (arXiv:2411.11194)        
==========================================================

Select an option:
  1. Passive Receipt Monitor (watch all background receipt traffic)
  2. Send Test Probe & Measure Device Delivery RTT (latency test)
  3. Display Careless Whisper Paper Summary & Technical Reference
  4. Exit

lab>
```

### Option 1: Passive Receipt Monitor
Watches all receipt stanzas passing through your multi-device connection, displaying microsecond-accurate timestamps and payload metadata.

### Option 2: Latency & Delivery RTT Probe
- Enter a phone number you own (e.g. a secondary test device).
- The lab sends a timestamped probe ping.
- When the target device decrypts and returns the `<receipt type="">` stanza, the lab calculates the exact **Round-Trip Time (RTT)** in milliseconds.
- Compare RTT when the target phone is:
  1. **Awake with screen on** (Expected RTT: 150ms – 350ms).
  2. **Asleep on a table for 15 minutes** (Expected RTT: 1500ms – 3500ms+ due to APNs wakeup).

---

## 🧹 Session Management & Reset

### How to Unlink / Log Out:
1. **From your phone:** Go to **WhatsApp** $\to$ **Linked Devices** $\to$ tap **whatsmeow** (or macOS) $\to$ **Log Out**.
2. **From your computer:** Delete the local database file:
   ```bash
   rm -rf store/session.db store/session.db-*
   ```

Running `go run ./cmd/minimal` again will present a fresh QR code.
