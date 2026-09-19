#!/usr/bin/env bash
# Generate a synthetic RTT log + HTML report without needing WhatsApp connection
# Useful to preview the visualization immediately.
set -e
CSV="data/demo_rtt_$(date +%Y%m%d-%H%M%S).csv"
mkdir -p data

cat > "$CSV" << 'CSVHEAD'
seq,timestamp_iso,sent_at_rfc3339,message_id,target_jid,server_ack_ms,device_rtt_ms,status,classification
CSVHEAD

# Synthetic sequence mimicking real behavior:
# 1: your 2527 ms deep-sleep sample, 2: awake, 3: light, 4: timeout, etc.
now=$(date -u +%Y-%m-%dT%H:%M:%S+00:00)
# helper to append
append() {
  seq=$1; dev=$2; ack=$3; status=$4; class=$5
  ts=$(date -u -v+${seq}M +%Y-%m-%dT%H:%M:%S+00:00 2>/dev/null || date -u -d "+${seq} minutes" +%Y-%m-%dT%H:%M:%S+00:00)
  # fallback for linux: use date -d
  if ! date -u -v+${seq}M +%s >/dev/null 2>&1; then
    ts=$(date -u -d "+${seq} minutes" +%Y-%m-%dT%H:%M:%S+00:00)
  fi
  echo "${seq},${ts},${ts},DEMO${seq}FAKEID,4915756941992@s.whatsapp.net,${ack},${dev},${status},${class}" >> "$CSV"
}

append 1 2527 887 delivered deep-sleep/doze
append 2 184 412 delivered awake/fast
append 3 892 523 delivered light-sleep/network
append 4 -1 450 timeout timeout
append 5 210 390 delivered awake/fast
append 6 3100 900 delivered deep-sleep/doze
append 7 640 410 delivered light-sleep/network
append 8 165 380 delivered awake/fast
append 9 2400 870 delivered deep-sleep/doze
append 10 290 405 delivered awake/fast

echo "📄 Created synthetic CSV: $CSV"
echo "Generating HTML via monitor (no WhatsApp needed)..."

# Try to use Go if available, otherwise generate minimal HTML fallback via python
if command -v go >/dev/null 2>&1; then
  CGO_ENABLED=0 go run ./cmd/monitor -plot-only "$CSV"
else
  echo "Go not found in this shell — using Python fallback to generate HTML..."
  python3 - << PY
import csv, pathlib, datetime
csv_path = pathlib.Path("$CSV")
html_path = csv_path.with_suffix(".html")
# Minimal fallback: just show CSV as table via python
rows = list(csv.DictReader(open(csv_path)))
html = f"""<!doctype html><html><head><meta charset="utf-8"><title>Demo RTT Report</title></head><body><h1>Demo RTT Report (synthetic)</h1><p>CSV: {csv_path}</p><table border=1><tr>{''.join(f'<th>{k}</th>' for k in rows[0].keys())}</tr>"""
for r in rows:
    html += "<tr>" + "".join(f"<td>{v}</td>" for v in r.values()) + "</tr>"
html += "</table><p>For full Chart.js report, run on your Mac: <code>go run ./cmd/monitor -plot-only {csv_path}</code></p></body></html>"
open(html_path,"w").write(html)
print(f"Fallback HTML: {html_path}")
PY
fi

HTML="${CSV%.csv}.html"
echo "✅ Done."
echo "   CSV:  $CSV"
echo "   HTML: $HTML"
echo "   Open: open \"$HTML\"  (macOS)  or  xdg-open \"$HTML\" (Linux)"
