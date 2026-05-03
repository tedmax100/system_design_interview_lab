// Package hmbook 以 HashMap + DoublyLinkedList 實作委託簿。
//
// 這是《System Design Interview Vol.2》第 13 章描述的「生產級」股票交易所資料結構，
// 代表從早期紅黑樹設計（internal/rbtreebook）演進而來的現代架構。
//
// # 三層架構
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│  第三層 │  bestPrice (int64)  ◄── 快取指針，O(1) 更新               │
//	├─────────────────────────────────────────────────────────────────────┤
//	│  第一層 │  priceMap: map[price → *bookLevel]   （HashMap）           │
//	│         │     ├── price 100.10 → bookLevel { 總量, *List }          │
//	│         │     ├── price 100.11 → bookLevel { 總量, *List }          │
//	│         │     └── price 100.12 → bookLevel { 總量, *List }          │
//	├─────────────────────────────────────────────────────────────────────┤
//	│  第二層 │  每個價位的 *list.List（雙向鏈結串列，FIFO）               │
//	│         │     → [訂單-A] ↔ [訂單-B] ↔ [訂單-C]                    │
//	├─────────────────────────────────────────────────────────────────────┤
//	│  索引   │  OrderMap: map[訂單ID → *orderEntry]                       │
//	│         │     → O(1) 取消，不需掃描委託簿                            │
//	└─────────────────────────────────────────────────────────────────────┘
//
// # 相較於 Naive 實作的關鍵優化
//
// Naive 做法（internal/orderbook）在每次 add 與 remove 後都呼叫 refreshBestPrice()—
// 這是一次 O(n) 全掃描。本套件修正了這個問題：
//
//	addOrder    ：O(1) 比較並更新（新價格優於快取？更新。否則不動。）
//	removeOrder ：O(1) 鏈結串列移除 + 僅在「最佳價位清空」時才 O(n) 重掃。
//	              在股票交易中此情況少見：價格集中在 spread 附近幾個 tick，
//	              下一個最佳價幾乎永遠緊鄰當前最佳。
//
// # 複雜度摘要
//
//	操作              hmbook（優化版）       naive orderbook       rbtreebook
//	-----------       ----------------       ---------------       ----------
//	AddOrder          O(1)                   O(1)+O(n)             O(log n)
//	CancelOrder       O(1) 一般情況          O(1)+O(n)             O(log n)
//	BestPrice         O(1) 快取              O(1) 快取             O(log n)
//	MatchOrder/價位   O(1)                   O(n) 每個消耗價位      O(log n)
//	GetL2Snapshot     O(n log n) 排序        O(n log n)            O(n) 中序走訪
package hmbook

import (
	"container/list"
	"fmt"
	"sort"

	"github.com/nathanyu/stock-exchange/internal/domain"
)

// ── bookLevel ─────────────────────────────────────────────────────────────────

// bookLevel 儲存同一價位的所有掛單，是 priceMap 的值型別。
type bookLevel struct {
	Price       int64
	TotalVolume int64
	Orders      *list.List // *domain.Order 的雙向鏈結串列，維持 FIFO 順序
}

// ── orderEntry ────────────────────────────────────────────────────────────────

// orderEntry 是 OrderMap 的值型別。
// 同時持有 list.Element 與 bookLevel 的直接指針，
// 使 CancelOrder 無需觸碰 priceMap 或掃描串列—純 O(1)。
type orderEntry struct {
	order   *domain.Order
	element *list.Element // 雙向鏈結串列中的節點指針，O(1) 移除
	level   *bookLevel   // 避免取消時多一次 map 查詢
}

// ── Book（單邊委託簿） ────────────────────────────────────────────────────────

// Book 是委託簿的單邊（買盤或賣盤）。
//
// bestPrice 欄位是快取指針，以三種方式更新：
//  1. addOrder：若新價格優於目前最佳，O(1) 比較並替換。
//  2. removeOrder（價位未清空）：無需更新—最佳價格不變。
//  3. removeOrder（價位清空且正好是最佳價位）：透過 refreshBestPrice 進行 O(n) 重掃。
//     實務上此情況不頻繁，因為撮合引擎一次就消耗掉最佳價位，
//     下一輪撮合會立即重新評估 BestPrice。
type Book struct {
	Side      domain.Side
	priceMap  map[int64]*bookLevel
	bestPrice int64 // 快取：最佳買價（買盤最高）或最佳賣價（賣盤最低）
	hasOrders bool
}

// NewBook 建立空的單邊委託簿。
func NewBook(side domain.Side) *Book {
	return &Book{
		Side:     side,
		priceMap: make(map[int64]*bookLevel),
	}
}

// BestPrice 返回快取的最佳價格。O(1)—不需走訪。
func (b *Book) BestPrice() int64 {
	if !b.hasOrders {
		return 0
	}
	return b.bestPrice
}

// HasOrders 回報此邊是否存在掛單。
func (b *Book) HasOrders() bool { return b.hasOrders }

// addOrder 將訂單附加到對應價位的串列尾端。
// 最佳價格快取以比較方式更新—O(1)，永不掃描。
func (b *Book) addOrder(order *domain.Order) *list.Element {
	level, exists := b.priceMap[order.Price]
	if !exists {
		level = &bookLevel{Price: order.Price, Orders: list.New()}
		b.priceMap[order.Price] = level
	}
	level.TotalVolume += order.RemainingQuantity
	elem := level.Orders.PushBack(order)

	// O(1) 快取更新：只做比較，絕不掃描。
	if !b.hasOrders {
		b.bestPrice = order.Price
		b.hasOrders = true
	} else if b.Side == domain.SideBuy && order.Price > b.bestPrice {
		b.bestPrice = order.Price // 新買價更高 → 成為新最佳
	} else if b.Side == domain.SideSell && order.Price < b.bestPrice {
		b.bestPrice = order.Price // 新賣價更低 → 成為新最佳
	}
	return elem
}

// removeOrder 從價位中移除一筆訂單。
//
// O(n) refreshBestPrice 僅在以下兩個條件同時成立時觸發：
//   - 被移除的訂單是該價位最後一筆（價位清空），且
//   - 該價位的價格等於快取的 bestPrice。
//
// 其餘所有移除均為 O(1)：list.Remove 是 O(1) 的指針操作，無需重掃。
func (b *Book) removeOrder(entry *orderEntry) {
	level := entry.level
	level.Orders.Remove(entry.element)      // O(1)—雙向鏈結串列指針操作
	level.TotalVolume -= entry.order.RemainingQuantity

	if level.Orders.Len() == 0 {
		delete(b.priceMap, level.Price) // O(1) map 刪除

		if len(b.priceMap) == 0 {
			b.hasOrders = false
			b.bestPrice = 0
			return
		}

		// 條件式重掃：只在真正必要時才付出 O(n) 代價。
		if level.Price == b.bestPrice {
			b.refreshBestPrice()
		}
	}
}

// refreshBestPrice 執行完整的 O(n) 掃描以尋找新的最佳價格。
// 這是慢路徑：僅在原本的最佳價位全數成交或取消後執行。
func (b *Book) refreshBestPrice() {
	if len(b.priceMap) == 0 {
		b.hasOrders = false
		b.bestPrice = 0
		return
	}
	b.hasOrders = true
	if b.Side == domain.SideBuy {
		best := int64(0)
		for price := range b.priceMap {
			if price > best {
				best = price
			}
		}
		b.bestPrice = best
	} else {
		best := int64(1<<62 - 1)
		for price := range b.priceMap {
			if price < best {
				best = price
			}
		}
		b.bestPrice = best
	}
}

// ── OrderBook（完整雙邊委託簿） ───────────────────────────────────────────────

// OrderBook 是單一交易標的的完整雙邊委託簿。
// 由兩個 Book 實例加上共用的 OrderMap 組成，提供 O(1) 的取消操作。
type OrderBook struct {
	Symbol   string
	BuyBook  *Book
	SellBook *Book
	OrderMap map[string]*orderEntry // 訂單 ID → entry，提供 O(1) 取消查詢
}

// NewOrderBook 建立指定標的的空委託簿。
func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{
		Symbol:   symbol,
		BuyBook:  NewBook(domain.SideBuy),
		SellBook: NewBook(domain.SideSell),
		OrderMap: make(map[string]*orderEntry),
	}
}

// AddOrder 將限價掛單放入委託簿的對應邊。
// 複雜度：O(1)—HashMap 插入 + 快取比較。
func (ob *OrderBook) AddOrder(order *domain.Order) {
	var book *Book
	if order.Side == domain.SideBuy {
		book = ob.BuyBook
	} else {
		book = ob.SellBook
	}
	elem := book.addOrder(order)
	level := book.priceMap[order.Price]
	ob.OrderMap[order.OrderID] = &orderEntry{order: order, element: elem, level: level}
}

// CancelOrder 依訂單 ID 移除掛單並標記為已取消。找不到時返回 nil。
// 複雜度：O(1) map 查詢 + O(1) 串列移除 + 最佳價位清空時 O(n) 重掃。
func (ob *OrderBook) CancelOrder(orderID string) *domain.Order {
	entry, ok := ob.OrderMap[orderID]
	if !ok {
		return nil
	}
	var book *Book
	if entry.order.Side == domain.SideBuy {
		book = ob.BuyBook
	} else {
		book = ob.SellBook
	}
	book.removeOrder(entry)
	delete(ob.OrderMap, orderID)
	entry.order.Status = domain.OrderStatusCanceled
	return entry.order
}

// MatchOrder 嘗試讓 taker 訂單與對手盤撮合。
// 返回所有成交記錄；未成交的剩餘數量保留在 taker 中。
//
// 每筆最佳價位的成本：O(1)—BestPrice 讀快取，直到價位清空前不需重掃。
func (ob *OrderBook) MatchOrder(taker *domain.Order) []*domain.Execution {
	var oppBook *Book
	if taker.Side == domain.SideBuy {
		oppBook = ob.SellBook
	} else {
		oppBook = ob.BuyBook
	}

	var executions []*domain.Execution
	execSeq := 0

	for taker.RemainingQuantity > 0 && oppBook.HasOrders() {
		bestPrice := oppBook.BestPrice() // O(1)—讀快取

		if taker.Side == domain.SideBuy && taker.Price < bestPrice {
			break
		}
		if taker.Side == domain.SideSell && taker.Price > bestPrice {
			break
		}

		level := oppBook.priceMap[bestPrice] // O(1) HashMap 查詢

		for taker.RemainingQuantity > 0 && level.Orders.Len() > 0 {
			front := level.Orders.Front()
			maker := front.Value.(*domain.Order)
			matchQty := min(taker.RemainingQuantity, maker.RemainingQuantity)

			taker.FilledQuantity += matchQty
			taker.RemainingQuantity -= matchQty
			maker.FilledQuantity += matchQty
			maker.RemainingQuantity -= matchQty
			level.TotalVolume -= matchQty

			if maker.RemainingQuantity == 0 {
				maker.Status = domain.OrderStatusFilled
				level.Orders.Remove(front)
				delete(ob.OrderMap, maker.OrderID)
			} else {
				maker.Status = domain.OrderStatusPartiallyFilled
			}

			if taker.RemainingQuantity == 0 {
				taker.Status = domain.OrderStatusFilled
			} else {
				taker.Status = domain.OrderStatusPartiallyFilled
			}

			execSeq++
			executions = append(executions, &domain.Execution{
				ExecID:       fmt.Sprintf("%s-exec-%d", taker.OrderID, execSeq),
				OrderID:      taker.OrderID,
				Symbol:       taker.Symbol,
				Side:         taker.Side,
				Price:        maker.Price,
				Quantity:     matchQty,
				MakerOrderID: maker.OrderID,
				TakerOrderID: taker.OrderID,
			})
		}

		// 價位全數成交：從 priceMap 移除並更新最佳價格快取。
		if level.Orders.Len() == 0 {
			delete(oppBook.priceMap, bestPrice)
			if len(oppBook.priceMap) == 0 {
				oppBook.hasOrders = false
				oppBook.bestPrice = 0
			} else {
				// 被消耗的價位正是最佳價位—立即重掃。
				oppBook.refreshBestPrice()
			}
		}
	}
	return executions
}

// GetL2Snapshot 返回最多 depth 個價位的 L2 委託簿快照。
// 複雜度：O(n log n)—HashMap 無內建順序，必須對鍵排序。
// （紅黑樹版本透過中序走訪避免此排序步驟，這是 HashMap 的取捨之一。）
func (ob *OrderBook) GetL2Snapshot(depth int) *domain.L2OrderBook {
	return &domain.L2OrderBook{
		Symbol: ob.Symbol,
		Bids:   aggregateLevels(ob.BuyBook, depth, true),
		Asks:   aggregateLevels(ob.SellBook, depth, false),
	}
}

// aggregateLevels 從單邊委託簿建立 PriceLevel 切片。
// descending=true → 買盤（價格由高到低）。
// descending=false → 賣盤（價格由低到高）。
func aggregateLevels(book *Book, depth int, descending bool) []domain.PriceLevel {
	prices := make([]int64, 0, len(book.priceMap))
	for price := range book.priceMap {
		prices = append(prices, price)
	}

	if descending {
		sort.Slice(prices, func(i, j int) bool { return prices[i] > prices[j] })
	} else {
		sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	}

	if depth > 0 && len(prices) > depth {
		prices = prices[:depth]
	}

	levels := make([]domain.PriceLevel, len(prices))
	for i, price := range prices {
		lvl := book.priceMap[price]
		levels[i] = domain.PriceLevel{Price: price, Quantity: lvl.TotalVolume}
	}
	return levels
}
