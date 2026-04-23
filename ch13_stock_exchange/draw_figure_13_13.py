from PIL import Image, ImageDraw, ImageFont

W, H = 860, 720
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK  = (0, 0, 0)
GRAY   = (160, 160, 160)
LGRAY  = (230, 230, 230)
RED    = (200, 50, 50)
BLUE   = (50, 80, 180)
FILL_W = (255, 255, 255)

try:
    f12 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 12)
    f13 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    f14 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    f15 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    f16 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 16)
    fmono = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 13)
except:
    f12 = f13 = f14 = f15 = f16 = fmono = ImageFont.load_default()

def tc(txt, cx, cy, fnt=f13, color=BLACK):
    bb = draw.textbbox((0,0), txt, font=fnt)
    tw, th = bb[2]-bb[0], bb[3]-bb[1]
    draw.text((cx-tw//2, cy-th//2), txt, fill=color, font=fnt)

def tl(txt, x, y, fnt=f13, color=BLACK):
    draw.text((x, y), txt, fill=color, font=fnt)

def cell(x, y, w, h, txt, fill=FILL_W, outline=BLACK, fnt=f13, color=BLACK, bold_outline=False):
    lw = 2 if bold_outline else 1
    draw.rectangle([x, y, x+w, y+h], fill=fill, outline=outline, width=lw)
    tc(txt, x+w//2, y+h//2, fnt, color)
    return x+w  # next x

# ── outer dashed border ───────────────────────────────────────────────────
def dashed_rect(x1, y1, x2, y2, dash=8, color=BLACK):
    for i, (ax,ay,bx,by) in enumerate([
        (x1,y1,x2,y1),(x2,y1,x2,y2),(x2,y2,x1,y2),(x1,y2,x1,y1)]):
        import math
        length = math.hypot(bx-ax, by-ay)
        steps = int(length/dash)
        for s in range(steps):
            t1,t2 = s/steps, (s+0.45)/steps
            draw.line([ax+(bx-ax)*t1, ay+(by-ay)*t1,
                       ax+(bx-ax)*t2, ay+(by-ay)*t2], fill=color, width=1)

# ── Layout constants ──────────────────────────────────────────────────────
OX, OY = 80, 60      # outer box origin
OW, OH = 680, 490    # outer box size
RH = 30              # row height
CW_LABEL = 120       # label column (e.g. "depth of ask")
CW_PRICE = 70        # price column
CW_QTY   = 65        # each quantity cell

# Title
draw.rectangle([OX, OY, OX+OW, OY+30], fill=LGRAY, outline=BLACK, width=2)
tc("APPLE stock", OX+OW//2, OY+15, f16)

# ── Section dividers ──────────────────────────────────────────────────────
SELL_Y = OY+30          # sell book starts
SELL_H = 4*RH + 25      # header(25) + 4 rows
BUY_Y  = SELL_Y+SELL_H
BUY_H  = 4*RH + 25

# outer box
draw.rectangle([OX, OY, OX+OW, OY+OH], fill=FILL_W, outline=BLACK, width=2)
draw.rectangle([OX, OY, OX+OW, OY+30], fill=LGRAY, outline=BLACK, width=2)

# Column header row (shared)
def col_header_row(start_y, label_section):
    # "Sell book" label on left, then Price, Quantity header
    HDR_Y = start_y
    HDR_H = 25
    draw.line([OX, HDR_Y+HDR_H, OX+OW, HDR_Y+HDR_H], fill=BLACK, width=1)
    tc("Price",    OX+CW_LABEL+CW_PRICE//2,         HDR_Y+HDR_H//2, f14)
    tc("Quantity", OX+CW_LABEL+CW_PRICE+2*CW_QTY,  HDR_Y+HDR_H//2, f14)

# ── Dashed inner box for sell ─────────────────────────────────────────────
S_INNER_X = OX+10
S_INNER_Y = SELL_Y
S_INNER_W = OW-20
S_INNER_H = SELL_H
dashed_rect(S_INNER_X, S_INNER_Y, S_INNER_X+S_INNER_W, S_INNER_Y+S_INNER_H)

# ── Sell book section label ───────────────────────────────────────────────
tc("Sell book", OX-42, SELL_Y + SELL_H//2, f14)

# Sell header
HDR_H = 25
col_header_row(SELL_Y, "Sell book")

# Sell rows  (price ascending from best ask upward in display, but book shows depth of ask at top)
# Row order top→bottom: depth of ask 100.13, 100.12, 100.11, best ask 100.10
sell_rows = [
    ("depth of ask", "100.13", ["100","200"],         False),
    ("",             "100.12", ["600","900"],          False),
    ("",             "100.11", ["900","700","400"],    False),
    ("best ask",     "100.10", ["200","400","1100","100"], True),
]

# highlighted cells (matched by the buy order) — sell side
# best ask 100.10: 200, 400, 1100, 100  → all 4 matched
# 100.11: first cell 900 matched
highlighted_sell = {
    (3,0):True,(3,1):True,(3,2):True,(3,3):True,  # row3 (100.10) all
    (2,0):True,                                     # row2 (100.11) first cell
}

ROW_Y0 = SELL_Y + HDR_H
for ri, (lbl, price, qtys, is_best) in enumerate(sell_rows):
    ry = ROW_Y0 + ri*RH
    # label
    tl(lbl, OX+8, ry+RH//2-7, f13 if not is_best else f14)
    # price
    draw.rectangle([OX+CW_LABEL, ry, OX+CW_LABEL+CW_PRICE, ry+RH], fill=FILL_W, outline=GRAY, width=1)
    tc(price, OX+CW_LABEL+CW_PRICE//2, ry+RH//2, fmono)
    # qty cells
    cx = OX+CW_LABEL+CW_PRICE
    for qi, q in enumerate(qtys):
        fill = (255,230,230) if (ri,qi) in highlighted_sell else FILL_W
        out  = RED if (ri,qi) in highlighted_sell else GRAY
        draw.rectangle([cx, ry, cx+CW_QTY, ry+RH], fill=fill, outline=out, width=2 if (ri,qi) in highlighted_sell else 1)
        tc(q, cx+CW_QTY//2, ry+RH//2, fmono, RED if (ri,qi) in highlighted_sell else BLACK)
        cx += CW_QTY

# dividing line between sell and buy
draw.line([OX, BUY_Y, OX+OW, BUY_Y], fill=BLACK, width=2)

# ── Dashed inner box for buy ──────────────────────────────────────────────
B_INNER_X = OX+10
B_INNER_Y = BUY_Y
B_INNER_W = OW-20
B_INNER_H = BUY_H
dashed_rect(B_INNER_X, B_INNER_Y, B_INNER_X+B_INNER_W, B_INNER_Y+B_INNER_H)

# ── Buy book section label ────────────────────────────────────────────────
tc("Buy book", OX-42, BUY_Y + BUY_H//2, f14)

# Buy header
col_header_row(BUY_Y, "Buy book")

buy_rows = [
    ("best bid",    "100.08", ["500","600","900"],         True),
    ("",            "100.07", ["100","700"],               False),
    ("",            "100.06", ["1100","400","300","200"],  False),
    ("depth of bid","100.05", ["500","100"],               False),
]

# highlighted buy cells (the last matched cell: 100.08 row, cell 2 = 900)
highlighted_buy = {
    (0,2): True,
}

ROW_BY0 = BUY_Y + HDR_H
for ri, (lbl, price, qtys, is_best) in enumerate(buy_rows):
    ry = ROW_BY0 + ri*RH
    tl(lbl, OX+8, ry+RH//2-7, f13 if not is_best else f14)
    draw.rectangle([OX+CW_LABEL, ry, OX+CW_LABEL+CW_PRICE, ry+RH], fill=FILL_W, outline=GRAY, width=1)
    tc(price, OX+CW_LABEL+CW_PRICE//2, ry+RH//2, fmono)
    cx = OX+CW_LABEL+CW_PRICE
    for qi, q in enumerate(qtys):
        fill = (255,230,230) if (ri,qi) in highlighted_buy else FILL_W
        out  = RED if (ri,qi) in highlighted_buy else GRAY
        draw.rectangle([cx, ry, cx+CW_QTY, ry+RH], fill=fill, outline=out, width=2 if (ri,qi) in highlighted_buy else 1)
        tc(q, cx+CW_QTY//2, ry+RH//2, fmono, RED if (ri,qi) in highlighted_buy else BLACK)
        cx += CW_QTY

# ── "price levels" annotation ────────────────────────────────────────────
ann_x = OX+CW_LABEL+CW_PRICE + 2*CW_QTY + 8
ann_y = ROW_Y0 + 0*RH + RH//2 - 2
draw.line([ann_x, ann_y, ann_x+80, ann_y], fill=BLACK, width=1)
# arrow tip
draw.polygon([(ann_x, ann_y),(ann_x+8,ann_y-4),(ann_x+8,ann_y+4)], fill=BLACK)
tl("price levels", ann_x+12, ann_y-8, f12)

# ── Arrows from matched cells pointing down to formula ───────────────────
# The 5 matched quantities: 200(sell r3c0), 400(sell r3c1), 1100(sell r3c2), 100(sell r3c3), 900(buy r0c2)
# We'll draw small diagonal arrows from each highlighted cell downward toward the formula
FORMULA_Y = OY + OH + 20

def cell_center_x(book, row, col):
    base_x = OX + CW_LABEL + CW_PRICE + col*CW_QTY + CW_QTY//2
    return base_x

def cell_center_y(book, row):
    if book == 's':
        return ROW_Y0 + row*RH + RH//2
    else:
        return ROW_BY0 + row*RH + RH//2

matched_cells = [
    ('s',3,0,'200'), ('s',3,1,'400'), ('s',3,2,'1100'), ('s',3,3,'100'), ('s',2,0,'900'),
]
# Draw arrows from matched cells toward the bottom of the outer box
for bk, ri, ci, q in matched_cells:
    cx = cell_center_x(bk, ri, ci)
    cy = cell_center_y(bk, ri)
    bot = OY+OH
    draw.line([cx, cy+RH//2+2, cx, bot+5], fill=RED, width=1)

# ── Formula at bottom ─────────────────────────────────────────────────────
FY = OY + OH + 18
formula_parts = [
    ("Buy 2700 shares:  2700 − ", f13, BLACK),
]
# render inline with boxes
fx = OX + 30
bb = draw.textbbox((0,0), "Buy 2700 shares:  2700 − ", font=f13)
draw.text((fx, FY), "Buy 2700 shares:  2700 − ", fill=BLACK, font=f13)
fx += bb[2]-bb[0]

boxes = [("200","400","1100","100","900")]
qtys_formula = ["200","400","1100","100","900"]
ops = [" − "," − "," − "," − "," = 0"]
BW, BH = 50, 24
for i,(q,op) in enumerate(zip(qtys_formula, ops)):
    draw.rectangle([fx, FY-3, fx+BW, FY+BH-3], fill=(255,220,220), outline=RED, width=1)
    tc(q, fx+BW//2, FY+BH//2-3, f13, RED)
    fx += BW
    draw.text((fx+2, FY), op, fill=BLACK, font=f13)
    bb2 = draw.textbbox((0,0), op, font=f13)
    fx += bb2[2]-bb2[0]+4

# ── Caption ───────────────────────────────────────────────────────────────
caption = "Figure 13.13: Limit order book illustrated"
bb = draw.textbbox((0,0), caption, font=f15)
tc(caption, W//2, H-20, f15)

out = "/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_13.webp"
img.save(out, "WEBP", quality=92)
print(f"Saved: {out}  ({W}x{H})")
