// marketdata 是 Event Store 的第二個獨立 consumer。它跑在自己的 process、
// 維護自己的 read offset —— producer 完全不知道它的存在，也不會被它拖慢
// matching engine。這正是 ch13.md 設計用 mmap 換來的「多消費者」特性
//（ch13.md:1094-1101）。
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/nathanyu/stock-exchange/demo/mmap_eventstore/eventstore"
)

func main() {
	var (
		path    = flag.String("path", "/dev/shm/exchange_events", "ring buffer 檔案路徑")
		from    = flag.String("from", "tail", "起始位置：'head' 或 'tail'")
		verbose = flag.Bool("v", false, "印出每一筆事件（會非常吵）")
	)
	flag.Parse()

	ring, err := eventstore.Open(*path)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer ring.Close()

	var readSeq uint64
	if *from == "tail" {
		readSeq = ring.WriteSeq()
	}
	log.Printf("marketdata up: starting at seq=%d", readSeq)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	stats := time.NewTicker(time.Second)
	defer stats.Stop()

	var (
		buyVol, sellVol int64
		processed       uint64
		lastProcessed   uint64
		startReport     = time.Now()
	)

	for {
		select {
		case <-stop:
			log.Printf("stopped at seq=%d (processed=%d, buyVol=%d sellVol=%d)",
				readSeq, processed, buyVol, sellVol)
			return
		case <-stats.C:
			delta := processed - lastProcessed
			elapsed := time.Since(startReport).Seconds()
			log.Printf("processed=%d (+%.0f/s) buyVol=%d sellVol=%d",
				processed, float64(delta)/elapsed, buyVol, sellVol)
			lastProcessed = processed
			startReport = time.Now()
		default:
		}

		next := readSeq + 1
		ev, res := ring.TryRead(next)
		switch res {
		case eventstore.ReadOK:
			if ev.EventType == eventstore.EventNewOrder {
				ord := eventstore.DecodeNewOrder(ev.Payload)
				if ord.Side == eventstore.SideBuy {
					buyVol += ord.Quantity
				} else {
					sellVol += ord.Quantity
				}
				if *verbose {
					log.Printf("seq=%d side=%d price=%d qty=%d", ev.Seq, ord.Side, ord.PriceCents, ord.Quantity)
				}
			}
			processed++
			readSeq = next
		case eventstore.ReadEmpty:
			runtime.Gosched()
		case eventstore.ReadOverrun:
			log.Printf("overrun: jumping from seq=%d to %d (lost %d)", next, ev.Seq, ev.Seq-next)
			readSeq = ev.Seq - 1
		}
	}
}
