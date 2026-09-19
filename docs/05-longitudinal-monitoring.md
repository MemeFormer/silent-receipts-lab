# Part 3: Longitudinal Monitoring — Automated Logging & Visual Report

You’ve proven the single-probe RTT works (`2527 ms` → deep sleep). Now let’s turn that into **continuous, consensual science**: log RTT every 60–120 s for minutes/hours and automatically generate a beautiful chart.

This is exactly what `cmd/monitor` is for.

---

## 🎯 Why a Monitor (vs. the one-shot `cmd/receipts`)?

| Tool | Purpose |
| :--- | :--- |
| `cmd/minimal` | Pair, keep a session alive, see all incoming traffic |
| `cmd/receipts` `→ 2` | Fire **one** manual probe and watch the receipt interactively |
| **`cmd/monitor`** | **Automated longitudinal study:** send *N* visible, consented probes at a fixed interval, log every `serverAck` + `device RTT` to CSV, and regenerate a live HTML report with Chart.js after each probe |

The monitor uses **the same timing physics** as the “silent” phantom described in the paper — but intentionally sends a **visible message** (`🔬 Lab Probe [15:04:05 #12]`). The server/device RTT leakage is identical; visibility is what keeps it ethical.

> **Why not silent?** Silent probes (e.g. a reaction to a non-existent `MessageID`) trigger `receipt type=""` **without any banner/sound**. That stealth is precisely the privacy risk the paper warns about. Automating it would build a covert tracker. Automating a *visible* probe gives you the same data, but the target *consents and sees every probe*.

---

## ⚠️ Responsible Use Checklist (Required)

Before you start `cmd/monitor`, confirm:

1. **Explicit consent** for this exact experiment Window (you already did — your family deal with “my number is the target, unlimited data, battery already degraded” ✅).
2. **Visible probes only.** Every probe appears in the WhatsApp chat. The target can mute/leave any time.
3. **Rate-limit:** `≥ 30 s` enforced by the tool (recommended `90 s`). Faster probing = noticeable spam, battery drain, and potential WhatsApp rate-limit/ban.
4. **Data awareness:** ~1–3 KB per probe. `90 s` → ~2.5 MB/hour, `60 s × 100 probes` → ~200 KB. Your “unlimited” covers you; your partner’s quota is the limit — as you already double-checked for roaming/vacation, nice.
5. **Self-JID note:** Probing `4917656345405` (your *own* paired number) self-acks instantly on the server — it does **not** produce a meaningful cross-device sleep/wake RTT. That’s why the docs’ “send yourself a message” felt confusing — you’re right! Use a **second device you own** or the consenting partner’s number (`4915756941992` in your log). WhatsApp’s “Message yourself” chat exists, but it’s not a useful Careless Whisper sample.

The monitor will ask you to type `YES` on an interactive terminal to confirm the above.

---

## 🚀 Quick Start

### 1. You already passed preflight ✅

```bash
./scripts/check-env.sh
# 14.8.3 x86_64, Go 1.27.1, store/session.db present
```

### 2. Run a short trial — 10 probes every 90 s (~15 min)

```bash
go run ./cmd/monitor -target 4915756941992 -interval 90s -count 10
# Type YES at the consent prompt
```

You’ll see:

```
📄 Logging to data/rtt_4915756941992_20260919-034539.csv
[seq 1] 📤 Sent 3EB01A8ADB60AEE662F250  serverAck 887 ms  awaiting device receipt...
        ✅ Delivered — RTT 2527 ms (deep-sleep/doze)  [03:45:38.606]
        📄 Logged → data/...csv  |  📊 data/...html updated (open with: open "data/...html")
        💤 Sleeping 1m27s until next probe...
```

### 3. Open the live report

```bash
open data/rtt_4915756941992_20260919-034539.html
# or: go run ./cmd/monitor -plot-only data/rtt_4915756941992_20260919-034539.csv
```

The HTML contains:
- **RTT over time** (line chart, color = inferred state: green <400 ms awake, yellow 400-1200 ms, red >1200 ms deep sleep)
- **Server Ack vs Device RTT** scatter
- Stats (avg, median, min/max, delivery %)
- Raw table
- Plain-English interpretation of your 2527 ms sample (APNs paging + radio ramp)

The report **regenerates after every probe**, so keep the browser tab open and refresh.

---

## 🕒 Longer Runs

```bash
# 30 probes every 2 min (~1 hour)
go run ./cmd/monitor -target 4915756941992 -interval 2m -count 30

# Overnight 8 h, every 90 s (stop with Ctrl+C)
go run ./cmd/monitor -target 4915756941992 -duration 8h -interval 90s

# Custom visible prefix
go run ./cmd/monitor -target 4915756941992 -interval 60s -count 50 -message "🧪 consented-probe"

# Different output location
go run ./cmd/monitor -target 4915756941992 -out ~/Desktop/rtt.csv -interval 90s -count 20
```

**Resume:** If you re-run with the same `-out data/...csv`, the monitor detects the existing file, resumes `seq` numbering, and includes history in the new HTML.

**Regenerate without sending:**

```bash
go run ./cmd/monitor -plot-only data/rtt_4915756941992_20260919-034539.csv
open data/rtt_4915756941992_20260919-034539.html
```

---

## 📄 CSV Format

`data/rtt_<target>_<timestamp>.csv`

```
seq,timestamp_iso,sent_at_rfc3339,message_id,target_jid,server_ack_ms,device_rtt_ms,status,classification
1,2026-09-19T03:45:36+02:00,2026-09-19T03:45:36.078+02:00,3EB01A8ADB60AEE662F250,4915756941992@s.whatsapp.net,887,2527,delivered,deep-sleep/doze
2,2026-09-19T03:47:06+02:00,2026-09-19T03:47:06.123+02:00,3A5AE3BA2D9BA51A9056,4915756941992@s.whatsapp.net,412,184,delivered,awake/fast
3,...,timeout,timeout
```

- `device_rtt_ms = -1` + `status=timeout` means no `receipt type=""` within 20 s (phone offline, WhatsApp killed, or no internet).
- `classification` thresholds match the paper’s empirical ranges and your observed `2527 ms` → `deep-sleep/doze`.

Open directly in **Numbers, Excel, or `python -m csv`**, or feed to your own plotting notebook.

---

## 🔍 Reading the Graph

- **Flat green band ~150–350 ms:** Phone awake, WhatsApp WebSocket hot (screen on recently).
- **Yellow spikes 400–1200 ms:** Screen-off transition, Wi-Fi ↔ LTE handover, light Doze.
- **Red excursions >1200 ms (like your 2.5 s):** Device paged via FCM/APNs, cellular baseband/radio wake-up, exactly what the paper exploits.
- **Timeouts:** Horizontal gaps. Correlates with airplane mode / force-quit.

Compare with ground truth: ask your partner to note “I turned screen off at 03:46” and align with the chart.

---

## 🛜 Your Hotspot / QR Gotcha

You nailed it:

> WhatsApp’s in-app QR scanner refuses to launch while iOS detects **Personal Hotspot is active** (it shows “Hotspot is on — turn it off to scan”), and since your Mac’s uplink *is* that hotspot, the WebSocket handshake stalls.

**Workarounds you discovered (documented for completeness):**
1. **Temporarily disable Hotspot → pair → re-enable:** Pair on a different uplink (café Wi-Fi, USB Ethernet, or the Big Sur fallback partition tethered via USB without hotspot). Once `store/session.db` is written, the session resumes fine over hotspot.
2. **Pair via WhatsApp Web on desktop Chrome over hotspot, then reuse `store/`:** The pairing QR is the same multi-device flow; the resulting `store/session.db` is portable.

After pairing, the WebSocket is just TLS over your hotspot — no QR needed again until logout (14 days inactivity or manual unlink).

---

## ❓ “Send yourself a message” — what did the docs mean?

You’re right to be confused. WhatsApp *does* have a **“Message yourself”** chat (`yourNumber@s.whatsapp.net` talking to itself), but:

- With `whatsmeow`, `Client.SendMessage(yourOwnJID, ...)` loops back and the server ACKs it almost instantly. There is **no cross-device delivery** and **no sleep-state RTT** because it’s the same logical user — you’re measuring loopback latency, not the partner’s phone state.
- So “send yourself a test message” in early docs just meant “verify your own client can send *anything*”, not “measure Careless Whisper on yourself”. For state leakage you need **a second physical device with a distinct number** (your unlimited-data number → partner’s number, as in your deal). Perfectly done.

---

## 🔧 Makefile Shortcuts

```bash
make monitor ARGS="-target 4915756941992 -interval 90s -count 10"
make plot CSV=data/rtt_4915756941992_20260919-034539.csv
```

See `make help`.

---

## 🔐 Sharing Results at Month-End (Your Deal)

At the start of the new month when quotas reset:

1. Both bring your `data/rtt_*.csv` + `*.html`.
2. Compare your `awake / light / deep` histograms vs. partner’s actual “I was asleep at X” notes.
3. Discuss mitigations: disabling background app refresh, Doze tuning differences, and why the paper proposes randomized receipt delays vs. instant ACK.

This keeps it **educational, reversible, and fully consented** — exactly the spirit of Responsible Research.
