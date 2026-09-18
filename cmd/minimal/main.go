package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	// Pure-Go SQLite driver: modernc.org/sqlite
	// WHY THIS MATTERS ON OCLP / INTEL MAC:
	// Unlike github.com/mattn/go-sqlite3, this driver is written in 100% pure Go.
	// That means it does NOT need CGO, gcc, or Apple Xcode Command Line Tools!
	// You can compile and run smoothly on macOS Ventura/Sonoma with CGO_ENABLED=0.
	_ "modernc.org/sqlite"
)

// eventCallback is called whenever whatsmeow receives an event from the WhatsApp servers.
func eventCallback(evt any) {
	switch v := evt.(type) {

	case *events.Message:
		sender := v.Info.Sender.User
		chat := v.Info.Chat.String()
		msgText := v.Message.GetConversation()
		if msgText == "" && v.Message.GetExtendedTextMessage() != nil {
			msgText = v.Message.GetExtendedTextMessage().GetText()
		}

		fmt.Printf("\n[📩 MESSAGE RECEIVED] from %s (Chat: %s)\n", sender, chat)
		if msgText != "" {
			fmt.Printf("   Content: %s\n", msgText)
		} else {
			fmt.Printf("   (Media / non-text message type: %s)\n", v.Info.Type)
		}
		fmt.Printf("   Message ID: %s | Timestamp: %s\n", v.Info.ID, v.Info.Timestamp.Format(time.RFC3339))

	case *events.Receipt:
		// Receipts are central to the "Careless Whisper" research!
		// They indicate delivery or read status across the multi-device network.
		receiptType := string(v.Type)
		if receiptType == "" {
			receiptType = "delivery (device ack)"
		}

		fmt.Printf("\n[⚡ RECEIPT RECEIVED]\n")
		fmt.Printf("   Type:       %s\n", receiptType)
		fmt.Printf("   Source:     %s\n", v.SourceString())
		fmt.Printf("   Timestamp:  %s\n", v.Timestamp.Format(time.RFC3339))
		fmt.Printf("   Target IDs: %v\n", v.MessageIDs)

	case *events.Connected:
		fmt.Println("\n[🟢 STATUS] Connected to WhatsApp multi-device websocket servers!")

	case *events.LoggedOut:
		fmt.Println("\n[🔴 STATUS] Device was logged out / unlinked from mobile phone. Session invalidated.")

	case *events.StreamReplaced:
		fmt.Println("\n[⚠️ STATUS] Session stream replaced by another client connection.")
	}
}

func main() {
	fmt.Println("==========================================================")
	fmt.Println("  silent-receipts-lab: Minimal whatsmeow Client")
	fmt.Println("  Running with pure-Go SQLite (No Xcode CLT / CGO required)")
	fmt.Println("==========================================================")

	// Step 1: Ensure store directory exists for session storage
	storeDir := "store"
	if err := os.MkdirAll(storeDir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session directory '%s': %v\n", storeDir, err)
		os.Exit(1)
	}

	// Step 2: Configure SQLite database connection string
	// IMPORTANT FOR modernc.org/sqlite:
	// whatsmeow requires SQLite foreign keys enabled. For modernc.org/sqlite,
	// the correct pragma query parameter syntax is:
	// ?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)
	dbPath := filepath.Join(storeDir, "session.db")
	dbURI := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", dbPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbLog := waLog.Stdout("DB", "WARN", true)
	container, err := sqlstore.New(ctx, "sqlite", dbURI, dbLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize SQLite session store: %v\n", err)
		os.Exit(1)
	}
	defer container.Close()

	// Step 3: Get or create the device record
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get device record: %v\n", err)
		os.Exit(1)
	}

	// Step 4: Initialize the whatsmeow client
	clientLog := waLog.Stdout("WA-Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventCallback)

	// Step 5: Handle pairing or resume existing session
	if client.Store.ID == nil {
		// No existing session: initiate QR code pairing
		fmt.Println("\n[ℹ️ PAIRING] No saved session found in", dbPath)
		fmt.Println("[ℹ️ PAIRING] Requesting pairing QR code from WhatsApp...")

		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create QR channel: %v\n", err)
			os.Exit(1)
		}

		err = client.Connect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to WhatsApp: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n==========================================================")
		fmt.Println("  INSTRUCTIONS:")
		fmt.Println("  1. Open WhatsApp on your primary phone.")
		fmt.Println("  2. Go to Settings -> Linked Devices -> Link a Device.")
		fmt.Println("  3. Point your camera at the QR code below.")
		fmt.Println("==========================================================\n")

		for evt := range qrChan {
			switch evt.Event {
			case "code":
				// Render QR code directly in the terminal
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				fmt.Println("\nScan the QR code above to link this client.")

			case "success":
				fmt.Println("\n[✅ SUCCESS] Pairing successful! Session credentials saved to", dbPath)
				fmt.Printf("[✅ SUCCESS] Linked JID: %s\n", client.Store.ID.String())

			case "timeout":
				fmt.Println("\n[⏱️ TIMEOUT] Pairing timed out. Please restart the client to try again.")

			case "error":
				fmt.Fprintf(os.Stderr, "\n[❌ ERROR] Pairing failed: %v\n", evt.Error)
			}
		}
	} else {
		// Existing session found! Reconnecting directly
		fmt.Println("\n[🔑 SESSION] Found existing session for account:", client.Store.ID.User)
		fmt.Println("[🔑 SESSION] Connecting directly without QR code...")

		err = client.Connect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect with existing session: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[✅ READY] Client connected and listening for events. Press Ctrl+C to exit.")
	}

	// Step 6: Block until interrupted (Ctrl+C)
	<-ctx.Done()
	fmt.Println("\n\nShutting down gracefully...")
	client.Disconnect()
	fmt.Println("Disconnected. Goodbye!")
}
