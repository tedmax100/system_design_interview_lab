package rbtreebook

import (
	"container/list"
	"fmt"

	"github.com/nathanyu/stock-exchange/internal/domain"
)

// ── 共用型別 ──────────────────────────────────────────────────────────────────

// bookLevel 儲存同一價位的所有掛單。
// 價格本身存在父節點 rbNode[*bookLevel].key，此處不重複儲存。
type bookLevel struct {
	TotalVolume int64
	Orders      *list.List // 元素為 *domain.Order，維持 FIFO 順序
}

// orderEntry 將一筆訂單連結到取消操作所需的全部資訊：
//   - element：在 bookLevel 鏈結串列中的位置，O(1) 移除
//   - node：RB 樹中的節點指針，當價位清空時 O(log n) 刪除
type orderEntry struct {
	order   *domain.Order
	element *list.Element
	node    *rbNode[*bookLevel]
}

// ── RBBook（委託簿單邊） ──────────────────────────────────────────────────────

// RBBook 是委託簿的單邊（買盤或賣盤），以紅黑樹維護價位索引。
//
// 複雜度摘要（對比 internal/orderbook 的 HashMap+List）：
//
//	操作            RBBook          HashMap+List
//	-----------     ----------      ------------
//	addOrder        O(log n)        O(1) 插入 + O(n) refreshBestPrice
//	removeOrder     O(log n)        O(1) 移除 + O(n) refreshBestPrice
//	BestPrice       O(log n)        O(1) 快取（O(n) 維護後讀取）
//	GetL2Snapshot   O(n) 中序走訪   O(n log n) 排序
type RBBook struct {
	Side domain.Side
	tree *rbTree[*bookLevel]
}

// NewRBBook 建立空的單邊委託簿。
func NewRBBook(side domain.Side) *RBBook {
	return &RBBook{Side: side, tree: newRBTree[*bookLevel]()}
}

// BestPrice 返回此邊的最佳價格：
//   - 買盤 → 最高買價（Max，走右脊柱）
//   - 賣盤 → 最低賣價（Min，走左脊柱）
//
// 委託簿為空時返回 0。
// 複雜度：O(log n)—無快取指針，每次呼叫都走脊柱。
func (b *RBBook) BestPrice() int64 {
	var node *rbNode[*bookLevel]
	if b.Side == domain.SideBuy {
		node = b.tree.Max()
	} else {
		node = b.tree.Min()
	}
	if node == nil {
		return 0
	}
	return node.key
}

// HasOrders 回報此邊是否存在掛單。
func (b *RBBook) HasOrders() bool { return b.tree.Len() > 0 }

// addOrder 將訂單插入對應的價位，若價位不存在則先建立。
// 同時返回 list.Element 與 rbNode，供 orderEntry 儲存以便快速取消。
// 複雜度：O(log n)—RB 樹搜尋／插入。
func (b *RBBook) addOrder(order *domain.Order) (*list.Element, *rbNode[*bookLevel]) {
	node := b.tree.Search(order.Price)
	if node == nil {
		lvl := &bookLevel{Orders: list.New()}
		node = b.tree.Insert(order.Price, lvl)
	}
	node.val.TotalVolume += order.RemainingQuantity
	elem := node.val.Orders.PushBack(order)
	return elem, node
}

// removeOrder 從價位中移除一筆訂單。
// 若價位清空，則同步從 RB 樹刪除對應節點。
// 複雜度：O(1) 鏈結串列移除 + 價位清空時 O(log n) RB 刪除。
func (b *RBBook) removeOrder(entry *orderEntry) {
	lvl := entry.node.val
	lvl.Orders.Remove(entry.element)
	lvl.TotalVolume -= entry.order.RemainingQuantity
	if lvl.Orders.Len() == 0 {
		b.tree.Delete(entry.node)
	}
}

// ── RBOrderBook（完整雙邊委託簿） ────────────────────────────────────────────

// RBOrderBook 是單一交易標的的完整雙邊委託簿。
// 結構上等同於 orderbook.OrderBook（HashMap+List 版本），
// 但以紅黑樹作為價格索引，對應早期電子交易所倚賴 C++ std::map / Java TreeMap 的設計。
type RBOrderBook struct {
	Symbol   string
	BuyBook  *RBBook
	SellBook *RBBook
	OrderMap map[string]*orderEntry // 訂單 ID → entry，提供 O(1) 取消查詢
}

// NewRBOrderBook 建立指定標的的空委託簿。
func NewRBOrderBook(symbol string) *RBOrderBook {
	return &RBOrderBook{
		Symbol:   symbol,
		BuyBook:  NewRBBook(domain.SideBuy),
		SellBook: NewRBBook(domain.SideSell),
		OrderMap: make(map[string]*orderEntry),
	}
}

// AddOrder 將限價掛單放入委託簿的對應邊。
// 複雜度：O(log n)。
func (ob *RBOrderBook) AddOrder(order *domain.Order) {
	var book *RBBook
	if order.Side == domain.SideBuy {
		book = ob.BuyBook
	} else {
		book = ob.SellBook
	}
	elem, node := book.addOrder(order)
	ob.OrderMap[order.OrderID] = &orderEntry{order: order, element: elem, node: node}
}

// CancelOrder 依訂單 ID 移除掛單，並將狀態標記為已取消。找不到時返回 nil。
// 複雜度：O(1) map 查詢 + O(1) 鏈結串列移除 + 價位清空時 O(log n) RB 刪除。
func (ob *RBOrderBook) CancelOrder(orderID string) *domain.Order {
	entry, ok := ob.OrderMap[orderID]
	if !ok {
		return nil
	}
	var book *RBBook
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
// 每個成交價位的成本：
//   - BestPrice  ：O(log n) 脊柱走訪
//   - Search     ：O(log n)
//   - 價位清空時 ：O(log n) RB 刪除
func (ob *RBOrderBook) MatchOrder(taker *domain.Order) []*domain.Execution {
	var oppBook *RBBook
	if taker.Side == domain.SideBuy {
		oppBook = ob.SellBook
	} else {
		oppBook = ob.BuyBook
	}

	var executions []*domain.Execution
	execSeq := 0

	for taker.RemainingQuantity > 0 && oppBook.HasOrders() {
		bestPrice := oppBook.BestPrice() // O(log n)—脊柱走訪

		if taker.Side == domain.SideBuy && taker.Price < bestPrice {
			break
		}
		if taker.Side == domain.SideSell && taker.Price > bestPrice {
			break
		}

		node := oppBook.tree.Search(bestPrice) // O(log n)
		if node == nil {
			break
		}
		lvl := node.val

		for taker.RemainingQuantity > 0 && lvl.Orders.Len() > 0 {
			front := lvl.Orders.Front()
			maker := front.Value.(*domain.Order)
			matchQty := min(taker.RemainingQuantity, maker.RemainingQuantity)

			taker.FilledQuantity += matchQty
			taker.RemainingQuantity -= matchQty
			maker.FilledQuantity += matchQty
			maker.RemainingQuantity -= matchQty
			lvl.TotalVolume -= matchQty

			if maker.RemainingQuantity == 0 {
				maker.Status = domain.OrderStatusFilled
				lvl.Orders.Remove(front)
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

		if lvl.Orders.Len() == 0 {
			oppBook.tree.Delete(node) // O(log n)
		}
	}
	return executions
}

// GetL2Snapshot 返回最多 depth 個價位的 L2 委託簿快照。
//
// 紅黑樹的中序走訪天然輸出有序價位，O(n) 即可完成—
// 這是相對 HashMap 版本（需要 sort.Slice，O(n log n)）的主要優勢。
func (ob *RBOrderBook) GetL2Snapshot(depth int) *domain.L2OrderBook {
	return &domain.L2OrderBook{
		Symbol: ob.Symbol,
		Bids:   rbAggregateLevels(ob.BuyBook, depth, true),
		Asks:   rbAggregateLevels(ob.SellBook, depth, false),
	}
}

// rbAggregateLevels 從 RBBook 建立 PriceLevel 切片。
// descending=true → 買盤（價格由高到低）。
// descending=false → 賣盤（價格由低到高）。
func rbAggregateLevels(book *RBBook, depth int, descending bool) []domain.PriceLevel {
	nodes := book.tree.InorderSlice() // 升序，O(n)
	result := make([]domain.PriceLevel, 0, len(nodes))

	if descending {
		// 買盤：反向中序走訪（最高價優先）
		for i := len(nodes) - 1; i >= 0; i-- {
			if depth > 0 && len(result) >= depth {
				break
			}
			n := nodes[i]
			result = append(result, domain.PriceLevel{Price: n.key, Quantity: n.val.TotalVolume})
		}
	} else {
		// 賣盤：已是升序
		for _, n := range nodes {
			if depth > 0 && len(result) >= depth {
				break
			}
			result = append(result, domain.PriceLevel{Price: n.key, Quantity: n.val.TotalVolume})
		}
	}
	return result
}
