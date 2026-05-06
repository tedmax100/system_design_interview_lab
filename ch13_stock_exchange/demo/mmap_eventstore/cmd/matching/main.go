// matching 是 Event Store 的一個 consumer。它把 NewOrder 事件 pull 出來，
// 假裝撮合，並回報每一筆事件從 sequencer 發布到本端收到所經過的延遲
// （也就是穿過 mmap 這條「wire」的時間）。
//
// 對應到 ch13.md 的 Matching Engine Application Loop（ch13.md:986-1012）：
// 單執行緒、自己 busy-poll 自己的 read offset。
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
		path = flag.String("path", "/dev/shm/exchange_events", "ring buffer 檔案路徑")
		busy = flag.Bool("busy", false, "空無事件時 busy-poll（書中設計），否則讓出 scheduler")
		from = flag.String("from", "tail", "起始位置：'head'（全部 replay）或 'tail'（跳過 backlog）")
	)
	flag.Parse()

	ring, err := eventstore.Open(*path)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer ring.Close()

	var readSeq uint64
	switch *from {
	case "head":
		readSeq = 0
	case "tail":
		readSeq = ring.WriteSeq()
	default:
		log.Fatalf("invalid -from=%s", *from)
	}
	log.Printf("matching up: slots=%d, starting at seq=%d (busy=%v)", ring.SlotCount(), readSeq, *busy)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	statsTicker := time.NewTicker(time.Second)
	defer statsTicker.Stop()

	var (
		processed     uint64
		overruns      uint64
		latencySumNs  int64
		latencyMaxNs  int64
		lastProcessed uint64
		startReport   = time.Now()
	)

	for {
		select {
		case <-stop:
			log.Printf("stopped at seq=%d (processed=%d, overruns=%d)", readSeq, processed, overruns)
			return
		case <-statsTicker.C:
			delta := processed - lastProcessed
			elapsed := time.Since(startReport).Seconds()
			avg := time.Duration(0)
			if delta > 0 {
				avg = time.Duration(latencySumNs / int64(delta))
			}
			log.Printf("processed=%d (+%.0f/s) overruns=%d avgLat=%v maxLat=%v",
				processed, float64(delta)/elapsed, overruns, avg, time.Duration(latencyMaxNs))
			lastProcessed = processed
			latencySumNs = 0
			latencyMaxNs = 0
			startReport = time.Now()
		default:
		}

		next := readSeq + 1
		ev, res := ring.TryRead(next)
		switch res {
		case eventstore.ReadOK:
			handleEvent(ev)
			now := time.Now().UnixNano()
			if ev.EventType == eventstore.EventNewOrder {
				ord := eventstore.DecodeNewOrder(ev.Payload)
				lat := now - int64(ord.TimestampNs)
				if lat > 0 {
					latencySumNs += lat
					if lat > latencyMaxNs {
						latencyMaxNs = lat
					}
				}
			}
			processed++
			readSeq = next
		case eventstore.ReadEmpty:
			if !*busy {
				// 在筆電上讓出 scheduler 比較友善（CPU 不會 100%）；
				// 書中釘核（pinned-CPU）的版本會直接 `continue`。
				runtime.Gosched()
			}
		case eventstore.ReadOverrun:
			lost := ev.Seq - next
			overruns += lost
			log.Printf("overrun: lost %d events, resyncing from seq=%d", lost, ev.Seq)
			readSeq = ev.Seq - 1 // 下一輪會去試 ev.Seq
		}
	}
}

// handleEvent 是真正撮合引擎更新 order book 的地方。Demo 裡我們只裝作這
// 筆訂單在限價成交，不做任何後續處理。
func handleEvent(ev eventstore.Event) {
	if ev.EventType != eventstore.EventNewOrder {
		return
	}
	// 真實實作會：找對應價位、跟對手方撮合、產生 fill，再把 OrderFilledEvent
	// 寫回 ring（這個 ring 是 single-writer，所以實際上需要第二條 ring）。
	// Demo 直接丟掉結果。
	_ = eventstore.DecodeNewOrder(ev.Payload)
}
