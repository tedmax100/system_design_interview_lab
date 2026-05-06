package eventstore

// EventType 是寫入 Slot.EventType 欄位的值。ch13.md 的設計用 SBE 編碼 payload；
// 本 demo 為了不引入外部相依，改用手寫、固定 offset 的二進位編排。
const (
	EventNewOrder    uint32 = 1
	EventOrderFilled uint32 = 2
)

// 訂單方向。
const (
	SideBuy  uint8 = 1
	SideSell uint8 = 2
)

// NewOrderEvent 佈局（共 41 bytes）：
//
//	off  size  欄位
//	  0     8  TimestampNs   (producer 寫入時的 wall clock；用來量延遲)
//	  8     8  OrderID
//	 16     1  Side          (1=buy, 2=sell)
//	 17     8  PriceCents
//	 25     8  Quantity
//	 33     8  ClientID
const newOrderSize = 41

type NewOrder struct {
	TimestampNs uint64
	OrderID     uint64
	Side        uint8
	PriceCents  int64
	Quantity    int64
	ClientID    uint64
}

func EncodeNewOrder(buf []byte, e NewOrder) int {
	_ = buf[newOrderSize-1] // 提示 compiler 一次做完邊界檢查
	PutUint64(buf, 0, e.TimestampNs)
	PutUint64(buf, 8, e.OrderID)
	buf[16] = e.Side
	PutUint64(buf, 17, uint64(e.PriceCents))
	PutUint64(buf, 25, uint64(e.Quantity))
	PutUint64(buf, 33, e.ClientID)
	return newOrderSize
}

func DecodeNewOrder(buf []byte) NewOrder {
	_ = buf[newOrderSize-1]
	return NewOrder{
		TimestampNs: GetUint64(buf, 0),
		OrderID:     GetUint64(buf, 8),
		Side:        buf[16],
		PriceCents:  int64(GetUint64(buf, 17)),
		Quantity:    int64(GetUint64(buf, 25)),
		ClientID:    GetUint64(buf, 33),
	}
}

func NewOrderSize() int { return newOrderSize }

// OrderFilledEvent 佈局（共 32 bytes）：
//
//	off  size  欄位
//	  0     8  TimestampNs
//	  8     8  OrderID
//	 16     8  FillPriceCents
//	 24     8  FillQuantity
const orderFilledSize = 32

type OrderFilled struct {
	TimestampNs    uint64
	OrderID        uint64
	FillPriceCents int64
	FillQuantity   int64
}

func EncodeOrderFilled(buf []byte, e OrderFilled) int {
	_ = buf[orderFilledSize-1]
	PutUint64(buf, 0, e.TimestampNs)
	PutUint64(buf, 8, e.OrderID)
	PutUint64(buf, 16, uint64(e.FillPriceCents))
	PutUint64(buf, 24, uint64(e.FillQuantity))
	return orderFilledSize
}

func DecodeOrderFilled(buf []byte) OrderFilled {
	_ = buf[orderFilledSize-1]
	return OrderFilled{
		TimestampNs:    GetUint64(buf, 0),
		OrderID:        GetUint64(buf, 8),
		FillPriceCents: int64(GetUint64(buf, 16)),
		FillQuantity:   int64(GetUint64(buf, 24)),
	}
}

func OrderFilledSize() int { return orderFilledSize }

// EventTypeName 回傳給 log 用的人類可讀名稱。
func EventTypeName(t uint32) string {
	switch t {
	case EventNewOrder:
		return "NewOrder"
	case EventOrderFilled:
		return "OrderFilled"
	default:
		return "Unknown"
	}
}
