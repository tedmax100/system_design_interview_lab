// Package eventstore 實作一個由 mmap 檔案承載的單一生產者、多消費者
// （SPMC, Single-Producer Multi-Consumer）lock-free 環形緩衝區。
//
// 佈局（Layout，二進位、little-endian、固定 offset）：
//
//	+------------------------------------------------+
//	| Header (64 bytes, cache-line aligned)          |
//	|   magic     uint64                             |
//	|   slotCount uint64   (必須是 2 的次方)         |
//	|   slotSize  uint64   (256 bytes)               |
//	|   writeSeq  uint64   (atomic, 最新已發布的 seq)|
//	|   _pad      [32]byte                           |
//	+------------------------------------------------+
//	| Slot[0]   (256 bytes)                          |
//	|   seq        uint64  (atomic; 0 = 空槽)        |
//	|   eventType  uint32                            |
//	|   payloadLen uint32                            |
//	|   payload    [240]byte                         |
//	+------------------------------------------------+
//	| Slot[1] ... Slot[N-1]                          |
//	+------------------------------------------------+
//
// 並行模型 — sequence number 就是同步原語：
//
//   - Producer（單一）先寫 payload，再以 atomic store 把 slot.seq 設成
//     nextSeq 來「發布」（在 x86/arm64 上 Go 的 sync/atomic 提供 release
//     語意）。最後更新 header.writeSeq 純粹是給統計用。
//
//   - Consumer（多個）各自維護自己的 readSeq。要消費 nextSeq 時，atomic
//     load slot.seq；若等於 nextSeq 就保證 payload 也可見（acquire 語意）。
//     若大於 nextSeq，代表 producer 已經繞回覆寫過去 — overrun。
package eventstore

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	Magic           uint64 = 0x4558434847454E54 // "EXCHGENT"
	HeaderSize             = 64
	SlotSize               = 256
	PayloadCapacity        = SlotSize - 16 // 240 bytes
)

// Header 是寫入檔案的檔頭。欄位順序與大小屬於 wire format 的一部分，
// 不可重新排列。
type Header struct {
	Magic     uint64
	SlotCount uint64
	SlotSize  uint64
	WriteSeq  uint64
	_         [32]byte
}

// Slot 是一筆固定大小的event log。欄位順序屬於 wire format 的一部分。
type Slot struct {
	Seq        uint64 // 這格目前裝的是第幾筆事件。它就是這格的「狀態標籤」
	EventType  uint32
	PayloadLen uint32
	Payload    [PayloadCapacity]byte // 固定大小，避免 variable-length payload 帶來的複雜度
}

// Ring 是一個 mmap 環形緩衝區的 handle。可安全地跨 process 由「一個 writer +
// 多個 reader」共用；不支援多個 writer。
type Ring struct {
	data      []byte
	header    *Header
	slots     []Slot // 別名指向 mmap 區域，不可再 reslice
	slotCount uint64
	mask      uint64
}

// Create 把 path 截斷為 slotCount 個 slot 所需的大小，寫入 header，並 mmap。
// slotCount 必須是 2 的次方。
func Create(path string, slotCount uint64) (*Ring, error) {
	if slotCount == 0 || slotCount&(slotCount-1) != 0 {
		return nil, fmt.Errorf("slotCount must be a power of two, got %d", slotCount)
	}
	size := int64(HeaderSize) + int64(slotCount)*int64(SlotSize)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		return nil, fmt.Errorf("truncate: %w", err)
	}

	r, err := mapFile(f, int(size))
	if err != nil {
		return nil, err
	}
	// 初始化 header。整塊清零可確保所有 slot.seq 從 0 開始。
	for i := range r.data {
		r.data[i] = 0
	}
	r.header.Magic = Magic
	r.header.SlotCount = slotCount
	r.header.SlotSize = SlotSize
	atomic.StoreUint64(&r.header.WriteSeq, 0)
	r.slotCount = slotCount
	r.mask = slotCount - 1
	return r, nil
}

// Open mmap 一個既存的 ring buffer（consumer 端用）。
func Open(path string) (*Ring, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	r, err := mapFile(f, int(st.Size()))
	if err != nil {
		return nil, err
	}
	if r.header.Magic != Magic {
		_ = unix.Munmap(r.data)
		return nil, fmt.Errorf("bad magic 0x%x — not an event store", r.header.Magic)
	}
	r.slotCount = r.header.SlotCount
	r.mask = r.slotCount - 1
	return r, nil
}

func mapFile(f *os.File, size int) (*Ring, error) {
	data, err := unix.Mmap(int(f.Fd()), 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, fmt.Errorf("mmap: %w", err)
	}
	if len(data) < HeaderSize+SlotSize {
		_ = unix.Munmap(data)
		return nil, fmt.Errorf("mapped region too small: %d bytes", len(data))
	}
	header := (*Header)(unsafe.Pointer(&data[0]))
	slotCount := (len(data) - HeaderSize) / SlotSize
	slotsPtr := (*Slot)(unsafe.Pointer(&data[HeaderSize]))
	slots := unsafe.Slice(slotsPtr, slotCount)
	return &Ring{data: data, header: header, slots: slots}, nil
}

// Close 解除 mmap 對應；backing file 不會被刪除。
func (r *Ring) Close() error {
	return unix.Munmap(r.data)
}

// SlotCount 回傳設定的 slot 總數。
func (r *Ring) SlotCount() uint64 { return r.slotCount }

// WriteSeq 回傳最後一筆已發布的 sequence number。對於後加入的 consumer
// 想要跳過歷史事件、從最新位置開始消費時很有用。
func (r *Ring) WriteSeq() uint64 { return atomic.LoadUint64(&r.header.WriteSeq) }

// Publish 把 payload 寫入下一個 slot，並 atomically 發布。
// nextSeq 是分配給這筆事件的 sequence number（在 ch13 的設計裡，呼叫者就是
// Sequencer）。只有在 payload 過大時才會回傳錯誤。
//
// 單一 writer 限定。呼叫者必須確保 nextSeq 嚴格遞增。
func (r *Ring) Publish(nextSeq uint64, eventType uint32, payload []byte) error {
	if len(payload) > PayloadCapacity {
		return fmt.Errorf("payload %d > capacity %d", len(payload), PayloadCapacity)
	}
	slot := &r.slots[nextSeq&r.mask]

	// 順序很重要：先寫 payload 與 header 欄位，最後才用 atomic store 寫入 Seq
	// 來「發布」。Seq 的 atomic store 是 release barrier，保證 payload 對
	// 後續執行 acquire load 的 consumer 可見。
	copy(slot.Payload[:], payload)
	slot.EventType = eventType
	slot.PayloadLen = uint32(len(payload))
	atomic.StoreUint64(&slot.Seq, nextSeq) // // ★ 最後一刻才 atomic 寫 seq

	atomic.StoreUint64(&r.header.WriteSeq, nextSeq)
	return nil
}

// Event 是 consumer 從 ring 拿到的一筆事件。
type Event struct {
	Seq       uint64
	EventType uint32
	Payload   []byte // 已複製出來，呼叫者擁有所有權（可長期保留）
}

// ReadResult 描述一次 Read 呼叫的結果。
type ReadResult int

const (
	// ReadOK — 成功讀到下一個 sequence number。
	ReadOK ReadResult = iota
	// ReadEmpty — producer 還沒寫到這個 seq；呼叫者應該 busy-poll 重試。
	ReadEmpty
	// ReadOverrun — producer 已經繞回覆寫過去；事件遺失。ev.Seq 會帶上目前
	// 看到的最新 seq，呼叫者可以據此 resync。
	ReadOverrun
)

// TryRead 嘗試讀取 `expected` 對應的 slot；不會 block。
//
// 回傳：
//   - ReadOK：ev 已填好，expected 已被消費。
//   - ReadEmpty：還沒資料，呼叫者應 spin / yield 後重試。
//   - ReadOverrun：這個 slot 現在裝的是比 `expected` 更新的 seq。ev.Seq 就是
//     那個更新的 seq；呼叫者可推算遺失筆數 = ev.Seq - expected，
//     決定要從 ev.Seq resync 還是當錯誤處理。
func (r *Ring) TryRead(expected uint64) (ev Event, result ReadResult) {
	// r.mask = slotCount - 1（slotCount 必須是 2 的次方），AND 一下就等於取餘數，但快很多。
	// 舉例（slotCount = 4，mask = 3 = 0b11）：
	// expected:  1  2  3  4  5  6  7  8  9  10  ...
	// slotIdx:   1  2  3  0  1  2  3  0  1   2  ← 每 4 個繞一圈   /
	// 「第 5 筆會放在 slot[1]、第 8 筆會放在 slot[0]」 — 這是 ring buffer 的本質。
	slot := &r.slots[expected&r.mask] //  算位置

	// slot.Seq 的值會隨著 producer 發布事件而嚴格遞增；consumer 以 expected 為基準去比較：
	got := atomic.LoadUint64(&slot.Seq) // 讀標籤
	switch {
	case got == expected:
		// 來了，正是我要的那筆
		// consumer 想要 seq=5 ; Producer 已經寫到 seq=5。
		// expected & mask = 5 & 3 = 1 → 看 slot[1]
		//   slot[1]:  seq=5  type=NewOrder  payload=...
		//      		↑
		//    		   got=5, expected=5 ✓
		ev.Seq = got
		ev.EventType = slot.EventType
		n := min(slot.PayloadLen, PayloadCapacity)
		ev.Payload = append([]byte(nil), slot.Payload[:n]...)
		return ev, ReadOK
	case got < expected:
		// Producer 還沒寫到我這筆
		return Event{}, ReadEmpty
	default: // got > expected
		// Producer 已經繞了一整圈把我那格覆蓋了，事件遺失
		// Producer 已經繞過去覆蓋了。回報目前最新 seq 讓呼叫者 resync。
		ev.Seq = got
		return ev, ReadOverrun
	}
}

// PutUint64 / GetUint64 等是給呼叫者組裝小型 binary payload 用的小工具，
// 避免每處都要 import encoding/binary。
func PutUint64(b []byte, off int, v uint64) { binary.LittleEndian.PutUint64(b[off:], v) }
func GetUint64(b []byte, off int) uint64    { return binary.LittleEndian.Uint64(b[off:]) }
func PutUint32(b []byte, off int, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }
func GetUint32(b []byte, off int) uint32    { return binary.LittleEndian.Uint32(b[off:]) }
