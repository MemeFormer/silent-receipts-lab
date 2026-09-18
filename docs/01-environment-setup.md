# Part 1: Environment & Toolchain Setup Guide

This guide is tailored specifically for setting up a clean, modern Go development environment on your **Intel Mac (Model MacBookPro11,1, `x86_64`)** running **OpenCore Legacy Patcher (OCLP) macOS Ventura or Sonoma**.

---

## 🎯 Architectural Decisions & Philosophy

Before typing any commands, here is why this setup is configured this way:

1. **Target OS: OCLP Ventura (macOS 13) or Sonoma (macOS 14)**
   - Modern Go toolchains and modern cryptographic libraries expect macOS 12+ APIs.
   - Native Big Sur (macOS 11) is kept intact on your disk as a reliable fallback and download/transfer station.
   - Sequoia (macOS 15) on your external USB is known to have stability issues on MacBookPro11,1 hardware — avoid using it for dev work.
2. **No Homebrew (`brew`)**
   - Homebrew often triggers compilation from source when bottle binaries are mismatched or when running under OCLP patched systems.
   - Homebrew forces Xcode Command Line Tools (`xcode-select --install`) and pulls hundreds of megabytes of unnecessary dependencies.
   - An official manual install is self-contained in `/usr/local/go` and can be upgraded or removed by simply replacing one folder.
3. **No Apple Command Line Tools (Xcode CLT / CGO) Required**
   - On OCLP patched Macs, Apple's `xcode-select --install` often hangs, fails certificate checks, or refuses to download from Apple Software Update.
   - Standard SQLite wrappers in Go (like `mattn/go-sqlite3`) require a C compiler (`gcc`/`clang`) via CGO.
   - **Our Solution:** We use `modernc.org/sqlite` — a pure-Go SQLite driver. It compiles with `CGO_ENABLED=0`. You do **not** need Xcode, Apple CLT, or any C compiler installed to build and run this lab!
4. **Target Architecture: Intel (`darwin-amd64`)**
   - Your Mac processor is Intel Core i5/i7 (`x86_64`).
   - Always choose the **`darwin-amd64`** installer, **NOT** `darwin-arm64` (which is for M1/M2/M3/M4 Apple Silicon).

---

## 📋 Step-by-Step Installation Walkthrough

### Step 1: Boot into OCLP Ventura or Sonoma
Boot your MacBook Pro into your OCLP Ventura or Sonoma volume. Verify your architecture and OS in Terminal:

```bash
uname -m
# Expected output: x86_64

sw_vers
# Expected output: ProductVersion 13.x or 14.x
```

---

### Step 2: Download the Official Go Package
1. Open Safari (or your preferred browser) and visit:  
   👉 **[https://go.dev/dl/](https://go.dev/dl/)**
2. Look for the **Apple macOS** section.
3. Download the **Intel (x86-64)** package:
   - Filename pattern: `go1.24.x.darwin-amd64.pkg` (or the latest stable `darwin-amd64.pkg`).
   - **Do not** download the `arm64.pkg`!

> 💡 **Offline / Big Sur Bridge Tip:** If your OCLP Ventura/Sonoma browser ever has Wi-Fi or download glitches, you can boot into native Big Sur, download `go*.darwin-amd64.pkg` directly to your internal drive or a USB thumb drive, reboot into Ventura/Sonoma, and run the installer.

---

### Step 3: Run the Official Go Installer
Double-click the downloaded `.pkg` file and follow the standard macOS installation wizard.
- The installer places the Go distribution into `/usr/local/go`.
- This is completely self-contained. It does not tamper with macOS system binaries.

---

### Step 4: Configure Your Shell PATH (`zprofile` & `zshrc`)
macOS uses `zsh` as its default shell.

When you install Go, its binaries reside in `/usr/local/go/bin`. To make the `go` command available everywhere (in Terminal, iTerm2, and VS Code's integrated terminal), add it to both `~/.zprofile` (read during login) and `~/.zshrc` (read for interactive shells).

Run the following commands in Terminal:

```bash
# Add Go to ~/.zprofile (for login shells)
echo 'export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"' >> ~/.zprofile

# Add Go to ~/.zshrc (for interactive sub-shells)
echo 'export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"' >> ~/.zshrc

# Reload your current terminal session immediately
source ~/.zprofile
```

---

### Step 5: Verify Go and Toolchain Health
Check that Go is detected and verify its version:

```bash
which go
# Expected output: /usr/local/go/bin/go

go version
# Expected output: go version go1.24.x darwin/amd64 (or newer)
```

Now run the built-in lab environment diagnostic check from inside this repository:

```bash
cd ~/silent-receipts-lab
./scripts/check-env.sh
```

You should see green checkmarks for:
- [✓ PASS] Operating System & CPU Architecture (`x86_64`)
- [✓ PASS] Go binary found
- [✓ PASS] `/usr/local/go/bin` in PATH
- [✓ PASS] Pure-Go SQLite configuration
- [✓ PASS] `.gitignore` session protection

---

### Step 6: Fetch Dependencies & Verify Build
Run:

```bash
cd ~/silent-receipts-lab
go mod tidy
```

Now verify that you can build the minimal client without CGO:

```bash
CGO_ENABLED=0 go build -o /dev/null ./cmd/minimal
```

If that command completes silently with no errors, congratulations! Your Intel Mac OCLP environment is completely configured and ready for **Part 2**.

---

## 🛡️ Fallback Reference (Only If Needed)

### What if Apple Command Line Tools (CLT) is ever required?
If you ever want to install CLT without using `xcode-select --install` (which often breaks on OCLP):
1. Go to **[https://developer.apple.com/download/all/](https://developer.apple.com/download/all/)**
2. Sign in with your free Apple ID.
3. Search for: `Command Line Tools for Xcode 15` (for Ventura/Sonoma).
4. Download the `.dmg` directly.
5. Mount the `.dmg`, run the installer package. It installs directly without network downloads.

### What about the Toshiba Omarchy Linux machine?
If hardware issues arise on your Mac, your Toshiba Linux machine provides an excellent fallback:
- Clone this repository.
- Install Go via your Linux package manager or official tarball.
- All code in this repository (`cmd/minimal`, `cmd/receipts`) works identically across macOS and Linux because it uses pure Go and standards-compliant SQLite.
