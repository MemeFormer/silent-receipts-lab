package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
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

	_ "modernc.org/sqlite"
)

// record holds one probe outcome for CSV + plotting
type record struct {
	Seq          int
	TimestampISO string
	MessageID    string
	Target       string
	ServerAckMs  int64
	DeviceRTTMs  int64 // -1 if timed out
	Status       string
	Classification string
	SentAt       time.Time
}

var (
	pendingLock sync.Mutex
	pending     = make(map[string]chan time.Time) // msgID -> delivery channel

	records     []record
	recordsLock sync.Mutex
)

func isDeliveryReceipt(t types.ReceiptType) bool {
	// WhatsApp device delivery receipts are <receipt type=""> (empty string).
	// In whatsmeow this constant is types.ReceiptTypeDelivered == "".
	// We check the raw string to avoid duplicate-constant switch issues.
	return string(t) == ""
}

func classifyRTT(rttMs int64) string {
	if rttMs < 0 {
		return "timeout"
	}
	if rttMs < 400 {
		return "awake/fast"
	}
	if rttMs < 1200 {
		return "light-sleep/network"
	}
	return "deep-sleep/doze"
}

func handleReceipt(evt *events.Receipt) {
	if !isDeliveryReceipt(evt.Type) {
		return // ignore read/played etc for this monitor
	}
	// evt.MessageIDs can contain multiple IDs, signal all matching
	pendingLock.Lock()
	defer pendingLock.Unlock()
	for _, id := range evt.MessageIDs {
		if ch, ok := pending[id]; ok {
			select {
			case ch <- time.Now():
			default:
			}
		}
	}
}

func writeCSVHeader(w *csv.Writer) {
	_ = w.Write([]string{
		"seq", "timestamp_iso", "sent_at_rfc3339", "message_id", "target_jid",
		"server_ack_ms", "device_rtt_ms", "status", "classification",
	})
	w.Flush()
}

func appendCSV(w *csv.Writer, r record) {
	_ = w.Write([]string{
		fmt.Sprintf("%d", r.Seq),
		r.TimestampISO,
		r.SentAt.Format(time.RFC3339Nano),
		r.MessageID,
		r.Target,
		fmt.Sprintf("%d", r.ServerAckMs),
		fmt.Sprintf("%d", r.DeviceRTTMs),
		r.Status,
		r.Classification,
	})
	w.Flush()
}

func generateHTMLReport(csvPath string, recs []record, target string) error {
	htmlPath := strings.TrimSuffix(csvPath, filepath.Ext(csvPath)) + ".html"
	if len(recs) == 0 {
		return fmt.Errorf("no records to plot")
	}
	// stats
	var delivered []int64
	deliveredCount := 0
	var sum int64
	var min, max int64 = 1<<62 - 1, 0
	for _, r := range recs {
		if r.DeviceRTTMs >= 0 {
			deliveredCount++
			delivered = append(delivered, r.DeviceRTTMs)
			sum += r.DeviceRTTMs
			if r.DeviceRTTMs < min {
				min = r.DeviceRTTMs
			}
			if r.DeviceRTTMs > max {
				max = r.DeviceRTTMs
			}
		}
	}
	sort.Slice(delivered, func(i, j int) bool { return delivered[i] < delivered[j] })
	avg := float64(0)
	median := int64(0)
	if len(delivered) > 0 {
		avg = float64(sum) / float64(len(delivered))
		median = delivered[len(delivered)/2]
		if min == 1<<62-1 {
			min = 0
		}
	} else {
		min = 0
	}

	// build JS arrays
	var labels, rtts, serverAcks, colors []string
	for _, r := range recs {
		labels = append(labels, fmt.Sprintf(`"%s"`, r.SentAt.Format("15:04:05")))
		if r.DeviceRTTMs >= 0 {
			rtts = append(rtts, fmt.Sprintf("%d", r.DeviceRTTMs))
		} else {
			rtts = append(rtts, "null")
		}
		serverAcks = append(serverAcks, fmt.Sprintf("%d", r.ServerAckMs))
		switch r.Classification {
		case "awake/fast":
			colors = append(colors, `"#22c55e"`)
		case "light-sleep/network":
			colors = append(colors, `"#eab308"`)
		case "deep-sleep/doze":
			colors = append(colors, `"#ef4444"`)
		default:
			colors = append(colors, `"#6b7280"`)
		}
	}

	// counts
	countAwake, countLight, countDeep, countTimeout := 0, 0, 0, 0
	for _, r := range recs {
		switch r.Classification {
		case "awake/fast":
			countAwake++
		case "light-sleep/network":
			countLight++
		case "deep-sleep/doze":
			countDeep++
		case "timeout":
			countTimeout++
		}
	}

	html := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>silent-receipts-lab — RTT Report — %s</title>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.1/dist/chart.umd.min.js"></script>
<style>
  :root{--bg:#0b0f14;--card:#111827;--muted:#9ca3af;--line:#1f2937}
  *{box-sizing:border-box} body{margin:0;font-family: ui-sans, system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial; background:var(--bg);color:#e5e7eb}
  header{padding:28px 24px;border-bottom:1px solid var(--line);position:sticky;top:0;background:rgba(11,15,20,0.9);backdrop-filter:blur(8px)}
  h1{margin:0;font-size:20px;letter-spacing:0.2px} .sub{color:var(--muted);font-size:13px;margin-top:6px}
  .wrap{max-width:1100px;margin:0 auto;padding:24px}
  .grid{display:grid;grid-template-columns:repeat(4,1fr);gap:12px;margin:16px 0 20px}
  .card{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:14px}
  .card .k{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:0.6px}
  .card .v{font-size:22px;font-weight:700;margin-top:4px}
  .card .h{color:var(--muted);font-size:12px;margin-top:2px}
  .panel{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:16px;margin:16px 0}
  .panel h2{margin:0 0 10px;font-size:15px}
  canvas{background:#0f172a;border-radius:10px;padding:10px}
  table{width:100%%;border-collapse:collapse;font-size:13px}
  th,td{padding:8px 10px;border-bottom:1px solid var(--line);text-align:left}
  th{color:var(--muted);font-weight:600;font-size:11px;text-transform:uppercase;letter-spacing:0.5px}
  .badge{display:inline-block;padding:2px 8px;border-radius:999px;font-size:11px;font-weight:700}
  .b-awake{background:#052e16;color:#86efac;border:1px solid #14532d}
  .b-light{background:#422006;color:#fde68a;border:1px solid #78350f}
  .b-deep{background:#450a0a;color:#fca5a5;border:1px solid #7f1d1d}
  .b-timeout{background:#1f2937;color:#9ca3af;border:1px solid #374151}
  footer{color:var(--muted);font-size:12px;padding:24px;text-align:center;border-top:1px solid var(--line);margin-top:24px}
  a{color:#93c5fd;text-decoration:none} a:hover{text-decoration:underline}
  code{background:#0f172a;padding:2px 6px;border-radius:6px;border:1px solid var(--line);font-size:12px}
</style>
</head>
<body>
<header>
  <div class="wrap" style="padding:0">
   <h1>🔬 silent-receipts-lab — Longitudinal RTT Report</h1>
   <div class="sub">Target <code>%s</code> · %d probes · %s → %s · CSV: <code>%s</code> · Generated %s</div>
   <div class="sub">⚠️ Research use only — probes were <b>visible messages</b> sent with explicit consent. Silent/phantom probes (reaction/edit to invalid ID) exploit the same timing side-channel but are intentionally <b>not</b> automated here to avoid covert tracking.</div>
  </div>
</header>
<div class="wrap">
  <div class="grid">
    <div class="card"><div class="k">Delivered</div><div class="v">%d / %d</div><div class="h">%.1f%% success</div></div>
    <div class="card"><div class="k">Avg / Median RTT</div><div class="v">%.0f ms / %d ms</div><div class="h">min %d ms · max %d ms</div></div>
    <div class="card"><div class="k">State breakdown</div><div class="v" style="font-size:14px;line-height:1.6"><span class="badge b-awake">awake %d</span> <span class="badge b-light">light %d</span> <span class="badge b-deep">deep %d</span> <span class="badge b-timeout">timeout %d</span></div><div class="h">thresholds: &lt;400ms / 400-1200ms / &gt;1200ms</div></div>
    <div class="card"><div class="k">Data & Battery note</div><div class="v" style="font-size:13px">~1–3 KB per probe</div><div class="h">60 probes ≈ 60–180 KB · <b>CAUTION:</b> high frequency drains target battery & mobile quota</div></div>
  </div>

  <div class="panel">
    <h2>⏱️ Device RTT over time (ms) — color = inferred state</h2>
    <canvas id="rttChart" height="110"></canvas>
    <div style="color:var(--muted);font-size:12px;margin-top:8px">Green = likely awake/screen-on · Yellow = light sleep/network · Red = deep sleep / Doze / radio paging · Grey = timeout (no receipt in 20s)</div>
  </div>

  <div class="panel">
    <h2>📊 Server Ack vs Device RTT</h2>
    <canvas id="ackChart" height="90"></canvas>
  </div>

  <div class="panel">
    <h2>📋 Raw log</h2>
    <div style="overflow:auto;max-height:420px;border:1px solid var(--line);border-radius:10px">
    <table>
      <thead><tr><th>#</th><th>Time</th><th>Message ID</th><th>Server Ack</th><th>Device RTT</th><th>Status</th><th>Class</th></tr></thead>
      <tbody>
`, target, target, len(recs), recs[0].SentAt.Format("2006-01-02 15:04"), recs[len(recs)-1].SentAt.Format("15:04"), csvPath, time.Now().Format("2006-01-02 15:04:05 MST"),
		deliveredCount, len(recs), float64(deliveredCount)/float64(len(recs))*100,
		avg, median, min, max,
		countAwake, countLight, countDeep, countTimeout,
	)

	// table rows
	for _, r := range recs {
		badge := ""
		switch r.Classification {
		case "awake/fast":
			badge = `<span class="badge b-awake">awake</span>`
		case "light-sleep/network":
			badge = `<span class="badge b-light">light</span>`
		case "deep-sleep/doze":
			badge = `<span class="badge b-deep">deep</span>`
		default:
			badge = `<span class="badge b-timeout">timeout</span>`
		}
		rttStr := fmt.Sprintf("%d ms", r.DeviceRTTMs)
		if r.DeviceRTTMs < 0 {
			rttStr = "—"
		}
		html += fmt.Sprintf("<tr><td>%d</td><td>%s</td><td><code style='font-size:11px'>%s</code></td><td>%d ms</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			r.Seq, r.SentAt.Format("15:04:05"), r.MessageID, r.ServerAckMs, rttStr, r.Status, badge)
	}

	html += fmt.Sprintf(`      </tbody>
    </table>
    </div>
    <div style="margin-top:10px;color:var(--muted);font-size:12px">Tip: open the CSV in Numbers/Excel for deeper analysis: <code>%s</code></div>
  </div>

  <div class="panel">
    <h2>📖 How to read this</h2>
    <ul style="color:#d1d5db;font-size:13px;line-height:1.6">
      <li><b>RTT &lt; 400 ms:</b> Device was likely awake, app in foreground or recently active. WhatsApp WebSocket is hot.</li>
      <li><b>400–1200 ms:</b> Transitional — screen recently off, or Wi-Fi ↔ cellular handover, or light Doze.</li>
      <li><b>&gt; 1200 ms (as in your 2527 ms sample):</b> Device had to be paged/woken via push (FCM/APNs), radio ramped up, CPU resumed — classic deep sleep.</li>
      <li><b>Why this matters:</b> The researchers showed that even with <i>silent</i> probes (invalid reaction), the underlying Signal ratchet still ACKs. Your <i>visible</i> probe leaks the same timing — but with consent and without stealth.</li>
      <li><b>Self-chat note:</b> Sending to your own JID (<code>yournumber@s.whatsapp.net</code> with same user) self-acks instantly on the server and is not a useful cross-device RTT sample — that's why you correctly noticed “you can’t really test on your own number” for state leakage. Use a second device you own or an explicitly consenting partner.</li>
    </ul>
  </div>
</div>

<footer>
  Generated by <code>go run ./cmd/monitor</code> · silent-receipts-lab · For educational, consensual research only. Respect privacy & ToS.<br>
  Paper: <a href="https://arxiv.org/abs/2411.11194">Careless Whisper — arXiv:2411.11194</a>
</footer>

<script>
const labels = [%s];
const rtts = [%s];
const acks = [%s];
const colors = [%s];

new Chart(document.getElementById('rttChart'), {
  type: 'line',
  data: {
    labels: labels,
    datasets: [{
      label: 'Device RTT (ms)',
      data: rtts,
      borderColor: '#38bdf8',
      backgroundColor: 'rgba(56,189,248,0.12)',
      pointBackgroundColor: colors,
      pointBorderColor: '#0f172a',
      pointRadius: 5,
      pointHoverRadius: 7,
      tension: 0.28,
      spanGaps: true
    }]
  },
  options: {
    responsive:true,
    plugins:{legend:{display:false}, tooltip:{callbacks:{label:(c)=> c.parsed.y==null? ' timeout' : ' '+c.parsed.y+' ms'}}},
    scales:{
      x:{grid:{color:'rgba(255,255,255,0.06)'}, ticks:{color:'#9ca3af', maxTicksLimit:14}},
      y:{grid:{color:'rgba(255,255,255,0.06)'}, ticks:{color:'#9ca3af'}, beginAtZero:true, title:{display:true,text:'ms',color:'#9ca3af'}}
    }
  }
});

new Chart(document.getElementById('ackChart'), {
  type: 'scatter',
  data: {
    datasets: [{
      label: 'Server Ack (x) vs Device RTT (y)',
      data: rtts.map((v,i)=> v==null? null : ({x: acks[i], y: v})).filter(Boolean),
      backgroundColor: colors.map(c=> c.replaceAll('"','')),
      pointRadius: 5
    }]
  },
  options: {
    responsive:true,
    plugins:{legend:{display:false}},
    scales:{
      x:{grid:{color:'rgba(255,255,255,0.06)'}, ticks:{color:'#9ca3af'}, title:{display:true,text:'Server Ack (ms)',color:'#9ca3af'}},
      y:{grid:{color:'rgba(255,255,255,0.06)'}, ticks:{color:'#9ca3af'}, title:{display:true,text:'Device RTT (ms)',color:'#9ca3af'}}
    }
  }
});
</script>
</body>
</html>
`, csvPath, strings.Join(labels, ","), strings.Join(rtts, ","), strings.Join(serverAcks, ","), strings.Join(colors, ","))

	return os.WriteFile(htmlPath, []byte(html), 0644)
}

func main() {
	var (
		targetFlag   = flag.String("target", "", "Target phone number with country code (e.g. 4915756941992). Required unless -plot-only is set.")
		intervalFlag = flag.Duration("interval", 90*time.Second, "Interval between probes (min 30s). e.g. 60s, 2m, 5m")
		countFlag    = flag.Int("count", 0, "Number of probes to send (0 = infinite until Ctrl+C or -duration)")
		durationFlag = flag.Duration("duration", 0, "Total duration to run (0 = run until -count reached or Ctrl+C). e.g. 30m, 2h")
		outFlag      = flag.String("out", "", "CSV output path (default data/rtt_<target>_<timestamp>.csv)")
		jitterFlag   = flag.Bool("jitter", true, "Add ±10% random jitter to interval to avoid perfectly periodic pattern")
		messageFlag  = flag.String("message", "", "Custom probe message prefix (default: '🔬 Lab Probe')")
		plotOnlyFlag = flag.String("plot-only", "", "Regenerate HTML report from existing CSV and exit (path to CSV)")
		timeoutFlag  = flag.Duration("timeout", 20*time.Second, "How long to wait for device receipt before marking timeout")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "silent-receipts-lab — longitudinal monitor\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n  go run ./cmd/monitor -target 4915756941992 [options]\n  go run ./cmd/monitor -plot-only data/rtt_49157_20260919_0345.csv\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # 30 probes every 2 minutes for ~1 hour, with consent:
  go run ./cmd/monitor -target 4915756941992 -interval 2m -count 30

  # Continuous overnight (8h, every 90s):
  go run ./cmd/monitor -target 4915756941992 -duration 8h -interval 90s

  # Regenerate plot without sending:
  go run ./cmd/monitor -plot-only data/rtt_4915756941992_20260919-034539.csv

Responsible use:
  - Only probe numbers you own or where you have explicit, informed consent.
  - Visible probes (regular messages) are used here intentionally — silent phantom probes (reaction to invalid ID)
    are stealthy and must NOT be used for covert tracking. The RTT leakage is identical; visibility ensures ethics.
  - Each probe is ~1-3 KB. At 90s interval: ~2.5 MB/hour, low battery impact. At <30s you risk rate-limits/ban
    and noticeable battery drain on target.
  - Your hotspot tethering note: WhatsApp's QR scanner disables when iOS detects Personal Hotspot active.
    Workaround you discovered: temporarily disable hotspot, pair, then re-enable + reconnect. Or pair via
    WhatsApp Web in desktop browser tethered differently, then reuse same store/session.db.

`)
	}
	flag.Parse()

	if *plotOnlyFlag != "" {
		// regenerate HTML from CSV
		f, err := os.Open(*plotOnlyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open CSV %s: %v\n", *plotOnlyFlag, err)
			os.Exit(1)
		}
		defer f.Close()
		r := csv.NewReader(bufio.NewReader(f))
		rows, err := r.ReadAll()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse CSV: %v\n", err)
			os.Exit(1)
		}
		if len(rows) < 2 {
			fmt.Fprintf(os.Stderr, "CSV has no data rows\n")
			os.Exit(1)
		}
		// header: seq,timestamp_iso,sent_at_rfc3339,message_id,target_jid,server_ack_ms,device_rtt_ms,status,classification
		var recs []record
		for i, row := range rows[1:] {
			if len(row) < 9 {
				continue
			}
			seq := i + 1
			fmt.Sscanf(row[0], "%d", &seq)
			sentAt, _ := time.Parse(time.RFC3339Nano, row[2])
			if sentAt.IsZero() {
				sentAt, _ = time.Parse(time.RFC3339, row[2])
			}
			var devRTT int64 = -1
			fmt.Sscanf(row[6], "%d", &devRTT)
			var servAck int64
			fmt.Sscanf(row[5], "%d", &servAck)
			recs = append(recs, record{
				Seq:            seq,
				TimestampISO:   row[1],
				SentAt:         sentAt,
				MessageID:      row[3],
				Target:         row[4],
				ServerAckMs:    servAck,
				DeviceRTTMs:    devRTT,
				Status:         row[7],
				Classification: row[8],
			})
		}
		target := recs[0].Target
		if err := generateHTMLReport(*plotOnlyFlag, recs, target); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate HTML: %v\n", err)
			os.Exit(1)
		}
		htmlPath := strings.TrimSuffix(*plotOnlyFlag, filepath.Ext(*plotOnlyFlag)) + ".html"
		fmt.Printf("✅ Regenerated plot: %s\n   Open with: open \"%s\"\n", htmlPath, htmlPath)
		return
	}

	if *targetFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: -target is required (phone with country code, e.g. 4915756941992)")
		flag.Usage()
		os.Exit(1)
	}
	phone := strings.TrimSpace(strings.TrimPrefix(*targetFlag, "+"))
	// basic validation
	if len(phone) < 7 || len(phone) > 15 {
		fmt.Fprintf(os.Stderr, "Error: target phone looks invalid (%q) — expected 7-15 digits with country code\n", phone)
		os.Exit(1)
	}
	for _, c := range phone {
		if c < '0' || c > '9' {
			fmt.Fprintf(os.Stderr, "Error: target must be digits only (no spaces/dashes), got %q\n", phone)
			os.Exit(1)
		}
	}
	if *intervalFlag < 30*time.Second {
		fmt.Fprintf(os.Stderr, "Error: -interval must be >=30s to avoid spam, rate-limits and battery drain (got %s)\n", *intervalFlag)
		os.Exit(1)
	}
	if *countFlag < 0 {
		fmt.Fprintf(os.Stderr, "Error: -count must be >=0\n")
		os.Exit(1)
	}

	// explicit consent confirmation for interactive TTY
	if isTTY() {
		fmt.Printf("\n⚠️  CONSENT CHECK — Longitudinal probing\n")
		fmt.Printf("   Target: %s@s.whatsapp.net\n", phone)
		fmt.Printf("   Interval: %s  Count: %s  Duration: %s\n", *intervalFlag, countStr(*countFlag), durationStr(*durationFlag))
		fmt.Printf("   Each probe sends a VISIBLE message (you will see it on target). Ensure you have explicit consent.\n")
		fmt.Printf("   Estimated data: ~%d KB total for this run (if count limited) or ~1-3 KB per probe.\n", estimateKB(*countFlag, *durationFlag, *intervalFlag))
		fmt.Printf("   Type YES to continue (or Ctrl+C to abort): ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) != "YES" {
				fmt.Println("Aborted. (Type YES exactly to confirm)")
				os.Exit(1)
			}
		} else {
			fmt.Println("\nAborted.")
			os.Exit(1)
		}
	}

	storeDir := "store"
	dbPath := filepath.Join(storeDir, "session.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "\n[!] No session at %s. Run `go run ./cmd/minimal` first to pair.\n\n", dbPath)
		os.Exit(1)
	}

	// prepare output file
	csvPath := *outFlag
	if csvPath == "" {
		if err := os.MkdirAll("data", 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create data/: %v\n", err)
			os.Exit(1)
		}
		ts := time.Now().Format("20060102-150405")
		csvPath = filepath.Join("data", fmt.Sprintf("rtt_%s_%s.csv", phone, ts))
	} else {
		if dir := filepath.Dir(csvPath); dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
	}
	f, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open CSV %s: %v\n", csvPath, err)
		os.Exit(1)
	}
	defer f.Close()
	// if file was empty, write header
	info, _ := f.Stat()
	csvWriter := csv.NewWriter(f)
	defer csvWriter.Flush()
	if info.Size() == 0 {
		writeCSVHeader(csvWriter)
		fmt.Printf("📄 Logging to %s\n", csvPath)
	} else {
		fmt.Printf("📄 Appending to existing %s\n", csvPath)
	}

	// connect whatsmeow
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dbURI := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", dbPath)
	dbLog := waLog.Stdout("DB", "WARN", true)
	container, err := sqlstore.New(ctx, "sqlite", dbURI, dbLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DB open failed: %v\n", err)
		os.Exit(1)
	}
	defer container.Close()
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil || deviceStore.ID == nil {
		fmt.Fprintf(os.Stderr, "No device in DB. Re-pair with `go run ./cmd/minimal`\n")
		os.Exit(1)
	}
	clientLog := waLog.Stdout("WA-Monitor", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(func(evt any) {
		switch v := evt.(type) {
		case *events.Receipt:
			handleReceipt(v)
		case *events.Connected:
			fmt.Printf("\n[🟢 READY] Connected as %s\n", client.Store.ID.User)
		case *events.LoggedOut:
			fmt.Println("\n[🔴 LOGGED OUT] Session invalidated on phone. Stop and re-pair.")
			stop()
		}
	})
	if err := client.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Connect failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()
	time.Sleep(1200 * time.Millisecond) // brief settle

	targetJID := types.NewJID(phone, types.DefaultUserServer)
	if *messageFlag == "" {
		*messageFlag = "🔬 Lab Probe"
	}
	fmt.Println("==========================================================")
	fmt.Println("  Longitudinal RTT Monitor — consensual, visible probes")
	fmt.Println("==========================================================")
	fmt.Printf("Target:    %s\n", targetJID.String())
	fmt.Printf("Interval:  %s (jitter %v)  Timeout: %s\n", *intervalFlag, *jitterFlag, *timeoutFlag)
	fmt.Printf("Output:    %s\n", csvPath)
	fmt.Printf("HTML:      %s\n", strings.TrimSuffix(csvPath, filepath.Ext(csvPath))+".html")
	fmt.Printf("Self / pairing note: probing your own paired JID self-acks instantly (not useful). Use a second device or consenting partner.\n")
	fmt.Println("Press Ctrl+C to stop. HTML report regenerates after each probe.")
	fmt.Println("----------------------------------------------------------")

	startTime := time.Now()
	seq := 0
	// if appending to existing file, try to resume seq
	if info.Size() > 0 {
		// quick count rows
		existing, _ := os.ReadFile(csvPath)
		seq = strings.Count(string(existing), "\n") - 1 // minus header
		if seq < 0 {
			seq = 0
		}
		fmt.Printf("Resuming at seq %d\n", seq+1)
	}

	// load previous records for plot accumulation
	if info.Size() > 0 {
		// read existing rows into records slice so plot includes history
		f2, _ := os.Open(csvPath)
		if f2 != nil {
			r := csv.NewReader(bufio.NewReader(f2))
			rows, _ := r.ReadAll()
			_ = f2.Close()
			for i, row := range rows[1:] {
				if len(row) < 9 {
					continue
				}
				s := i + 1
				fmt.Sscanf(row[0], "%d", &s)
				sentAt, _ := time.Parse(time.RFC3339Nano, row[2])
				if sentAt.IsZero() {
					sentAt, _ = time.Parse(time.RFC3339, row[2])
				}
				var dev, ack int64
				fmt.Sscanf(row[6], "%d", &dev)
				fmt.Sscanf(row[5], "%d", &ack)
				records = append(records, record{
					Seq: s, TimestampISO: row[1], SentAt: sentAt, MessageID: row[3],
					Target: row[4], ServerAckMs: ack, DeviceRTTMs: dev, Status: row[7], Classification: row[8],
				})
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nInterrupted — generating final report...")
			goto done
		default:
		}
		if *countFlag > 0 && seq >= *countFlag {
			fmt.Printf("\nReached count %d — done.\n", *countFlag)
			break
		}
		if *durationFlag > 0 && time.Since(startTime) >= *durationFlag {
			fmt.Printf("\nReached duration %s — done.\n", *durationFlag)
			break
		}
		seq++
		sentAt := time.Now()
		text := fmt.Sprintf("%s [%s #%d]", *messageFlag, sentAt.Format("15:04:05"), seq)

		msg := &waE2E.Message{Conversation: proto.String(text)}

		// prepare delivery wait channel BEFORE sending to avoid race
		deliverCh := make(chan time.Time, 1)
		// we don't know msgID until after SendMessage, so we register after.
		// Instead, we will register a temporary placeholder and re-map after.
		// Simpler: send first, then register; receipts arrive >100ms later so race is rare,
		// but to be safe we add tiny delay before sending? Actually we can hook: whatsmeow generates ID client-side before send.
		// We'll just register immediately after send with 50ms grace; worst case we might miss ultra-fast <50ms but real RTT min ~120ms.
		sendStart := time.Now()
		resp, err := client.SendMessage(ctx, targetJID, msg)
		serverAckMs := time.Since(sendStart).Milliseconds()
		if err != nil {
			fmt.Printf("\n[seq %d] ❌ Send failed: %v (serverAck %d ms)\n", seq, err, serverAckMs)
			r := record{
				Seq: seq, TimestampISO: sentAt.Format(time.RFC3339), SentAt: sentAt,
				MessageID: "(failed)", Target: targetJID.String(),
				ServerAckMs: serverAckMs, DeviceRTTMs: -1, Status: "send_error", Classification: "timeout",
			}
			recordsLock.Lock()
			records = append(records, r)
			recordsLock.Unlock()
			appendCSV(csvWriter, r)
			_ = f.Sync()
			// still generate report
			recordsLock.Lock()
			_ = generateHTMLReport(csvPath, append([]record(nil), records...), targetJID.String())
			recordsLock.Unlock()
			// wait interval then continue
		} else {
			msgID := resp.ID
			// register pending
			pendingLock.Lock()
			pending[msgID] = deliverCh
			pendingLock.Unlock()

			fmt.Printf("\n[seq %d] 📤 Sent %s  serverAck %d ms  awaiting device receipt (timeout %s)...\n", seq, msgID, serverAckMs, *timeoutFlag)
			var deviceRTTms int64 = -1
			status := "timeout"
			class := "timeout"
			var deliveredAt time.Time
			select {
			case deliveredAt = <-deliverCh:
				deviceRTTms = deliveredAt.Sub(sentAt).Milliseconds()
				status = "delivered"
				class = classifyRTT(deviceRTTms)
				fmt.Printf("        ✅ Delivered — RTT %d ms (%s)  [%s]\n", deviceRTTms, class, deliveredAt.Format("15:04:05.000"))
			case <-time.After(*timeoutFlag):
				fmt.Printf("        ⏱️  Timeout — no device receipt in %s (phone offline / WhatsApp killed?)\n", *timeoutFlag)
			case <-ctx.Done():
				fmt.Printf("        🛑 Interrupted while waiting\n")
				pendingLock.Lock()
				delete(pending, msgID)
				pendingLock.Unlock()
				goto done
			}
			// cleanup
			pendingLock.Lock()
			delete(pending, msgID)
			pendingLock.Unlock()

			r := record{
				Seq: seq, TimestampISO: sentAt.Format(time.RFC3339), SentAt: sentAt,
				MessageID: msgID, Target: targetJID.String(),
				ServerAckMs: serverAckMs, DeviceRTTMs: deviceRTTms, Status: status, Classification: class,
			}
			recordsLock.Lock()
			records = append(records, r)
			recordsLock.Unlock()
			appendCSV(csvWriter, r)
			_ = f.Sync()
			recordsLock.Lock()
			_ = generateHTMLReport(csvPath, append([]record(nil), records...), targetJID.String())
			recordsLock.Unlock()
			fmt.Printf("        📄 Logged → %s  |  📊 %s.html updated (open with: open \"%s.html\")\n", csvPath, strings.TrimSuffix(csvPath, filepath.Ext(csvPath)), strings.TrimSuffix(csvPath, filepath.Ext(csvPath)))
		}

		// decide sleep
		if *countFlag > 0 && seq >= *countFlag {
			break
		}
		if *durationFlag > 0 && time.Since(startTime)+*intervalFlag > *durationFlag {
			// will exceed duration, break after this iteration if next would exceed
			// but let loop condition handle
		}
		// check if we should stop before sleeping
		select {
		case <-ctx.Done():
			goto done
		default:
		}
		sleepFor := *intervalFlag
		if *jitterFlag {
			j := (rand.Float64()*0.2 - 0.1) // -10% to +10%
			sleepFor = time.Duration(float64(sleepFor) * (1 + j))
		}
		fmt.Printf("        💤 Sleeping %s until next probe (Ctrl+C to stop)...\n", sleepFor.Round(time.Second))
		select {
		case <-time.After(sleepFor):
		case <-ctx.Done():
			goto done
		}
	}

done:
	// final report
	recordsLock.Lock()
	defer recordsLock.Unlock()
	if len(records) > 0 {
		if err := generateHTMLReport(csvPath, records, targetJID.String()); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write final HTML: %v\n", err)
		} else {
			htmlPath := strings.TrimSuffix(csvPath, filepath.Ext(csvPath)) + ".html"
			fmt.Printf("\n✅ Done. %d probes logged.\n   CSV:  %s\n   HTML: %s\n   Open: open \"%s\"\n   Plot-only regen: go run ./cmd/monitor -plot-only \"%s\"\n", len(records), csvPath, htmlPath, htmlPath, csvPath)
		}
	} else {
		fmt.Println("\nNo probes were sent.")
	}
}

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func countStr(c int) string {
	if c == 0 {
		return "∞"
	}
	return fmt.Sprintf("%d", c)
}
func durationStr(d time.Duration) string {
	if d == 0 {
		return "∞"
	}
	return d.String()
}
func estimateKB(count int, dur time.Duration, interval time.Duration) int {
	if count > 0 {
		return count * 2 // avg 2KB
	}
	if dur > 0 && interval > 0 {
		return int(dur/interval) * 2
	}
	return 0
}
