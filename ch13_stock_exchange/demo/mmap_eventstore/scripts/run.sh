#!/usr/bin/env bash
# Run sequencer + 2 consumers in one terminal. Ctrl-C cleanly stops all three.
#
# Why this exists: starting `./bin/sequencer & ./bin/matching & ./bin/marketdata`
# by hand leaves orphaned background jobs if your shell exits, and you have to
# hunt PIDs to kill them. This script keeps everything in one foreground
# process group so a single Ctrl-C terminates the lot.

set -u
cd "$(dirname "$0")/.."

SHM="${SHM:-/dev/shm/exchange_events}"
RATE="${RATE:-50000}"
SEQ_FLAGS="${SEQ_FLAGS:--rate $RATE}"

# Refuse to start if a previous run left processes alive — this is what caused
# the SIGSEGV: multiple sequencers writing the same mmap region violates the
# single-writer rule (ch13.md:1141).
if pgrep -x sequencer >/dev/null || pgrep -x matching >/dev/null || pgrep -x marketdata >/dev/null; then
    echo "ERROR: previous run still alive. Run 'make stop' first." >&2
    pgrep -ax 'sequencer|matching|marketdata' >&2 || true
    exit 1
fi

rm -f "$SHM"

cleanup() {
    trap '' INT TERM EXIT          # disarm so we don't recurse
    echo
    echo "[run] stopping all processes…"
    kill 0 2>/dev/null              # signal the whole process group
    wait 2>/dev/null
    rm -f "$SHM"
    echo "[run] done."
    exit 0                          # signal-driven shutdown is the expected path
}
trap cleanup INT TERM EXIT

# Prefix each child's stdout so you can tell them apart in one terminal.
./bin/sequencer  -path "$SHM" $SEQ_FLAGS                  2>&1 | sed -u 's/^/[seq] /' &
sleep 0.3                                                  # let the file initialise
./bin/matching   -path "$SHM" -from tail                  2>&1 | sed -u 's/^/[mch] /' &
./bin/marketdata -path "$SHM" -from tail                  2>&1 | sed -u 's/^/[mkt] /' &

wait
