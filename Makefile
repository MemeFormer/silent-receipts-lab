# Makefile for silent-receipts-lab
# Optimized for macOS (OCLP Ventura/Sonoma Intel x86_64)

.PHONY: help check run lab build clean tidy

help:
	@echo "silent-receipts-lab - Available Commands:"
	@echo "  make check   - Run environment & toolchain preflight check"
	@echo "  make run     - Run minimal client (for initial pairing & basic listening)"
	@echo "  make lab     - Run delivery receipt timing & analysis lab"
	@echo "  make build   - Compile standalone binaries with CGO_ENABLED=0 into bin/"
	@echo "  make tidy    - Download and synchronize Go module dependencies"
	@echo "  make clean   - Remove built binaries"

check:
	@chmod +x scripts/check-env.sh
	@./scripts/check-env.sh

run:
	CGO_ENABLED=0 go run ./cmd/minimal

lab:
	CGO_ENABLED=0 go run ./cmd/receipts

tidy:
	go mod tidy

build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -o bin/minimal ./cmd/minimal
	CGO_ENABLED=0 go build -o bin/receipts ./cmd/receipts
	@echo "Built binaries in bin/: minimal, receipts"

clean:
	rm -rf bin/
