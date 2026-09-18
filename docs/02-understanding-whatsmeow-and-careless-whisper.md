# Part 2: Understanding whatsmeow & The "Careless Whisper" Research

This document breaks down the technical concepts behind the **Careless Whisper** vulnerability paper ([arXiv:2411.11194](https://arxiv.org/abs/2411.11194)), the YouTube deep-dive video, and how the **whatsmeow** Go library connects to it.

---

## 🧭 The Big Picture

```
                                  [ WhatsApp Protocol Infrastructure ]
                                              │
              ┌───────────────────────────────┴───────────────────────────────┐
              ▼                                                               ▼
   [ Primary Mobile Device ]                                       [ Linked Companion Client ]
   - iOS / Android WhatsApp App                                    - whatsmeow / Web Client
   - Direct cellular / Wi-Fi                                       - Go-based WebSocket
   - Full Signal Protocol keypair                                  - Pairwise E2EE Signal keys
```

In late 2024, security researchers published:
> **Careless Whisper: Exploiting Silent Delivery Receipts to Monitor Users in End-to-End Encrypted Messaging**  
> *(arXiv:2411.11194, University of Vienna & SBA Research)*

The paper revealed how an architectural property of modern multi-device instant messaging protocols (specifically WhatsApp and Signal) can be turned into a **stealthy presence, activity, and location side-channel** — without sending any visible messages, triggering notifications, or requiring mutual contact status.

---

## 🔐 WhatsApp Multi-Device Protocol Basics

To understand the attack, we first have to understand how modern WhatsApp works under the hood.

### 1. Multi-Device Signal Protocol Architecture
WhatsApp uses the **Signal Protocol** (incorporating Double Ratchet, Curve25519 key agreements, and HMAC-SHA256).

- In legacy WhatsApp (pre-2021), your phone was the sole master. WhatsApp Web was merely a remote desktop proxy streaming from the phone.
- In **Multi-Device WhatsApp (current)**, each linked device (up to 4 companion devices + 1 primary phone) is an **independent cryptographic actor**.
- Each device has its own `IdentityKey`, `SignedPreKey`, and active ratchet sessions.

When someone sends you a message:
1. Their client fetches the directory list of all your active devices.
2. Their client encrypts a separate pairwise ciphertext for your primary phone, your desktop app, your web session, etc.
3. The server fans out these encrypted payloads to each device independently.

---

## ⚡ The Anatomy of Delivery Receipts

WhatsApp uses three distinct tiers of receipts to confirm message flow:

| Receipt Type | Protocol Stanza / Event | Triggered By | User Visible? | Can Be Disabled in Settings? |
| :--- | :--- | :--- | :--- | :--- |
| **Server Ack** | `<ack class="receipt">` | WhatsApp server receives incoming frame | No (Internal) | No |
| **Device Delivery Receipt** | `<receipt type="">` | Recipient device receives & decrypts frame | **Two grey ticks** | **NO** (Mandatory) |
| **Read Receipt** | `<receipt type="read">` | Recipient opens the chat window | **Two blue ticks** | **Yes** (Privacy Settings) |

### The Critical Flaw:
Users can disable **Read Receipts** (the blue ticks) in WhatsApp privacy settings.  
However, users **CANNOT** disable **Device Delivery Receipts** (the second grey tick). The protocol considers device delivery receipts essential to guarantee network delivery and ratchet synchronization.

---

## 🕵️ The "Careless Whisper" Mechanism: Silent Delivery Receipts

In normal operation, sending a message causes:
1. A push notification on the victim's phone (banner, ring, or vibration).
2. A new entry in their chat list.
3. An automatic device delivery receipt sent back to the sender.

If an attacker sent thousands of regular messages to track someone, the victim would obviously notice immediately.

### The Research Breakthrough: "Phantom" Payloads
The researchers discovered that certain types of stanzas cause the recipient device to **decrypt the payload, silently discard it, but STILL issue the low-level device delivery receipt!**

```
Attacker Client (whatsmeow)                 Target Phone
     │                                            │
     │ 1. Send Reaction to Invalid MessageID     │
     ├───────────────────────────────────────────►│ (Decrypted by Signal layer)
     │                                            │
     │                                            │ [App Checks Message ID]
     │                                            │ -> Target ID does not exist!
     │                                            │ -> DISCARD payload silently!
     │                                            │ -> NO notification, NO sound
     │                                            │
     │ 2. Automatic E2EE Device Delivery Receipt  │
     │◄───────────────────────────────────────────┤ (Sent automatically by protocol)
     │                                            │
  (Attacker measures Round-Trip Time)
```

### Why Does This Happen?
1. **Reactions to Non-Existent Messages:** If you send a WhatsApp reaction (emoji reaction) referencing a `MessageID` that was never sent or was deleted, the target's app engine cannot attach the emoji to any chat message. It silently drops it.
2. **Orphaned Message Edits:** If an edit stanza is received for an unknown message ID, the app discards it without alerting the user.
3. **Decryption Must Acknowledge:** Because the Signal Protocol ratchet state advances when a message is decrypted, the protocol layer acknowledges receipt `<receipt type="">` to confirm the ratchet state is synchronized.

**The result:** A 100% covert delivery receipt probe. The victim receives no notification, but the sender receives a millisecond-accurate timestamp from the victim's device.

---

## ⏱️ What Round-Trip Time (RTT) Exposes

By measuring the duration between transmitting the probe and receiving the device delivery receipt, an observer measures the **Round-Trip Time (RTT)**:

$$\text{RTT} = t_{\text{receipt\_arrival}} - t_{\text{probe\_sent}}$$

This timing side-channel leaks extensive physical and device information:

### 1. Screen Active vs. Sleeping (Doze State)
- **Active Device (Screen On / User Typing):**  
  The application CPU is awake, the baseband radio is in high-power active mode (`DCH` / `RRC_CONNECTED`), and the WebSocket or push channel is immediately serviced.  
  👉 **Typical RTT: 150 ms – 350 ms.**
- **Sleeping Device (Deep Sleep / Background):**  
  Modern iOS and Android phones power down background radios to save battery (Doze mode, Apple APNs batching). The incoming probe triggers an Apple Push Notification (APNs) or Google FCM wakeup packet, waking the baseband, transitioning the radio state (`IDLE` $\to$ `CONNECTED`), and waking the CPU.  
  👉 **Typical RTT: 1,500 ms – 4,000+ ms.**

### 2. Network Medium (Wi-Fi vs. Cellular)
- Wi-Fi connections exhibit tight, low-jitter RTT clusters.
- Cellular connections (4G LTE / 5G) exhibit characteristic baseband scheduling latency curves and higher variance.
- When a target leaves home (disconnecting from Wi-Fi and falling back to cellular), their RTT profile immediately shifts.

### 3. Multi-Device Discrimination
Because delivery receipts are emitted by **each linked device individually**, an attacker can see:
- When the victim is working on their MacBook/PC (`companion device ack` arrives fast).
- When the victim leaves their computer and uses their phone.

---

## 🛠️ Where `whatsmeow` Fits In

`whatsmeow` (written by Tulir Asokan) is a comprehensive Go implementation of the WhatsApp Web Multi-Device protocol.

- It handles the **Noise protocol** handshake over WebSockets (`wss://web.whatsapp.com/ws/chat`).
- It manages cryptographic keys, Signal identity sessions, and message protobuf parsing.
- It exposes low-level protocol events via Go channels and callbacks, including:
  - `*events.Message` (incoming messages)
  - `*events.Receipt` (delivery receipts, read receipts, sender acks)
  - `*events.Presence` / `*events.ChatPresence` (typing status)

`whatsmeow` is the engine behind research tools like Careless Whisper because it provides raw, unthrottled access to protocol receipts that standard WhatsApp consumer apps hide behind simple grey tick graphics.

---

## ⚖️ Research Ethics & Lab Safety Boundaries

This repository (`silent-receipts-lab`) is created strictly for **educational exploration, technical learning, and security research**:

1. **Only probe devices you own:** Test between your primary phone and this lab client, or between secondary test accounts you control.
2. **Never covertly monitor third parties:** Probing unsuspecting individuals without explicit consent violates telecommunications privacy laws and WhatsApp terms of service.
3. **Respect Rate Limits:** High-frequency message probing will trigger Meta's anti-abuse telemetry and result in account bans.
