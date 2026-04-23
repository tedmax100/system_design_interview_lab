from PIL import Image, ImageDraw, ImageFont

W, H = 900, 700
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK = (0, 0, 0)
DARK_GRAY = (60, 60, 60)
BOX_FILL = (245, 248, 255)
HEADER_FILL = (220, 230, 250)

try:
    font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    font_bold = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    font_mono = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 12)
    font_caption = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 16)
except:
    font = ImageFont.load_default()
    font_bold = font
    font_mono = font
    font_caption = font

def text_center(txt, cx, cy, fnt=None, color=BLACK):
    fnt = fnt or font
    bb = draw.textbbox((0, 0), txt, font=fnt)
    tw, th = bb[2]-bb[0], bb[3]-bb[1]
    draw.text((cx - tw//2, cy - th//2), txt, fill=color, font=fnt)

def draw_class_box(x, y, title, fields):
    line_h = 20
    pad = 8
    w = 220
    header_h = 32
    body_h = len(fields) * line_h + pad * 2
    h = header_h + body_h

    # Header
    draw.rectangle([x, y, x+w, y+header_h], fill=HEADER_FILL, outline=BLACK, width=2)
    text_center(title, x+w//2, y+header_h//2, font_bold)

    # Body
    draw.rectangle([x, y+header_h, x+w, y+h], fill=BOX_FILL, outline=BLACK, width=2)
    for i, field in enumerate(fields):
        fy = y + header_h + pad + i*line_h + line_h//2
        draw.text((x+10, fy - 7), field, fill=DARK_GRAY, font=font_mono)

    return x, y, w, h

def arrow_line(x1, y1, x2, y2, label_start="", label_end="", dashed=False):
    if dashed:
        # draw dashed line
        import math
        length = math.hypot(x2-x1, y2-y1)
        steps = int(length / 8)
        for i in range(steps):
            t1, t2 = i/steps, (i+0.4)/steps
            lx1, ly1 = x1+(x2-x1)*t1, y1+(y2-y1)*t1
            lx2, ly2 = x1+(x2-x1)*t2, y1+(y2-y1)*t2
            draw.line([lx1, ly1, lx2, ly2], fill=BLACK, width=2)
    else:
        draw.line([x1, y1, x2, y2], fill=BLACK, width=2)

    # arrowhead at end
    import math
    angle = math.atan2(y2-y1, x2-x1)
    size = 10
    ax = x2 - size*math.cos(angle-0.4)
    ay = y2 - size*math.sin(angle-0.4)
    bx = x2 - size*math.cos(angle+0.4)
    by = y2 - size*math.sin(angle+0.4)
    draw.polygon([(x2,y2),(ax,ay),(bx,by)], fill=BLACK)

    # labels
    if label_start:
        bb = draw.textbbox((0,0), label_start, font=font)
        tw = bb[2]-bb[0]
        draw.text((x1+6, y1-18), label_start, fill=BLACK, font=font)
    if label_end:
        bb = draw.textbbox((0,0), label_end, font=font)
        tw = bb[2]-bb[0]
        draw.text((x2-tw-6, y2+4), label_end, fill=BLACK, font=font)

# ── Order fields ───────────────────────────────────────────────────────────
order_fields = [
    "+ orderID: UUID",
    "+ productID: int",
    "+ price: long",
    "+ quantity: long",
    "+ side: Side",
    "+ orderStatus: OrderStatus",
    "+ orderType: OrderType",
    "+ timeInForce: TimeInForce",
    "+ symbol: long",
    "+ userID: long",
    "+ clientOrderID: string",
    "+ broker: string",
    "+ accountID: long",
    "+ entryTime: long",
    "+ transactionTime: long",
]

# ── Execution fields ───────────────────────────────────────────────────────
exec_fields = [
    "+ execID: UUID",
    "+ orderID: UUID",
    "+ price: long",
    "+ quantity: long",
    "+ side: Side",
    "+ orderStatus: OrderStatus",
    "+ orderType: OrderType",
    "+ symbol: long",
    "+ userID: long",
    "+ feeCurrency: Currency",
    "+ feeRate: long",
    "+ feeAmount: long",
    "+ accountID: long",
    "+ execStatus: ExecStatus",
    "+ transactionTime: long",
]

# ── Product fields ─────────────────────────────────────────────────────────
product_fields = [
    "+ productID: int",
    "+ symbol: type",
    "+ lotSize: int",
    "+ tickSize: decimal",
    "+ quoteCurrency: Currency",
    "+ settleCurrency: Currency",
    "+ description: string",
    "+ field: type",
]

# positions
ox, oy = 40, 50
ex_x, ex_y = 630, 50
prod_x, prod_y = 330, 440

ox, oy, ow, oh = draw_class_box(ox, oy, "Order", order_fields)
ex_x, ex_y, ew, eh = draw_class_box(ex_x, ex_y, "Execution", exec_fields)
prod_x, prod_y, pw, ph = draw_class_box(prod_x, prod_y, "Product", product_fields)

# ── Relationship: Order 1 → 0..n Execution (horizontal) ───────────────────
# from right edge of Order to left edge of Execution, mid-height
order_right_x = ox + ow
order_mid_y = oy + 32 + 5*20 + 10  # roughly middle

exec_left_x = ex_x
exec_mid_y = ex_y + 32 + 5*20 + 10

# use a horizontal line at consistent y
rel_y = oy + 32 + 3*20
arrow_line(ox+ow, rel_y, ex_x, rel_y)
# multiplicity labels
draw.text((ox+ow+8, rel_y-18), "1", fill=BLACK, font=font)
draw.text((ex_x-24, rel_y-18), "0..n", fill=BLACK, font=font)

# ── Relationship: Order 1 → 1 Product (vertical) ──────────────────────────
order_bot_x = ox + ow//2
order_bot_y = oy + oh

prod_top_x = prod_x + pw//2
prod_top_y = prod_y

# L-shaped connector
mid_y2 = (order_bot_y + prod_top_y) // 2
draw.line([order_bot_x, order_bot_y, order_bot_x, mid_y2], fill=BLACK, width=2)
draw.line([order_bot_x, mid_y2, prod_top_x, mid_y2], fill=BLACK, width=2)

import math
angle = math.atan2(prod_top_y - mid_y2, 0)
draw.line([prod_top_x, mid_y2, prod_top_x, prod_top_y], fill=BLACK, width=2)
# arrowhead
ax = prod_top_x - 10*math.cos(math.pi/2 - 0.4)
ay = prod_top_y - 10*math.sin(math.pi/2 - 0.4)
bx = prod_top_x - 10*math.cos(math.pi/2 + 0.4)
by = prod_top_y - 10*math.sin(math.pi/2 + 0.4)
draw.polygon([(prod_top_x, prod_top_y),(ax,ay),(bx,by)], fill=BLACK)

# multiplicity
draw.text((order_bot_x+6, order_bot_y+4), "1", fill=BLACK, font=font)
draw.text((prod_top_x+6, prod_top_y-20), "1", fill=BLACK, font=font)

# ── Caption ────────────────────────────────────────────────────────────────
caption = "Figure 13.12: Product, order, execution"
bb = draw.textbbox((0,0), caption, font=font_caption)
tw = bb[2]-bb[0]
draw.text(((W-tw)//2, H-30), caption, fill=BLACK, font=font_caption)

out = "/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_12.webp"
img.save(out, "WEBP", quality=92)
print(f"Saved: {out}  ({W}x{H})")
