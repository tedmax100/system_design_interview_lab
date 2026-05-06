// sequencer 是這個 demo 中 Event Store 唯一的 producer。它以固定速率產生
// 假的 NewOrder 事件，注入嚴格遞增的 sequence number，再寫進 mmap ring buffer。
//
// 在 ch13.md 的設計裡，Sequencer 是唯一的 writer（ch13.md:1141 規定
// 「single Writer is non-negotiable」）。多個 producer 會在 seq counter 上
// 互相競爭，破壞 ordering 保證。
package main

import (
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nathanyu/stock-exchange/demo/mmap_eventstore/eventstore"
)

func main() {
	var (
		path  = flag.String("path", "/dev/shm/exchange_events", "ring buffer 檔案路徑（建議放在 tmpfs）")
		slots = flag.Uint64("slots", 1<<16, "slot 數量（必須是 2 的次方）")
		rate  = flag.Int("rate", 50_000, "目標每秒事件數（用 time.Ticker pace；受 ticker 解析度限制）")
		burst = flag.Bool("burst", false, "忽略 -rate，全速 publish（量測最大吞吐用）")
		reset = flag.Bool("reset", true, "啟動時把檔案 truncate 並重新初始化")
	)
	flag.Parse()

	if !*reset {
		if _, err := os.Stat(*path); err != nil {
			log.Fatalf("file %s missing and -reset=false: %v", *path, err)
		}
	}
	if *reset {
		if err := os.MkdirAll(filepath.Dir(*path), 0o755); err != nil {
			log.Fatalf("mkdir: %v", err)
		}
		_ = os.Remove(*path)
	}

	ring, err := eventstore.Create(*path, *slots)
	if err != nil {
		log.Fatalf("create ring: %v", err)
	}
	defer ring.Close()

	log.Printf("sequencer up: path=%s slots=%d rate=%d/s", *path, *slots, *rate)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	buf := make([]byte, eventstore.NewOrderSize())

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()

	var nextSeq uint64
	var lastReportSeq uint64
	startReport := time.Now()

	publishOne := func() {
		nextSeq++
		ord := eventstore.NewOrder{
			TimestampNs: uint64(time.Now().UnixNano()),
			OrderID:     nextSeq,
			Side:        pickSide(rng),
			PriceCents:  500_00 + int64(rng.Intn(2000)-1000),
			Quantity:    int64(1 + rng.Intn(100)),
			ClientID:    uint64(1 + rng.Intn(1000)),
		}
		n := eventstore.EncodeNewOrder(buf, ord)
		if err := ring.Publish(nextSeq, eventstore.EventNewOrder, buf[:n]); err != nil {
			log.Fatalf("publish: %v", err)
		}
	}

	logStats := func() {
		elapsed := time.Since(startReport).Seconds()
		rps := float64(nextSeq-lastReportSeq) / elapsed
		log.Printf("published seq=%d (%.0f ev/s)", nextSeq, rps)
		lastReportSeq = nextSeq
		startReport = time.Now()
	}

	if *burst {
		// Hot loop。每 1024 筆事件才檢查一次 stop / stats，避免拖慢 publish 吞吐。
		for {
			for range 1024 {
				publishOne()
			}
			select {
			case <-stop:
				log.Printf("stopped at seq=%d", nextSeq)
				return
			case <-statsTicker.C:
				logStats()
			default:
			}
		}
	}

	interval := time.Second / time.Duration(*rate)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			log.Printf("stopped at seq=%d", nextSeq)
			return
		case <-statsTicker.C:
			logStats()
		case <-ticker.C:
			publishOne()
		}
	}
}

func pickSide(rng *rand.Rand) uint8 {
	if rng.Intn(2) == 0 {
		return eventstore.SideBuy
	}
	return eventstore.SideSell
}
