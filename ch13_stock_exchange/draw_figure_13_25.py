from PIL import Image, ImageDraw, ImageFont
import math

W, H = 1180, 880
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK   = (0, 0, 0)
GRAY    = (160, 160, 160)
LGRAY   = (235, 235, 235)
DGRAY   = (90, 90, 90)
RED     = (200, 50, 50)
LRED    = (255, 220, 220)
BLUE    = (50, 80, 180)
LBLUE   = (220, 232, 255)
GREEN   = (40, 130, 70)
LGREEN  = (215, 240, 220)
ORANGE  = (210, 120, 30)
LORANGE = (255, 235, 200)
WHITE   = (255, 255, 255)

CJK_REG = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
CJK_BLD = "/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc"
try:
    f11 = ImageFont.truetype(CJK_REG, 11)
    f12 = ImageFont.truetype(CJK_REG, 12)
    f13 = ImageFont.truetype(CJK_REG, 13)
    f14b = ImageFont.truetype(CJK_BLD, 14)
    f16b = ImageFont.truetype(CJK_BLD, 16)
    f18b = ImageFont.truetype(CJK_BLD, 18)
    fmono = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 12)
    fmonob = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", 12)
except:
    f11 = f12 = f13 = f14b = f16b = f18b = fmono = fmonob = ImageFont.load_default()


def tc(txt, cx, cy, fnt=f13, color=BLACK):
    bb = draw.textbbox((0, 0), txt, font=fnt)
    tw, th = bb[2] - bb[0], bb[3] - bb[1]
    draw.text((cx - tw // 2, cy - th // 2), txt, fill=color, font=fnt)


def tl(txt, x, y, fnt=f13, color=BLACK):
    draw.text((x, y), txt, fill=color, font=fnt)


def arrow(x1, y1, x2, y2, color=BLACK, width=1, head=6):
    draw.line([x1, y1, x2, y2], fill=color, width=width)
    ang = math.atan2(y2 - y1, x2 - x1)
    p1 = (x2 - head * math.cos(ang - math.pi / 7), y2 - head * math.sin(ang - math.pi / 7))
    p2 = (x2 - head * math.cos(ang + math.pi / 7), y2 - head * math.sin(ang + math.pi / 7))
    draw.polygon([(x2, y2), p1, p2], fill=color)


def panel(x, y, w, h, title, accent):
    draw.rectangle([x, y, x + w, y + h], fill=WHITE, outline=BLACK, width=2)
    draw.rectangle([x, y, x + w, y + 32], fill=accent, outline=BLACK, width=2)
    tc(title, x + w // 2, y + 16, f16b, WHITE)


# ── Title ─────────────────────────────────────────────────────────────────
tc("Order Book 資料結構選型比較", W // 2, 26, f18b)
tc("Insert / Cancel / Best-Price 操作的形貌與複雜度", W // 2, 50, f13, DGRAY)

# Layout: 2 x 2 grid
PAD = 24
PW = (W - PAD * 3) // 2
PH = 360
PY1 = 76
PY2 = PY1 + PH + PAD
PX1 = PAD
PX2 = PAD * 2 + PW

# ════════════════════════════════════════════════════════════════════════════
# Quadrant 1 (top-left): HashMap + DoublyLinkedList   ★ 本章採用
# ════════════════════════════════════════════════════════════════════════════
panel(PX1, PY1, PW, PH, "① HashMap + DoublyLinkedList   ★ 本章採用", BLUE)

bx, by = PX1 + 20, PY1 + 50
tl("HashMap<Price, PriceLevel>", bx, by, f14b)
tl("Insert/Cancel/Match: O(1)   ·   Best price: O(1) (cache pointer)", bx, by + 18, f12, DGRAY)

# Draw HashMap buckets (3 prices), each bucket → linked list nodes
hm_x = bx + 10
hm_y = by + 50
KEY_W, KEY_H = 80, 28
keys = [("100.12", LBLUE), ("100.11", LBLUE), ("100.10", LRED)]   # last = best ask
for i, (k, c) in enumerate(keys):
    yy = hm_y + i * (KEY_H + 6)
    draw.rectangle([hm_x, yy, hm_x + KEY_W, yy + KEY_H], fill=c, outline=BLUE, width=2)
    tc(k, hm_x + KEY_W // 2, yy + KEY_H // 2, fmonob, BLUE)

# linked list cells per bucket
NW, NH = 56, 28
ll_xs = hm_x + KEY_W + 28
node_lists = [
    ["600", "900"],
    ["400", "700"],
    ["200", "400", "1100", "100"],
]
best_ask_y = None
for i, lst in enumerate(node_lists):
    yy = hm_y + i * (KEY_H + 6)
    cx = ll_xs
    # arrow from key to first node
    arrow(hm_x + KEY_W + 2, yy + KEY_H // 2, cx - 2, yy + KEY_H // 2, BLUE, 1, 5)
    for j, q in enumerate(lst):
        outline = RED if i == 2 else GRAY
        fill = LRED if i == 2 else WHITE
        draw.rectangle([cx, yy, cx + NW, yy + NH], fill=fill, outline=outline, width=2 if i == 2 else 1)
        tc(q, cx + NW // 2, yy + NH // 2, fmono, RED if i == 2 else BLACK)
        if j < len(lst) - 1:
            # double arrow (doubly linked)
            arrow(cx + NW, yy + NH // 2 - 4, cx + NW + 18, yy + NH // 2 - 4, GRAY, 1, 4)
            arrow(cx + NW + 18, yy + NH // 2 + 4, cx + NW, yy + NH // 2 + 4, GRAY, 1, 4)
        cx += NW + 18
    if i == 2:
        best_ask_y = yy + NH // 2

# Best-ask pointer (above the row to avoid overlap)
tl("↑ best_ask", hm_x + KEY_W + 4, best_ask_y + NH // 2 + 4, f11, RED)

# Pros/cons box
pc_y = PY1 + PH - 90
draw.rectangle([PX1 + 16, pc_y, PX1 + PW - 16, PY1 + PH - 14], fill=LGRAY, outline=GRAY, width=1)
tl("✓ 真正的常數時間（無 rebalance、無 log n）", PX1 + 24, pc_y + 8, f12, GREEN)
tl("✓ Latency Determinism 佳，P99.99 平穩", PX1 + 24, pc_y + 26, f12, GREEN)
tl("✓ OrderMap 可 O(1) 取消任意訂單", PX1 + 24, pc_y + 44, f12, GREEN)
tl("△ 跨價位掃描需走 HashMap key（價格分散時較弱）", PX1 + 24, pc_y + 62, f12, ORANGE)


# ════════════════════════════════════════════════════════════════════════════
# Quadrant 2 (top-right): Red-Black Tree
# ════════════════════════════════════════════════════════════════════════════
panel(PX2, PY1, PW, PH, "② Red-Black Tree（傳統做法）", RED)

bx, by = PX2 + 20, PY1 + 50
tl("Tree<Price → PriceLevel>", bx, by, f14b)
tl("Insert/Cancel: O(log n)   ·   Best price: O(log n) 或 O(1) cached", bx, by + 18, f12, DGRAY)

# tree nodes
def tnode(cx, cy, label, color):
    r = 22
    draw.ellipse([cx - r, cy - r, cx + r, cy + r], fill=color, outline=BLACK, width=2)
    tc(label, cx, cy, fmonob, WHITE)

# layout: root, two children, four grandchildren
root_x = PX2 + PW // 2
root_y = by + 70
tnode(root_x, root_y, "100.11", BLACK)

c1x, c1y = root_x - 110, root_y + 70
c2x, c2y = root_x + 110, root_y + 70
tnode(c1x, c1y, "100.09", RED)
tnode(c2x, c2y, "100.13", RED)
draw.line([root_x, root_y + 22, c1x, c1y - 22], fill=BLACK, width=1)
draw.line([root_x, root_y + 22, c2x, c2y - 22], fill=BLACK, width=1)

g_xs = [c1x - 55, c1x + 55, c2x - 55, c2x + 55]
g_lbl = ["100.08", "100.10", "100.12", "100.14"]
for gx, lbl, par in zip(g_xs, g_lbl, [(c1x, c1y), (c1x, c1y), (c2x, c2y), (c2x, c2y)]):
    tnode(gx, c1y + 70, lbl, BLACK)
    draw.line([par[0], par[1] + 22, gx, c1y + 70 - 22], fill=BLACK, width=1)

# rotate annotation
arr_x = root_x + 140
arr_y = root_y + 90
draw.arc([arr_x - 26, arr_y - 26, arr_x + 26, arr_y + 26], start=0, end=270, fill=ORANGE, width=2)
arrow(arr_x + 18, arr_y - 18, arr_x + 26, arr_y - 6, ORANGE, 2, 5)
tl("rebalance", arr_x + 30, arr_y - 8, f12, ORANGE)

pc_y = PY1 + PH - 90
draw.rectangle([PX2 + 16, pc_y, PX2 + PW - 16, PY1 + PH - 14], fill=LGRAY, outline=GRAY, width=1)
tl("✓ 有序，跨價位 range scan 較自然", PX2 + 24, pc_y + 8, f12, GREEN)
tl("✗ Rebalance（旋轉節點）造成 cache miss & latency jitter", PX2 + 24, pc_y + 26, f12, RED)
tl("✗ 節點分散在 heap，cache locality 差", PX2 + 24, pc_y + 44, f12, RED)
tl("△ O(log n) 在高頻撮合下累積差距明顯", PX2 + 24, pc_y + 62, f12, ORANGE)


# ════════════════════════════════════════════════════════════════════════════
# Quadrant 3 (bottom-left): Skip List
# ════════════════════════════════════════════════════════════════════════════
panel(PX1, PY2, PW, PH, "③ Skip List（加密貨幣 / 期貨適合）", GREEN)

bx, by = PX1 + 20, PY2 + 50
tl("多層 Linked List with Skip Pointers", bx, by, f14b)
tl("Insert/Cancel: O(log n) 期望   ·   Best price: O(1) (head)", bx, by + 18, f12, DGRAY)

# 4 levels, 6 base nodes
sl_x0 = bx + 10
sl_y0 = by + 60
LEVELS = 4
NODES = ["100.08", "100.09", "100.10", "100.11", "100.12", "100.13"]
NW2, NH2 = 56, 22
GAP = 14
node_xs = [sl_x0 + i * (NW2 + GAP) for i in range(len(NODES))]
# Each level has presence pattern (top sparse, bottom = all)
present = [
    [1, 0, 0, 0, 0, 1],   # L3 (top)
    [1, 0, 1, 0, 0, 1],   # L2
    [1, 0, 1, 1, 0, 1],   # L1
    [1, 1, 1, 1, 1, 1],   # L0 (bottom = full list)
]
for li in range(LEVELS):
    yy = sl_y0 + li * (NH2 + 8)
    tl(f"L{LEVELS - 1 - li}", sl_x0 - 22, yy + 4, f11, DGRAY)
    last_x = None
    for i, has in enumerate(present[li]):
        if has:
            xx = node_xs[i]
            fill = LGREEN if li == LEVELS - 1 else WHITE
            draw.rectangle([xx, yy, xx + NW2, yy + NH2], fill=fill, outline=GREEN, width=1)
            tc(NODES[i], xx + NW2 // 2, yy + NH2 // 2, fmono, BLACK)
            if last_x is not None:
                draw.line([last_x, yy + NH2 // 2, xx, yy + NH2 // 2], fill=GREEN, width=1)
                arrow(last_x, yy + NH2 // 2, xx, yy + NH2 // 2, GREEN, 1, 4)
            last_x = xx + NW2
# Search arrow from top → drilling down example
sx, sy = sl_x0 - 50, sl_y0 + LEVELS * (NH2 + 8) + 6
tl("search 100.11 → drill down levels", sl_x0, sy, f11, DGRAY)

pc_y = PY2 + PH - 90
draw.rectangle([PX1 + 16, pc_y, PX1 + PW - 16, PY2 + PH - 14], fill=LGRAY, outline=GRAY, width=1)
tl("✓ 無 rebalance，操作路徑平穩", PX1 + 24, pc_y + 8, f12, GREEN)
tl("✓ Cache locality 比 RB-Tree 好", PX1 + 24, pc_y + 26, f12, GREEN)
tl("✓ 實作比紅黑樹簡單", PX1 + 24, pc_y + 44, f12, GREEN)
tl("△ 仍是 O(log n)；極致場景輸給 HashMap 與 Array", PX1 + 24, pc_y + 62, f12, ORANGE)


# ════════════════════════════════════════════════════════════════════════════
# Quadrant 4 (bottom-right): Array-based
# ════════════════════════════════════════════════════════════════════════════
panel(PX2, PY2, PW, PH, "④ Array-based（極致 HFT、單一商品）", ORANGE)

bx, by = PX2 + 20, PY2 + 50
tl("PriceLevel[] book   //   index = (price - base) / tick", bx, by, f14b)
tl("Insert/Cancel: O(1)   ·   Best price: O(1)   ·   ★ Cache 最佳", bx, by + 18, f12, DGRAY)

# Draw array with 10 cells
arr_x = bx + 10
arr_y = by + 60
CELL_W, CELL_H = 53, 36
# index labels and prices
prices = ["100.05", "100.06", "100.07", "100.08", "100.09",
          "100.10", "100.11", "100.12", "100.13", "100.14"]
qtys   = ["", "100", "200", "1100", "", "1700", "900", "1500", "", "200"]
best_idx = 5  # 100.10 = best ask

for i, (p, q) in enumerate(zip(prices, qtys)):
    xx = arr_x + i * CELL_W
    is_empty = q == ""
    if is_empty:
        fill = (245, 245, 245)
        outline = GRAY
        text_color = GRAY
    elif i == best_idx:
        fill = LORANGE
        outline = ORANGE
        text_color = ORANGE
    else:
        fill = WHITE
        outline = ORANGE
        text_color = BLACK
    draw.rectangle([xx, arr_y, xx + CELL_W, arr_y + CELL_H], fill=fill, outline=outline,
                   width=2 if i == best_idx else 1)
    # index on top
    tc(f"[{i}]", xx + CELL_W // 2, arr_y + 8, f11, DGRAY)
    tc(p, xx + CELL_W // 2, arr_y + 22, fmono, text_color)

# qty row below
qty_y = arr_y + CELL_H + 4
for i, q in enumerate(qtys):
    xx = arr_x + i * CELL_W
    if q != "":
        text_color = ORANGE if i == best_idx else BLACK
        tc(f"qty:{q}", xx + CELL_W // 2, qty_y + 10, f11, text_color)
    else:
        tc("∅", xx + CELL_W // 2, qty_y + 10, f12, GRAY)

# Best-ask annotation (place below the array to avoid overlapping with subtitle)
ba_y = arr_y + CELL_H + qty_y - arr_y + 18
ann_x = arr_x + best_idx * CELL_W + CELL_W // 2
arrow(ann_x, qty_y + 28, ann_x, qty_y + 22, ORANGE, 2, 5)
tc("↑ best ask", ann_x, qty_y + 34, f11, ORANGE)

# Memory layout note
mem_y = qty_y + 56
tl("記憶體連續排列 → CPU prefetcher 100% 命中", arr_x, mem_y, f12, DGRAY)
draw.rectangle([arr_x, mem_y + 22, arr_x + 10 * CELL_W, mem_y + 32], fill=LORANGE, outline=ORANGE, width=1)
tc("contiguous memory", arr_x + 5 * CELL_W, mem_y + 27, f11, ORANGE)

pc_y = PY2 + PH - 90
draw.rectangle([PX2 + 16, pc_y, PX2 + PW - 16, PY2 + PH - 14], fill=LGRAY, outline=GRAY, width=1)
tl("✓ 真正的 O(1)，且 CPU cache 最友善", PX2 + 24, pc_y + 8, f12, GREEN)
tl("✓ 適合單一商品、tick 固定的 HFT 系統", PX2 + 24, pc_y + 26, f12, GREEN)
tl("✗ 價格範圍大 → 記憶體浪費", PX2 + 24, pc_y + 44, f12, RED)
tl("✗ 需處理稀疏空槽（多數價位常時為空）", PX2 + 24, pc_y + 62, f12, RED)


# ── Caption ───────────────────────────────────────────────────────────────
caption = "Figure 13.25: 訂單簿四種主流資料結構的形貌與權衡"
tc(caption, W // 2, H - 20, f14b)

out = "/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_25.webp"
img.save(out, "WEBP", quality=92)
print(f"Saved: {out}  ({W}x{H})")
