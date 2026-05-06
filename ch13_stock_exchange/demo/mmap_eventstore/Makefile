.PHONY: build run stop ps clean tidy test sequencer matching marketdata help

BIN := bin
SHM := /dev/shm/exchange_events

help:
	@echo "Targets:"
	@echo "  make build      — compile all three binaries"
	@echo "  make run        — start sequencer + matching + marketdata together (Ctrl-C stops all)"
	@echo "  make stop       — force-kill any running demo processes and remove the shm file"
	@echo "  make ps         — list running demo processes"
	@echo "  make test       — run unit tests"
	@echo "  make clean      — remove bin/ and the shm file"
	@echo ""
	@echo "Single-process targets (use only if you really want separate terminals):"
	@echo "  make sequencer / matching / marketdata"

build: tidy
	@mkdir -p $(BIN)
	go build -o $(BIN)/sequencer ./cmd/sequencer
	go build -o $(BIN)/matching ./cmd/matching
	go build -o $(BIN)/marketdata ./cmd/marketdata

tidy:
	go mod tidy

test:
	go test ./eventstore/...

# Run all three together with a single foreground process group.
# Ctrl-C cleanly terminates everything and removes the shm file.
run: build
	./scripts/run.sh

# Safety net: kill any orphaned demo processes and remove the shm file.
stop:
	-@pkill -x sequencer  2>/dev/null && echo "killed sequencer"  || true
	-@pkill -x matching   2>/dev/null && echo "killed matching"   || true
	-@pkill -x marketdata 2>/dev/null && echo "killed marketdata" || true
	-@rm -f $(SHM) && echo "removed $(SHM)" || true

ps:
	@pgrep -ax 'sequencer|matching|marketdata' || echo "no demo processes running"

# Single-process targets (separate terminals). Run sequencer first.
sequencer: build
	$(BIN)/sequencer -path $(SHM) -rate 50000

matching: build
	$(BIN)/matching -path $(SHM) -from tail

marketdata: build
	$(BIN)/marketdata -path $(SHM) -from tail

clean: stop
	rm -rf $(BIN)
