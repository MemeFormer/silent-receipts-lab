package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite" // Pure-Go SQLite (No CGO required)
)

// pendingProbe tracks outgoing test probes to calculate exact Round-Trip Time (RTT).
type pendingProbe struct {
	targetJID  types.JID
	sentAt     time.Time
	serverAck  time.Time
	delivered  time.Time
	readAt     time.Time
}

var (
	probesLock sync.Mutex
	probes     = make(map[types.MessageID]*pendingProbe)
)

func logReceiptAnalysis(receipt *events.Receipt) {
	fmt.Printf("\n----------------------------------------------------------\n")
	fmt.Printf(" [⚡ RECEIPT EVENT DETECTED]\n")
	fmt.Printf("   Type:       %s\n", receipt.Type)
	fmt.Printf("   Source:     %s\n", receipt.SourceString())
	fmt.Printf("   Timestamp:  %s\n", receipt.Timestamp.Format("15:04:05.000"))
	fmt.Printf("   Acknowledged MsgIDs: %v\n", receipt.MessageIDs)

	probesLock.Lock()
	defer probesLock.Unlock()

	for _, msgID := range receipt.MessageIDs {
		if probe, found := probes[msgID]; found {
			now := time.Now()
			rtt := now.Sub(probe.sentAt)

			switch receipt.Type {
			case types.ReceiptTypeDelivered, "":
				probe.delivered = now
				fmt.Printf("\n   🎯 [PROBE MATCH] Target Device Acknowledged Delivery!\n")
				fmt.Printf("      Message ID: %s\n", msgID)
				fmt.Printf("      Target JID: %s\n", probe.targetJID.String())
				fmt.Printf("      Sent At:    %s\n", probe.sentAt.Format("15:04:05.000"))
				fmt.Printf("      Delivered:  %s\n", now.Format("15:04:05.000"))
				fmt.Printf("      ⏱️ Measured Device RTT: %v (%d ms)\n", rtt.Round(time.Millisecond), rtt.Milliseconds())

				// Educational commentary from arXiv:2411.11194
				fmt.Printf("\n   📖 [Careless Whisper Insight]\n")
				if rtt < 400*time.Millisecond {
					fmt.Printf("      → RTT < 400ms: Typical for device actively connected (screen ON or recent activity).\n")
				} else if rtt < 1200*time.Millisecond {
					fmt.Printf("      → RTT 400-1200ms: Standard network latency or light background state.\n")
				} else {
					fmt.Printf("      → RTT > 1200ms: Typical for phone waking from deep sleep / cellular paging (Doze state).\n")
				}

			case types.ReceiptTypeRead:
				probe.readAt = now
				readRTT := now.Sub(probe.sentAt)
				fmt.Printf("\n   👀 [PROBE MATCH] Read Receipt Received!\n")
				fmt.Printf("      Target user opened chat after: %v (%d ms)\n", readRTT.Round(time.Millisecond), readRTT.Milliseconds())
			}
		}
	}
	fmt.Printf("----------------------------------------------------------\n")
}

func main() {
	fmt.Println("==========================================================")
	fmt.Println("   silent-receipts-lab: Delivery Receipt & Timing Lab     ")
	fmt.Println("   Exploring 'Careless Whisper' (arXiv:2411.11194)        ")
	fmt.Println("==========================================================")

	storeDir := "store"
	dbPath := filepath.Join(storeDir, "session.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "\n[!] No session database found at '%s'.\n", dbPath)
		fmt.Fprintf(os.Stderr, "    Please first run: go run ./cmd/minimal\n")
		fmt.Fprintf(os.Stderr, "    to pair your WhatsApp device via QR code.\n\n")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURI := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", dbPath)
	dbLog := waLog.Stdout("DB", "WARN", true)
	container, err := sqlstore.New(ctx, "sqlite", dbURI, dbLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open session database: %v\n", err)
		os.Exit(1)
	}
	defer container.Close()

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil || deviceStore.ID == nil {
		fmt.Fprintf(os.Stderr, "No logged-in device found in database. Run ./cmd/minimal first.\n")
		os.Exit(1)
	}

	clientLog := waLog.Stdout("WA-Lab", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	client.AddEventHandler(func(evt any) {
		switch v := evt.(type) {
		case *events.Receipt:
			logReceiptAnalysis(v)
		case *events.Connected:
			fmt.Printf("\n[🟢 READY] Connected as %s\n", client.Store.ID.User)
		}
	})

	err = client.Connect()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// Wait briefly for connection
	time.Sleep(1 * time.Second)

	fmt.Println("\nSelect an option:")
	fmt.Println("  1. Passive Receipt Monitor (watch all background receipt traffic)")
	fmt.Println("  2. Send Test Probe & Measure Device Delivery RTT (latency test)")
	fmt.Println("  3. Display Careless Whisper Paper Summary & Technical Reference")
	fmt.Println("  4. Exit")
	fmt.Println("")

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("lab> ")
		if !scanner.Scan() {
			break
		}
		choice := strings.TrimSpace(scanner.Text())

		switch choice {
		case "1":
			fmt.Println("\n[📡 Passive Monitoring Active] Waiting for incoming receipt frames...")
			fmt.Println("Send or receive messages on your phone to observe receipts in real time.")
			fmt.Println("Press Ctrl+C or type 'back' to return to menu.")

		case "2":
			fmt.Println("\n-- Send Test Probe --")
			fmt.Println("NOTE: Always test on your own secondary device or with explicit consent.")
			fmt.Print("Enter recipient phone number with country code (e.g. 491701234567 or 15551234567): ")
			if !scanner.Scan() {
				break
			}
			phone := strings.TrimSpace(scanner.Text())
			phone = strings.TrimPrefix(phone, "+")
			if phone == "" {
				fmt.Println("Canceled.")
				continue
			}

			targetJID := types.NewJID(phone, types.DefaultUserServer)
			probeText := fmt.Sprintf("🔬 Lab Probe Ping [%s]", time.Now().Format("15:04:05"))

			msg := &waE2E.Message{
				Conversation: proto.String(probeText),
			}

			sendStart := time.Now()
			resp, err := client.SendMessage(ctx, targetJID, msg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error sending probe message: %v\n", err)
				continue
			}

			serverAckLatency := time.Since(sendStart)
			probesLock.Lock()
			probes[resp.ID] = &pendingProbe{
				targetJID: targetJID,
				sentAt:    sendStart,
				serverAck: time.Now(),
			}
			probesLock.Unlock()

			fmt.Printf("\n[📤 PROBE SENT]\n")
			fmt.Printf("   Target:           %s\n", targetJID.String())
			fmt.Printf("   Message ID:       %s\n", resp.ID)
			fmt.Printf("   Server Ack Time:  %v (%d ms)\n", serverAckLatency.Round(time.Millisecond), serverAckLatency.Milliseconds())
			fmt.Printf("   Waiting for device delivery receipt (<receipt type=\"\">)...\n\n")

		case "3":
			displayPaperSummary()

		case "4", "exit", "quit":
			fmt.Println("Exiting lab.")
			return

		case "back":
			fmt.Println("Returned to main prompt.")

		default:
			fmt.Println("Unknown command. Choices: 1 (Monitor), 2 (Probe), 3 (Summary), 4 (Exit)")
		}
	}
}

func displayPaperSummary() {
	fmt.Println("\n==========================================================")
	fmt.Println("  Careless Whisper: Technical Summary (arXiv:2411.11194)  ")
	fmt.Println("==========================================================")
	fmt.Println(`
1. The Core Mechanism:
   WhatsApp Multi-Device delivers encrypted messages to all registered
   devices using the Signal Protocol (Double Ratchet). When any device
   receives and decrypts a message stanza, it immediately replies with a
   low-level E2EE delivery receipt node: <receipt type="">.

2. Why 'Silent' Receipts Exist:
   If an incoming payload references a non-existent message ID (e.g. an
   orphaned reaction or edit stanza), the client application discards it
   without showing any user notification, vibrating, or alerting the UI.
   However, the underlying crypto layer STILL executes and issues the
   device delivery receipt!

3. What Timing (RTT) Leaks:
   - Screen Active vs Sleep: Awake devices process packets instantly (~150-300ms).
     Sleeping devices must be woken via APNs/FCM and radio transitions (~1500-3000ms+).
   - Network Medium: Wi-Fi vs LTE/5G baseband scheduling latency signatures.
   - Device Separation: Multi-device companion devices reveal individual device states.

4. Research Ethics:
   This tool is strictly for educational experimentation on accounts and
   devices you own or have explicit authorization to test.
`)
	fmt.Println("==========================================================")
}
