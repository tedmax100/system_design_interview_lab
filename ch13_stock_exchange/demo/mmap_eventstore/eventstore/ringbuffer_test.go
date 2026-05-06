package eventstore

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func tmpRing(t *testing.T, slots uint64) *Ring {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ring")
	r, err := Create(path, slots)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestPublishThenRead(t *testing.T) {
	r := tmpRing(t, 16)
	if err := r.Publish(1, 42, []byte("hello")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	ev, res := r.TryRead(1)
	if res != ReadOK {
		t.Fatalf("want ReadOK, got %d", res)
	}
	if ev.Seq != 1 || ev.EventType != 42 || string(ev.Payload) != "hello" {
		t.Fatalf("bad event: %+v", ev)
	}
}

func TestReadEmpty(t *testing.T) {
	r := tmpRing(t, 16)
	_, res := r.TryRead(1)
	if res != ReadEmpty {
		t.Fatalf("want ReadEmpty, got %d", res)
	}
}

func TestOverrunDetection(t *testing.T) {
	r := tmpRing(t, 4) // 用很小的 ring 強迫繞回
	// Slot index = seq & 3。連續寫入 seq 1..5 後，slot 1 一開始放 seq=1，
	// 之後被 seq=5 覆蓋。落後的 consumer 來要 seq=1 時應該看到 ReadOverrun，
	// 並能從回傳的 ev.Seq 得知這個 slot 現在裝的是 seq=5。
	for i := uint64(1); i <= 5; i++ {
		if err := r.Publish(i, 1, []byte{byte(i)}); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}
	ev, res := r.TryRead(1)
	if res != ReadOverrun {
		t.Fatalf("want ReadOverrun, got %d (ev=%+v)", res, ev)
	}
	if ev.Seq != 5 {
		t.Fatalf("overrun should report newest seq=5, got %d", ev.Seq)
	}
}

func TestPublishOverlargePayload(t *testing.T) {
	r := tmpRing(t, 16)
	too := make([]byte, PayloadCapacity+1)
	if err := r.Publish(1, 1, too); err == nil {
		t.Fatal("want error for oversize payload")
	}
}

func TestSlotCountMustBePowerOfTwo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ring")
	if _, err := Create(path, 7); err == nil {
		t.Fatal("want error for non-power-of-two slot count")
	}
}

// TestSPMC_ProducerConsumer 用 goroutine 跑 1 個 producer + 2 個 consumer
//（同 process，但走的是同一條 mmap 路徑 —— 一樣會踩到 atomic publish/load
// 協定）。沒有 lock；ordering 與可見性完全靠 Slot.Seq 上的 release-acquire 配對。
func TestSPMC_ProducerConsumer(t *testing.T) {
	const total = 50_000
	// slotCount > total 確保不會繞回，consumer 即使慢也不會被 producer 覆蓋。
	// 實際部署不是把 buffer 配到能容納最壞情況的 lag，就是加 backpressure。
	r := tmpRing(t, 65_536)

	var wg sync.WaitGroup
	consumed := [2]uint64{}

	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var readSeq uint64
			deadline := time.Now().Add(5 * time.Second)
			for readSeq < total {
				ev, res := r.TryRead(readSeq + 1)
				switch res {
				case ReadOK:
					if ev.Seq != readSeq+1 {
						t.Errorf("consumer %d: expected seq %d got %d", idx, readSeq+1, ev.Seq)
						return
					}
					atomic.AddUint64(&consumed[idx], 1)
					readSeq = ev.Seq
				case ReadEmpty:
					if time.Now().After(deadline) {
						t.Errorf("consumer %d timeout at seq %d", idx, readSeq)
						return
					}
				case ReadOverrun:
					t.Errorf("consumer %d unexpected overrun at seq %d", idx, readSeq+1)
					return
				}
			}
		}(i)
	}

	// Producer（單一 writer）。
	go func() {
		buf := []byte("payload")
		for i := uint64(1); i <= total; i++ {
			if err := r.Publish(i, 1, buf); err != nil {
				t.Errorf("publish: %v", err)
				return
			}
		}
	}()

	wg.Wait()
	if consumed[0] != total || consumed[1] != total {
		t.Fatalf("consumers saw %v, want both = %d", consumed, total)
	}
}
