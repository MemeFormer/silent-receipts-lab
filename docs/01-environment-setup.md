# Environment Setup (Part 1)

Target: OCLP Ventura or Sonoma on Intel Mac (x86_64)

Preferred path
1. Boot into Ventura or Sonoma
2. Install Go manually from https://go.dev/dl (darwin-amd64)
3. Add /usr/local/go/bin to ~/.zprofile
4. Prefer pure-Go SQLite driver first (modernc.org/sqlite) to avoid CGO / Xcode CLT
5. Only install Apple Command Line Tools if really needed

Current status
- [ ] Go installed and go version works
- [ ] Project initialized with go mod init
- [ ] Can import go.mau.fi/whatsmeow
