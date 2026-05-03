from PIL import Image, ImageDraw, ImageFont
import math

W, H = 1240, 820
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK   = (0, 0, 0)
GRAY    = (160, 160, 160)
LGRAY   = (235, 235, 235)
DGRAY   = (90, 90, 90)
RED     = (200, 50, 50)
LRED    = (255, 220, 220)
DRED    = (160, 30, 30)
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
    f10 = ImageFont.truetype(CJK_REG, 10)
    f11 = ImageFont.truetype(CJK_REG, 11)
    f12 = ImageFont.truetype(CJK_REG, 12)
    f13 = ImageFont.truetype(CJK_REG, 13)
    f14b = ImageFont.truetype(CJK_BLD, 14)
    f16b = ImageFont.truetype(CJK_BLD, 16)
    f18b = ImageFont.truetype(CJK_BLD, 18)
    fmono = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 11)
    fmonob = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", 11)
except:
    f10 = f11 = f12 = f13 = f14b = f16b = f18b = fmono = fmonob = ImageFont.load_default()


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


def node(cx, cy, label, color, r=22, label_color=WHITE, highlight=False):
    if highlight:
        # outer ring to mark "the changing node"
        draw.ellipse([cx - r - 4, cy - r - 4, cx + r + 4, cy + r + 4],
                     outline=ORANGE, width=3)
    draw.ellipse([cx - r, cy - r, cx + r, cy + r], fill=color, outline=BLACK, width=2)
    tc(label, cx, cy, fmonob, label_color)


def nil_leaf(cx, cy):
    s = 8
    draw.rectangle([cx - s, cy - s, cx + s, cy + s], fill=BLACK, outline=BLACK)


def edge(x1, y1, x2, y2, r=22):
    # connect two node circles, stopping at their boundary
    ang = math.atan2(y2 - y1, x2 - x1)
    sx = x1 + r * math.cos(ang)
    sy = y1 + r * math.sin(ang)
    ex = x2 - r * math.cos(ang)
    ey = y2 - r * math.sin(ang)
    draw.line([sx, sy, ex, ey], fill=BLACK, width=1)


def edge_to_nil(x1, y1, x2, y2, r=22):
    ang = math.atan2(y2 - y1, x2 - x1)
    sx = x1 + r * math.cos(ang)
    sy = y1 + r * math.sin(ang)
    draw.line([sx, sy, x2, y2], fill=BLACK, width=1)


# ── Title ─────────────────────────────────────────────────────────────────
tc("Red-Black Tree 訂單簿：Insert 為何造成 Latency Jitter", W // 2, 26, f18b)
tc("以 Sell Book（價格升序）為例，示範新增價位 100.06 後的 rebalance 過程", W // 2, 50, f12, DGRAY)


# ════════════════════════════════════════════════════════════════════════════
# Step 1: Initial balanced RB-Tree
# ════════════════════════════════════════════════════════════════════════════
PW = (W - 80) // 3
P1X = 20
P2X = P1X + PW + 20
P3X = P2X + PW + 20
PY = 80
PH = 660

# panel 1
draw.rectangle([P1X, PY, P1X + PW, PY + PH], fill=WHITE, outline=BLACK, width=2)
draw.rectangle([P1X, PY, P1X + PW, PY + 32], fill=BLUE, outline=BLACK, width=2)
tc("Step 1：原始平衡狀態", P1X + PW // 2, PY + 16, f16b, WHITE)

# Subtitle
tl("黑高度 BH=2，所有性質滿足", P1X + 20, PY + 42, f12, DGRAY)

# Tree layout for panel 1 (6 nodes)
def draw_tree_state(ox, oy, nodes_def, edges_def, hilite=None):
    """nodes_def: list of (key, x_offset, y_offset, color, hilight)"""
    # First draw edges
    for (a, b) in edges_def:
        ax, ay = nodes_def[a][1] + ox, nodes_def[a][2] + oy
        bx, by = nodes_def[b][1] + ox, nodes_def[b][2] + oy
        edge(ax, ay, bx, by)
    # Then draw nodes
    for i, (lbl, dx, dy, c, hi) in enumerate(nodes_def):
        node(dx + ox, dy + oy, lbl, c, highlight=hi)


# Step 1 tree (balanced, ~6 nodes)
# Layout (relative offsets):
#                100.10 (B) [root]
#         100.08 (R)        100.13 (R)
#     100.07(B) 100.09(B) 100.11(B) 100.15(B)
TREE_OX = P1X + PW // 2
TREE_OY = PY + 90
LV1_DY = 60
LV2_DY = 130
LV3_DY = 200

# Step 1 must match Step 2's pre-insert state for narrative coherence
t1_nodes = [
    ("100.10", 0,    0,        BLACK, False),  # 0 root
    ("100.08", -100, LV1_DY,   BLACK, False),  # 1
    ("100.13", 100,  LV1_DY,   BLACK, False),  # 2
    ("100.07", -160, LV2_DY,   RED,   False),  # 3
    ("100.09", -40,  LV2_DY,   RED,   False),  # 4
    ("100.11", 40,   LV2_DY,   RED,   False),  # 5
    ("100.15", 160,  LV2_DY,   RED,   False),  # 6
]
t1_edges = [(0, 1), (0, 2), (1, 3), (1, 4), (2, 5), (2, 6)]

draw_tree_state(TREE_OX, TREE_OY, t1_nodes, t1_edges)

# best ask annotation — anchored inside the panel (use absolute panel coords)
left_x = TREE_OX + t1_nodes[3][1]
left_y = TREE_OY + t1_nodes[3][2]
ann_y = left_y + 80
ann_x = max(P1X + 80, left_x)
arrow(ann_x, ann_y - 14, left_x, left_y + 24, GREEN, 2, 6)
tc("best ask = leftmost", ann_x, ann_y, f12, GREEN)
tc("(走 left spine 找最小價)", ann_x, ann_y + 16, f11, GREEN)

# legend
lg_y = PY + PH - 90
tl("圖例：", P1X + 20, lg_y, f12, DGRAY)
draw.ellipse([P1X + 60, lg_y - 2, P1X + 78, lg_y + 16], fill=BLACK, outline=BLACK)
tl("黑節點", P1X + 84, lg_y, f12)
draw.ellipse([P1X + 140, lg_y - 2, P1X + 158, lg_y + 16], fill=RED, outline=BLACK)
tl("紅節點", P1X + 164, lg_y, f12)
nil_leaf(P1X + 230, lg_y + 7)
tl("NIL（黑）", P1X + 244, lg_y, f12)

# Properties summary at bottom
prop_y = lg_y + 28
draw.rectangle([P1X + 14, prop_y, P1X + PW - 14, PY + PH - 14], fill=LGRAY, outline=GRAY, width=1)
tl("✓ Root 為黑", P1X + 22, prop_y + 6, f11, DGRAY)
tl("✓ 紅節點不相鄰", P1X + 22, prop_y + 22, f11, DGRAY)
tl("✓ 黑高度恆定（每條 root→NIL 路徑黑節點數相同）", P1X + 22, prop_y + 38, f11, DGRAY)


# ════════════════════════════════════════════════════════════════════════════
# Step 2: After BST insert — violation detected
# ════════════════════════════════════════════════════════════════════════════
draw.rectangle([P2X, PY, P2X + PW, PY + PH], fill=WHITE, outline=BLACK, width=2)
draw.rectangle([P2X, PY, P2X + PW, PY + 32], fill=ORANGE, outline=BLACK, width=2)
tc("Step 2：BST 插入後 → 違反性質", P2X + PW // 2, PY + 16, f16b, WHITE)

tl("插入新價位 100.06（染紅）→ 發現 parent 也是紅！", P2X + 20, PY + 42, f12, DRED)

TREE_OX2 = P2X + PW // 2
# Same as t1 but add 100.06 as left child of 100.07
t2_nodes = [
    ("100.10", 0,    0,        BLACK, False),  # 0
    ("100.08", -100, LV1_DY,   RED,   True),   # 1 parent (red, violation)
    ("100.13", 100,  LV1_DY,   RED,   False),  # 2 uncle (red!)
    ("100.07", -160, LV2_DY,   BLACK, False),  # 3
    ("100.09", -40,  LV2_DY,   BLACK, False),  # 4
    ("100.11", 40,   LV2_DY,   BLACK, False),  # 5
    ("100.15", 160,  LV2_DY,   BLACK, False),  # 6
    ("100.06", -200, LV3_DY,   RED,   True),   # 7 NEW — violation
]
t2_edges = [(0, 1), (0, 2), (1, 3), (1, 4), (2, 5), (2, 6), (3, 7)]

draw_tree_state(TREE_OX2, TREE_OY, t2_nodes, t2_edges)

# Annotation: violation arrow
new_x = TREE_OX2 + t2_nodes[7][1]
new_y = TREE_OY + t2_nodes[7][2]
parent_x = TREE_OX2 + t2_nodes[3][1]
parent_y = TREE_OY + t2_nodes[3][2]

# Mark "two reds in a row" -- but here, parent is BLACK, grandparent is RED
# Wait — let me re-think: 100.06 inserted under 100.07 (BLACK). 100.07 is BLACK.
# So no violation. Let me adjust: insert under 100.07's left child... but it has none.
# Need insertion point where parent is RED.
# Better: insert 100.085 under 100.08 directly... no, 100.08 has children.
# Re-design: simpler tree where insertion violates.

# Actually let me redesign with a 5-node tree where insertion under a red node causes violation.

# REDESIGN — clear out and use new layout
# Tree:
#         100.10 (B)
#      100.08 (B)  100.13 (B)
#    100.07 (R)
# Insert 100.06 → child of 100.07 → both red → violation

# Cover and redraw step 2 area
draw.rectangle([P2X + 1, PY + 33, P2X + PW - 1, PY + PH - 1], fill=WHITE)

tl("插入新價位 100.06（染紅）→ 發現 parent 100.07 也是紅！", P2X + 20, PY + 42, f12, DRED)

t2b_nodes = [
    ("100.10", 0,    0,        BLACK, False),    # 0 root
    ("100.08", -100, LV1_DY,   BLACK, False),    # 1
    ("100.13", 100,  LV1_DY,   BLACK, False),    # 2 uncle of new node? -- NO, see below
    ("100.07", -160, LV2_DY,   RED,   True),     # 3 parent (RED)
    ("100.09", -40,  LV2_DY,   RED,   False),    # 4
    ("100.11", 40,   LV2_DY,   RED,   False),    # 5
    ("100.15", 160,  LV2_DY,   RED,   False),    # 6
    ("100.06", -200, LV3_DY,   RED,   True),     # 7 NEW
]
t2b_edges = [(0, 1), (0, 2), (1, 3), (1, 4), (2, 5), (2, 6), (3, 7)]

draw_tree_state(TREE_OX2, TREE_OY, t2b_nodes, t2b_edges)

# Annotation lines: red-red violation
nx = TREE_OX2 + t2b_nodes[7][1]
ny = TREE_OY + t2b_nodes[7][2]
px = TREE_OX2 + t2b_nodes[3][1]
py_ = TREE_OY + t2b_nodes[3][2]

# Red dashed bracket between parent and new node
def dashed_line(x1, y1, x2, y2, color, dash=6):
    length = math.hypot(x2 - x1, y2 - y1)
    if length == 0:
        return
    steps = max(1, int(length / dash))
    for s in range(steps):
        t1 = s / steps
        t2 = (s + 0.5) / steps
        draw.line([x1 + (x2 - x1) * t1, y1 + (y2 - y1) * t1,
                   x1 + (x2 - x1) * t2, y1 + (y2 - y1) * t2], fill=color, width=2)

# uncle annotation — placed below uncle node
ux = TREE_OX2 + t2b_nodes[4][1]
uy = TREE_OY + t2b_nodes[4][2]
tc("uncle (RED)", ux, uy + 36, f11, DRED)

# parent annotation — placed above parent node, far to the left
parent_label_x = TREE_OX2 + t2b_nodes[3][1]
parent_label_y = TREE_OY + t2b_nodes[3][2]
tc("parent (RED)", parent_label_x, parent_label_y - 38, f11, DRED)

# Violation label — placed below the new node
viol_x = nx - 70
viol_y = ny + 36
draw.rectangle([viol_x - 4, viol_y - 2, viol_x + 144, viol_y + 32], fill=LRED, outline=DRED, width=2)
tl("⚠ Red-Red 違反", viol_x + 6, viol_y + 2, f12, DRED)
tl("(性質 4)", viol_x + 6, viol_y + 18, f11, DRED)
arrow(viol_x + 70, viol_y - 2, nx, ny + 24, DRED, 2, 6)

# Bottom analysis
ana_y = lg_y
draw.rectangle([P2X + 14, ana_y, P2X + PW - 14, PY + PH - 14], fill=LORANGE, outline=ORANGE, width=1)
tl("Fix-up 必須處理：", P2X + 22, ana_y + 6, f12, DRED)
tl("• Uncle 100.09 是紅 → 套用 Case 1：重染色", P2X + 22, ana_y + 24, f11, BLACK)
tl("  parent + uncle 變黑、grandparent 變紅", P2X + 22, ana_y + 40, f11, BLACK)
tl("• 違反向上推到 grandparent，可能再次觸發", P2X + 22, ana_y + 56, f11, BLACK)
tl("• 最壞情況：fix-up 一路傳到 root（O(log n)）", P2X + 22, ana_y + 72, f11, BLACK)


# ════════════════════════════════════════════════════════════════════════════
# Step 3: After rebalance
# ════════════════════════════════════════════════════════════════════════════
draw.rectangle([P3X, PY, P3X + PW, PY + PH], fill=WHITE, outline=BLACK, width=2)
draw.rectangle([P3X, PY, P3X + PW, PY + 32], fill=GREEN, outline=BLACK, width=2)
tc("Step 3：Rebalance 完成", P3X + PW // 2, PY + 16, f16b, WHITE)

tl("Case 1 重染色 + 上推：root 維持黑，黑高度恢復", P3X + 20, PY + 42, f12, GREEN)

TREE_OX3 = P3X + PW // 2

# After fix-up:
# 100.07 → BLACK (was RED)
# 100.09 → BLACK (was RED)
# 100.08 → RED (was BLACK)
# But root grandparent of new node was 100.08 (BLACK). After recolor, 100.08 becomes RED.
# Now check 100.08 (RED) vs its parent 100.10 (BLACK) — OK, no violation.
# Final tree.

t3_nodes = [
    ("100.10", 0,    0,        BLACK, False),    # 0
    ("100.08", -100, LV1_DY,   RED,   True),     # 1 newly RED
    ("100.13", 100,  LV1_DY,   BLACK, False),    # 2
    ("100.07", -160, LV2_DY,   BLACK, True),     # 3 newly BLACK
    ("100.09", -40,  LV2_DY,   BLACK, True),     # 4 newly BLACK
    ("100.11", 40,   LV2_DY,   RED,   False),    # 5
    ("100.15", 160,  LV2_DY,   RED,   False),    # 6
    ("100.06", -200, LV3_DY,   RED,   False),    # 7
]
t3_edges = [(0, 1), (0, 2), (1, 3), (1, 4), (2, 5), (2, 6), (3, 7)]

draw_tree_state(TREE_OX3, TREE_OY, t3_nodes, t3_edges)

# Highlight which 3 nodes flipped color
flip_x = TREE_OX3 - 220
flip_y = TREE_OY + 30
tc("✓ 已修正", flip_x, flip_y, f12, GREEN)

# Cost annotation at bottom
cost_y = lg_y
draw.rectangle([P3X + 14, cost_y, P3X + PW - 14, PY + PH - 14], fill=LGREEN, outline=GREEN, width=1)
tl("這次 Insert 的實際成本：", P3X + 22, cost_y + 6, f12, DRED)
tl("• 3 個節點重染色 → 3 次記憶體寫入", P3X + 22, cost_y + 24, f11, BLACK)
tl("• 節點散落在 heap → 每次 ~100 cycles cache miss", P3X + 22, cost_y + 40, f11, BLACK)
tl("• Fix-up if-else 分支多 → 分支預測失敗", P3X + 22, cost_y + 56, f11, BLACK)
tl("⚠ 平均 OK，但 P99.99 出現微秒級尖峰", P3X + 22, cost_y + 72, f11, DRED)


# ── Caption ───────────────────────────────────────────────────────────────
caption = "Figure 13.26: Red-Black Tree 在 Order Book 中的 Insert 與 Rebalance"
tc(caption, W // 2, H - 18, f14b)

out = "/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_26.webp"
img.save(out, "WEBP", quality=92)
print(f"Saved: {out}  ({W}x{H})")
