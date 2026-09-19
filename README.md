# 🔬 silent-receipts-lab

An educational security research laboratory exploring WhatsApp Multi-Device delivery receipts, state synchronization, and the **"Careless Whisper"** timing vulnerability ([arXiv:2411.11194](https://arxiv.org/abs/2411.11194)) using [`whatsmeow`](https://github.com/tulir/whatsmeow).

Designed specifically for clean, dependency-free development on **Intel macOS (OCLP Ventura/Sonoma)** using official Go distributions and **pure-Go SQLite** (no Xcode Command Line Tools or Homebrew required).

---

## 📌 Background & Inspiration

- **The Video:** [YouTube: WhatsApp Multi-Device Vulnerability](https://www.youtube.com/watch?v=HHEQVXNCrW8)
- **The Research Paper:** *Careless Whisper: Exploiting Silent Delivery Receipts to Monitor Users in End-to-End Encrypted Messaging* ([arXiv:2411.11194](https://arxiv.org/abs/2411.11194))
- **The Core Library:** [`tulir/whatsmeow`](https://github.com/tulir/whatsmeow) — Open-source Go implementation of the WhatsApp Web Multi-Device protocol.

---

## 🗂️ Repository Structure

```text
silent-receipts-lab/
├── cmd/
│   ├── minimal/               # Minimal whatsmeow client (pairing, listening, session management)
│   │   └── main.go
│   ├── receipts/              # Careless Whisper timing & delivery receipt analysis lab (single probe)
│   │   └── main.go
│   └── monitor/               # Longitudinal monitor: automated CSV logging + HTML Chart.js report
│       └── main.go
├── data/                      # Generated RTT logs (*.csv + *.html, git-ignored)
├── docs/
│   ├── 01-environment-setup.md # Part 1: Intel Mac OCLP, manual Go, no-brew, no-CLT guide
│   ├── 02-understanding-whatsmeow-and-careless-whisper.md # Part 2: Protocol architecture & vulnerability deep-dive
│   ├── 03-quickstart-guide.md # Part 2: Step-by-step pairing & running the lab
│   ├── 04-troubleshooting-and-faq.md # Part 2: Common issues, DSN fixes, and FAQs
│   └── 05-longitudinal-monitoring.md # Part 3: Automated logging, plotting & hotspot/consent notes
├── scripts/
│   └── check-env.sh           # Environment & toolchain preflight diagnostic script
├── store/                     # Local SQLite session database (auto-created, git-ignored)
├── Makefile                   # Convenient shortcuts (make check, make run, make lab, make monitor)
├── go.mod                     # Go module definition
├── .gitignore                 # Prevents committing secrets or session credentials
└── README.md
```

---

## 🧭 The Two-Part Roadmap

### 📦 Part 1: Requirements & Environment Setup
- **Target Machine:** Intel Mac (`MacBookPro11,1`, `x86_64`) on OCLP Ventura or Sonoma.
- **Manual Go Install:** Official Go distribution from [go.dev/dl](https://go.dev/dl) (`darwin-amd64`).
- **No Homebrew:** Avoids complex package managers and bloated dependency graphs.
- **No Apple Command Line Tools / CGO:** Uses `modernc.org/sqlite` (pure Go), eliminating dependencies on `clang`/`gcc` or Xcode CLT.
- 👉 **[Read Part 1: Environment Setup Guide](docs/01-environment-setup.md)**

### 🚀 Part 2: whatsmeow & Careless Whisper Exploration
- **Minimal Client (`cmd/minimal`):** Pairs your device via an in-terminal QR code, saves credentials in `store/session.db`, and displays incoming messages and delivery receipts.
- **Receipt & Timing Lab (`cmd/receipts`):** Measures Round-Trip Time (RTT) of device delivery receipts (`<receipt type="">`), demonstrating how mobile sleep/wake states leak through the timing side-channel.
- **Longitudinal Monitor (`cmd/monitor`):** Automated, consensual RTT logging at fixed intervals with live CSV + interactive HTML report — your requested automation & visualization.
- 👉 **[Read Part 2: Understanding Careless Whisper](docs/02-understanding-whatsmeow-and-careless-whisper.md)**
- 👉 **[Read Part 2: Quickstart Guide](docs/03-quickstart-guide.md)**
- 👉 **[Read Part 3: Longitudinal Monitoring (automation & graphs)](docs/05-longitudinal-monitoring.md)**
- 👉 **[Read Troubleshooting & FAQ](docs/04-troubleshooting-and-faq.md)**

---

## ⚡ Quickstart

```bash
# 1. Run the environment check
./scripts/check-env.sh

# 2. Synchronize module dependencies
go mod tidy

# 3. Launch the minimal client and scan the terminal QR code
go run ./cmd/minimal

# 4. Run the receipt timing lab (single probe)
go run ./cmd/receipts

# 5. Run the longitudinal monitor (automated logging + graph)
go run ./cmd/monitor -target 4915756941992 -interval 90s -count 10
open data/rtt_4915756941992_*.html
```

*(Or use the Makefile: `make check`, `make run`, `make lab`, `make monitor ARGS="-target ..."` , `make build`)*

---

## ⚖️ Responsible Research & Disclaimer

This project is created strictly for **educational and research purposes**. All testing and experimentation should be performed exclusively on accounts and hardware you own or have explicit authorization to inspect. Probing third parties without consent violates communications privacy regulations and WhatsApp terms of service.
