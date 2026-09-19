# Makefile for silent-receipts-lab
# Optimized for macOS (OCLP Ventura/Sonoma Intel x86_64)

.PHONY: help check run lab monitor plot build clean tidy

help:
	@echo "silent-receipts-lab - Available Commands:"
	@echo "  make check   - Run environment & toolchain preflight check"
	@echo "  make run     - Run minimal client (for initial pairing & basic listening)"
	@echo "  make lab     - Run delivery receipt timing & analysis lab (single probe)"
	@echo "  make monitor - Run longitudinal monitor (automated CSV + HTML)"
	@echo "               - usage: make monitor ARGS=\"-target 4915756941992 -interval 90s -count 10\""
	@echo "  make plot    - Regenerate HTML from existing CSV"
	@echo "               - usage: make plot CSV=data/rtt_49157_20260919-034539.csv"
	@echo "  make demo    - Generate synthetic demo CSV + HTML (no WhatsApp needed)"
	@echo "  make build   - Compile standalone binaries with CGO_ENABLED=0 into bin/"
	@echo "  make tidy    - Download and synchronize Go module dependencies"
	@echo "  make clean   - Remove built binaries and generated reports"

check:
	@chmod +x scripts/check-env.sh
	@./scripts/check-env.sh

run:
	CGO_ENABLED=0 go run ./cmd/minimal

lab:
	CGO_ENABLED=0 go run ./cmd/receipts

monitor:
	CGO_ENABLED=0 go run ./cmd/monitor $(ARGS)

plot:
ifndef CSV
	@echo "Usage: make plot CSV=data/rtt_<target>_<ts>.csv"
	@ls -lh data/*.csv 2>/dev/null || echo "No CSV files in data/ yet."
else
	CGO_ENABLED=0 go run ./cmd/monitor -plot-only $(CSV)
	@echo "Open: open \"$(CSV:.csv=.html)\""
endif

demo:
	@chmod +x scripts/demo-plot.sh
	@./scripts/demo-plot.sh

tidy:
	go mod tidy

build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -o bin/minimal ./cmd/minimal
	CGO_ENABLED=0 go build -o bin/receipts ./cmd/receipts
	CGO_ENABLED=0 go build -o bin/monitor ./cmd/monitor
	@echo "Built binaries in bin/: minimal, receipts, monitor"

clean:
	rm -rf bin/
