# Troubleshooting & FAQ

Common questions, error messages, and solutions when running `silent-receipts-lab` on macOS with OCLP.

---

## 🛑 Common Errors & Solutions

### 1. `zsh: command not found: go`
**Cause:** `/usr/local/go/bin` is not yet loaded in your active shell's `$PATH`.  
**Solution:**
```bash
# Add to ~/.zprofile
echo 'export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"' >> ~/.zprofile

# Reload current terminal session
source ~/.zprofile
```
If you are using VS Code, restart VS Code completely after running the above command so its integrated terminal inherits the new PATH.

---

### 2. `panic: failed to upgrade database: foreign keys are not enabled`
**Cause:** WhatsApp's multi-device database schema relies on relational integrity between device keys, identities, and sessions. SQLite has foreign keys disabled by default. Different SQLite drivers require different connection string syntax.  
**Solution:**
In `cmd/minimal/main.go` and `cmd/receipts/main.go`, we explicitly configure the pure-Go `modernc.org/sqlite` connection string as:
```go
dbURI := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", dbPath)
```
Ensure you keep `_pragma=foreign_keys(1)` in the connection URI.

---

### 3. `database is locked (5) (SQLITE_BUSY)`
**Cause:** SQLite allows multiple readers, but only one active writer. If you run both `cmd/minimal` and `cmd/receipts` simultaneously, or if a previous run didn't exit cleanly, one process may lock `store/session.db`.  
**Solution:**
1. Stop any running lab process using `Ctrl+C`.
2. Check if a rogue process is running:
   ```bash
   ps aux | grep "cmd/minimal\|cmd/receipts"
   ```
3. If necessary, terminate it:
   ```bash
   pkill -f "cmd/minimal"
   ```

---

### 4. The QR Code in Terminal Looks Scrambled or Unreadable
**Cause:** The terminal window is too narrow, line wrapping is interfering with the QR block characters, or line height is stretched.  
**Solution:**
1. Make your Terminal window wider (at least 80–100 columns wide).
2. Use standard macOS Terminal fonts (such as **Menlo**, **Monaco**, or **SF Mono**).
3. If using VS Code's integrated terminal, you can zoom out slightly (`Cmd + -`) so the QR code fits on screen without wrapping lines.

---

### 5. `[🔴 STATUS] Device was logged out / unlinked from mobile phone`
**Cause:** WhatsApp security automatically unlinks companion devices if:
- The device was manually unlinked in the mobile WhatsApp app.
- The device remained inactive for more than 14 consecutive days.
- The primary phone account was re-registered.

**Solution:**
Reset your local session database and re-pair:
```bash
rm -rf store/session.db store/session.db-*
go run ./cmd/minimal
```

---

### 6. CGO / Xcode CLT Prompts Appearing on macOS
**Cause:** If any Go dependency accidentally invokes CGO, macOS might pop up a prompt asking to install Command Line Tools.  
**Solution:**
Force pure-Go compilation by setting `CGO_ENABLED=0`:
```bash
CGO_ENABLED=0 go run ./cmd/minimal
```
*(All commands in our `Makefile` automatically set `CGO_ENABLED=0` for you).*

---

## ❓ Frequently Asked Questions (FAQ)

### Can I run this without keeping the terminal window open permanently?
Yes. Once paired, you can compile a standalone binary:
```bash
make build
# Creates bin/minimal and bin/receipts
```
You can run it in a `screen` or `tmux` session, or run it directly whenever you want to test.

### Does this violate WhatsApp's Terms of Service?
`whatsmeow` connects as a standard multi-device companion client (the same protocol WhatsApp Web uses). However, sending automated messages or high-frequency probes to numbers you do not own can trigger automated spam detection. For research and learning, always probe only your own devices.

### Can I run this on my native Big Sur partition?
Yes! However, newer Go releases and dependencies are best maintained on Ventura or Sonoma. Big Sur is best used as a reliable fallback partition if you need to download packages or recover your system.
