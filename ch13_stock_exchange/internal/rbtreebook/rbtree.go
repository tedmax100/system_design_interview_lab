// Package rbtreebook 以紅黑樹（Red-Black Tree）作為價格索引，實作委託簿。
// 此設計對應交易所電子化早期（C++ std::map / Java TreeMap 時代）的架構，
// 與 internal/orderbook 的 HashMap+List 實作並列，可透過單元測試與 benchmark 直接比較。
//
// 紅黑樹五條不變式（CLRS §13）：
//  1. 每個節點為紅或黑。
//  2. 根節點為黑。
//  3. 每個葉節點（哨兵 nil_）為黑。
//  4. 紅節點的兩個子節點皆為黑（不得有相鄰兩個紅節點）。
//  5. 從任一節點出發到其後代葉節點的所有簡單路徑，黑節點數量相同（黑高度一致）。
//
// 與 HashMap+List 相比，各操作複雜度：
//   - Insert  ：O(log n) + 最多 2 次旋轉 + O(log n) 次重染色
//   - Delete  ：O(log n) + 最多 3 次旋轉 + O(log n) 次重染色
//   - Min/Max ：O(log n)—無快取指針，每次呼叫都走脊柱
//   - Search  ：O(log n)
//   - Inorder ：O(n)—天然有序，不需額外排序
package rbtreebook

// rbColor 代表紅黑樹節點的顏色。
type rbColor bool

const (
	rbRed   rbColor = true
	rbBlack rbColor = false
)

// rbNode 是紅黑樹的單一節點。
// V 為值型別（委託簿中使用 *bookLevel）。
type rbNode[V any] struct {
	key    int64
	val    V
	color  rbColor
	left   *rbNode[V]
	right  *rbNode[V]
	parent *rbNode[V]
}

// rbTree 是以 int64 為鍵的泛型紅黑樹。
// 使用哨兵節點 nil_（永遠為黑）取代 nil 指針，消除邊界條件判斷。
type rbTree[V any] struct {
	root *rbNode[V]
	nil_ *rbNode[V] // 哨兵節點—所有「空」的子節點／父節點指針都指向它
	size int
}

func newRBTree[V any]() *rbTree[V] {
	var zero V
	sentinel := &rbNode[V]{color: rbBlack, val: zero}
	// 讓哨兵的三個指針都指向自身，確保遍歷時不會跳出樹外。
	sentinel.left = sentinel
	sentinel.right = sentinel
	sentinel.parent = sentinel
	return &rbTree[V]{
		nil_: sentinel,
		root: sentinel,
	}
}

// ── 旋轉 ──────────────────────────────────────────────────────────────────────

func (t *rbTree[V]) leftRotate(x *rbNode[V]) {
	y := x.right
	x.right = y.left
	if y.left != t.nil_ {
		y.left.parent = x
	}
	y.parent = x.parent
	if x.parent == t.nil_ {
		t.root = y
	} else if x == x.parent.left {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.left = x
	x.parent = y
}

func (t *rbTree[V]) rightRotate(y *rbNode[V]) {
	x := y.left
	y.left = x.right
	if x.right != t.nil_ {
		x.right.parent = y
	}
	x.parent = y.parent
	if y.parent == t.nil_ {
		t.root = x
	} else if y == y.parent.right {
		y.parent.right = x
	} else {
		y.parent.left = x
	}
	x.right = y
	y.parent = x
}

// ── 插入 ──────────────────────────────────────────────────────────────────────

// Insert 新增一個鍵值對；若鍵已存在則更新值並返回既有節點。
// 複雜度：O(log n) BST 走訪 + O(log n) 修復 + 最多 2 次旋轉。
func (t *rbTree[V]) Insert(key int64, val V) *rbNode[V] {
	y := t.nil_
	x := t.root
	for x != t.nil_ {
		y = x
		switch {
		case key < x.key:
			x = x.left
		case key > x.key:
			x = x.right
		default:
			// 鍵已存在—直接更新值。
			x.val = val
			return x
		}
	}

	z := &rbNode[V]{
		key:    key,
		val:    val,
		color:  rbRed,
		left:   t.nil_,
		right:  t.nil_,
		parent: y,
	}
	if y == t.nil_ {
		t.root = z
	} else if z.key < y.key {
		y.left = z
	} else {
		y.right = z
	}
	t.size++
	t.insertFixup(z)
	return z
}

// insertFixup 在 BST 插入後修復紅黑樹性質。
// 最多 O(log n) 次重染色 + 2 次旋轉（Case 2+3 執行後立即終止）。
func (t *rbTree[V]) insertFixup(z *rbNode[V]) {
	for z.parent.color == rbRed {
		if z.parent == z.parent.parent.left {
			uncle := z.parent.parent.right
			if uncle.color == rbRed {
				// Case 1—叔節點為紅：重染色，將違規向上推至祖父節點
				z.parent.color = rbBlack
				uncle.color = rbBlack
				z.parent.parent.color = rbRed
				z = z.parent.parent
			} else {
				if z == z.parent.right {
					// Case 2—叔節點為黑，z 是內側子節點：左旋使其對齊 Case 3
					z = z.parent
					t.leftRotate(z)
				}
				// Case 3—叔節點為黑，z 是外側子節點：單次右旋，終止
				z.parent.color = rbBlack
				z.parent.parent.color = rbRed
				t.rightRotate(z.parent.parent)
			}
		} else {
			// 對稱情況：父節點是右子節點
			uncle := z.parent.parent.left
			if uncle.color == rbRed {
				z.parent.color = rbBlack
				uncle.color = rbBlack
				z.parent.parent.color = rbRed
				z = z.parent.parent
			} else {
				if z == z.parent.left {
					z = z.parent
					t.rightRotate(z)
				}
				z.parent.color = rbBlack
				z.parent.parent.color = rbRed
				t.leftRotate(z.parent.parent)
			}
		}
	}
	t.root.color = rbBlack
}

// ── 刪除 ──────────────────────────────────────────────────────────────────────

// transplant 將以 u 為根的子樹替換為以 v 為根的子樹。
func (t *rbTree[V]) transplant(u, v *rbNode[V]) {
	if u.parent == t.nil_ {
		t.root = v
	} else if u == u.parent.left {
		u.parent.left = v
	} else {
		u.parent.right = v
	}
	v.parent = u.parent
}

// treeMinimum 返回以 x 為根的子樹中鍵值最小的節點。
func (t *rbTree[V]) treeMinimum(x *rbNode[V]) *rbNode[V] {
	for x.left != t.nil_ {
		x = x.left
	}
	return x
}

// Delete 從樹中移除節點 z（CLRS §13.4）。
// 複雜度：O(log n) 修復 + 最多 3 次旋轉。
// 呼叫者須確保節點屬於此樹。
func (t *rbTree[V]) Delete(z *rbNode[V]) {
	y := z
	yOrigColor := y.color
	var x *rbNode[V]

	if z.left == t.nil_ {
		x = z.right
		t.transplant(z, z.right)
	} else if z.right == t.nil_ {
		x = z.left
		t.transplant(z, z.left)
	} else {
		// y = 後繼節點（右子樹的最小值）
		y = t.treeMinimum(z.right)
		yOrigColor = y.color
		x = y.right
		if y.parent == z {
			x.parent = y
		} else {
			t.transplant(y, y.right)
			y.right = z.right
			y.right.parent = y
		}
		t.transplant(z, y)
		y.left = z.left
		y.left.parent = y
		y.color = z.color
	}
	t.size--
	if yOrigColor == rbBlack {
		t.deleteFixup(x)
	}
}

// deleteFixup 在刪除後修復紅黑樹性質。
func (t *rbTree[V]) deleteFixup(x *rbNode[V]) {
	for x != t.root && x.color == rbBlack {
		if x == x.parent.left {
			w := x.parent.right // 兄弟節點
			if w.color == rbRed {
				// Case 1—兄弟為紅：轉換為 Case 2–4
				w.color = rbBlack
				x.parent.color = rbRed
				t.leftRotate(x.parent)
				w = x.parent.right
			}
			if w.left.color == rbBlack && w.right.color == rbBlack {
				// Case 2—兄弟兩個子節點皆為黑：將雙黑向上推
				w.color = rbRed
				x = x.parent
			} else {
				if w.right.color == rbBlack {
					// Case 3—兄弟右子節點為黑：旋轉對齊 Case 4
					w.left.color = rbBlack
					w.color = rbRed
					t.rightRotate(w)
					w = x.parent.right
				}
				// Case 4—兄弟右子節點為紅：單次左旋，結束
				w.color = x.parent.color
				x.parent.color = rbBlack
				w.right.color = rbBlack
				t.leftRotate(x.parent)
				x = t.root
			}
		} else {
			// 對稱情況
			w := x.parent.left
			if w.color == rbRed {
				w.color = rbBlack
				x.parent.color = rbRed
				t.rightRotate(x.parent)
				w = x.parent.left
			}
			if w.right.color == rbBlack && w.left.color == rbBlack {
				w.color = rbRed
				x = x.parent
			} else {
				if w.left.color == rbBlack {
					w.right.color = rbBlack
					w.color = rbRed
					t.leftRotate(w)
					w = x.parent.left
				}
				w.color = x.parent.color
				x.parent.color = rbBlack
				w.left.color = rbBlack
				t.rightRotate(x.parent)
				x = t.root
			}
		}
	}
	x.color = rbBlack
}

// ── 查詢 ──────────────────────────────────────────────────────────────────────

// Search 返回鍵值等於 key 的節點，找不到時返回 nil。
// 複雜度：O(log n)。
func (t *rbTree[V]) Search(key int64) *rbNode[V] {
	x := t.root
	for x != t.nil_ {
		switch {
		case key == x.key:
			return x
		case key < x.key:
			x = x.left
		default:
			x = x.right
		}
	}
	return nil
}

// Min 返回鍵值最小的節點（賣盤的最佳賣價）。
// 複雜度：O(log n)—沿左脊柱走訪，無快取指針。
func (t *rbTree[V]) Min() *rbNode[V] {
	if t.root == t.nil_ {
		return nil
	}
	return t.treeMinimum(t.root)
}

// Max 返回鍵值最大的節點（買盤的最佳買價）。
// 複雜度：O(log n)—沿右脊柱走訪，無快取指針。
func (t *rbTree[V]) Max() *rbNode[V] {
	if t.root == t.nil_ {
		return nil
	}
	x := t.root
	for x.right != t.nil_ {
		x = x.right
	}
	return x
}

// Len 返回樹中目前的價位數量。
func (t *rbTree[V]) Len() int { return t.size }

// inorderAppend 以遞迴方式將節點按鍵值升序附加到 result。
func (t *rbTree[V]) inorderAppend(x *rbNode[V], result *[]*rbNode[V]) {
	if x == t.nil_ {
		return
	}
	t.inorderAppend(x.left, result)
	*result = append(*result, x)
	t.inorderAppend(x.right, result)
}

// InorderSlice 返回所有節點按鍵值升序排列的切片。
// 用於 GetL2Snapshot—O(n)，不需額外排序。
func (t *rbTree[V]) InorderSlice() []*rbNode[V] {
	nodes := make([]*rbNode[V], 0, t.size)
	t.inorderAppend(t.root, &nodes)
	return nodes
}
