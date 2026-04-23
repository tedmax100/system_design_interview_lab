from PIL import Image, ImageDraw, ImageFont
import os

W, H = 900, 760
bg = (255, 255, 255)
img = Image.new("RGB", (W, H), bg)
draw = ImageDraw.Draw(img)

# Colors
BLACK = (0, 0, 0)
GRAY = (200, 200, 200)
LIGHT_GRAY = (240, 240, 240)
DARK_GRAY = (80, 80, 80)
BOX_FILL = (245, 245, 245)
DOC_FILL = (255, 255, 240)

try:
    font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 14)
    font_bold = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    font_small = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 12)
    font_title = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 17)
except:
    font = ImageFont.load_default()
    font_bold = font
    font_small = font
    font_title = font

def rect(x1, y1, x2, y2, fill=BOX_FILL, outline=BLACK, width=2):
    draw.rectangle([x1, y1, x2, y2], fill=fill, outline=outline, width=width)

def text_center(txt, cx, cy, fnt=font, color=BLACK):
    bb = draw.textbbox((0, 0), txt, font=fnt)
    tw = bb[2] - bb[0]
    th = bb[3] - bb[1]
    draw.text((cx - tw // 2, cy - th // 2), txt, fill=color, font=fnt)

def arrow(x1, y1, x2, y2, label="", color=BLACK):
    draw.line([x1, y1, x2, y2], fill=color, width=2)
    # arrowhead
    import math
    angle = math.atan2(y2 - y1, x2 - x1)
    size = 10
    ax = x2 - size * math.cos(angle - 0.4)
    ay = y2 - size * math.sin(angle - 0.4)
    bx = x2 - size * math.cos(angle + 0.4)
    by = y2 - size * math.sin(angle + 0.4)
    draw.polygon([(x2, y2), (ax, ay), (bx, by)], fill=color)
    if label:
        mx = (x1 + x2) // 2
        my = (y1 + y2) // 2
        text_center(label, mx + 30, my, font_small, DARK_GRAY)

def cylinder(x, y, w, h, fill=BOX_FILL):
    ew = w
    eh = 18
    # body
    draw.rectangle([x, y + eh//2, x + w, y + h], fill=fill, outline=BLACK, width=2)
    # top ellipse
    draw.ellipse([x, y, x + ew, y + eh], fill=fill, outline=BLACK, width=2)
    # bottom ellipse cap
    draw.ellipse([x, y + h - eh//2, x + ew, y + h + eh//2], fill=fill, outline=BLACK, width=2)

def doc_shape(x, y, w, h, label, sub=None):
    """Document icon with folded corner"""
    fold = 15
    pts = [
        (x, y),
        (x + w - fold, y),
        (x + w, y + fold),
        (x + w, y + h),
        (x, y + h),
    ]
    draw.polygon(pts, fill=DOC_FILL, outline=BLACK)
    draw.line([(x + w - fold, y), (x + w - fold, y + fold), (x + w, y + fold)], fill=BLACK, width=1)
    text_center(label, x + w // 2, y + h // 2 - 6, font_small)
    if sub:
        text_center(sub, x + w // 2, y + h // 2 + 10, font_small)

# ─── Layout ───────────────────────────────────────────────────────────────────
# Top row: Order Manager  ←orders─  Matching Manager (renamed Matching Engine)
om_x, om_y, om_w, om_h = 100, 50, 160, 50
me_x, me_y, me_w, me_h = 620, 50, 160, 50
rect(om_x, om_y, om_x + om_w, om_y + om_h)
text_center("Order Manager", om_x + om_w // 2, om_y + om_h // 2, font_bold)
rect(me_x, me_y, me_x + me_w, me_y + me_h)
text_center("Matching", me_x + me_w // 2, me_y + 18, font_bold)
text_center("Engine", me_x + me_w // 2, me_y + 34, font_bold)

# arrows between them
arrow(me_x, om_y + 18, om_x + om_w, om_y + 18, "orders")
arrow(om_x + om_w, om_y + 36, me_x, om_y + 36, "fills/rejects")

# ─── Reporter big box ────────────────────────────────────────────────────────
rep_x, rep_y, rep_w, rep_h = 220, 150, 440, 280
rect(rep_x, rep_y, rep_x + rep_w, rep_y + rep_h, fill=(248, 248, 255))
text_center("Reporter", rep_x + rep_w // 2, rep_y + 18, font_title)

# Dashed sub-boxes inside Reporter: Request | Response
# Request box
req_x, req_y, req_w, req_h = rep_x + 20, rep_y + 45, 160, 170
draw.rectangle([req_x, req_y, req_x + req_w, req_y + req_h], fill=(255,255,255), outline=DARK_GRAY, width=1)
text_center("Request", req_x + req_w // 2, req_y + 14, font_small, DARK_GRAY)

# Response box
res_x, res_y, res_w, res_h = rep_x + 220, rep_y + 45, 200, 170
draw.rectangle([res_x, res_y, res_x + res_w, res_y + res_h], fill=(255,255,255), outline=DARK_GRAY, width=1)
text_center("Response", res_x + res_w // 2, res_y + 14, font_small, DARK_GRAY)

# Doc inside Request: NewOrderReq
doc_shape(req_x + 30, req_y + 35, 80, 90, "NewOrderReq")

# Docs inside Response: NewOrderAck | Fill
doc_shape(res_x + 10, res_y + 35, 75, 90, "NewOrder", "Ack")
doc_shape(res_x + 105, res_y + 35, 60, 90, "Fill")

# ExecutionReport box (bottom of Reporter)
er_x, er_y, er_w, er_h = rep_x + 110, rep_y + 225, 200, 38
rect(er_x, er_y, er_x + er_w, er_y + er_h)
text_center("ExecutionReport", er_x + er_w // 2, er_y + er_h // 2, font_bold)

# arrow from docs to ExecutionReport (merge arrow from middle of inner boxes)
mid_req = req_x + req_w // 2
mid_res = res_x + res_w // 2
er_top_cx = er_x + er_w // 2
draw.line([mid_req, req_y + req_h, mid_req, er_y - 10], fill=BLACK, width=2)
draw.line([mid_res, res_y + res_h, mid_res, er_y - 10], fill=BLACK, width=2)
draw.line([mid_req, er_y - 10, mid_res, er_y - 10], fill=BLACK, width=2)
arrow(er_top_cx, er_y - 10, er_top_cx, er_y)

# ─── Arrow from Order Manager down to Reporter ───────────────────────────────
arrow(om_x + om_w // 2, om_y + om_h, om_x + om_w // 2, rep_y)

# ─── Arrow from Matching Engine down to Reporter ─────────────────────────────
arrow(me_x + me_w // 2, me_y + me_h, me_x + me_w // 2, rep_y + 50)
draw.line([me_x + me_w // 2, rep_y + 50, rep_x + rep_w, rep_y + 50], fill=BLACK, width=2)

# ─── Below Reporter: two outputs ─────────────────────────────────────────────
# Arrow from ExecutionReport down
arrow(er_x + er_w // 2, er_y + er_h, er_x + er_w // 2, er_y + er_h + 30)

split_y = er_y + er_h + 30
sc_cx = 220   # Settlement & Clearing center
br_cx = 660   # Books & Records center

# horizontal split line
draw.line([sc_cx, split_y, br_cx, split_y], fill=BLACK, width=2)
arrow(sc_cx, split_y, sc_cx, split_y + 30)
arrow(br_cx, split_y, br_cx, split_y + 30)

# Settlement & Clearing box
sc_x, sc_y, sc_w, sc_h = sc_cx - 95, split_y + 30, 190, 50
rect(sc_x, sc_y, sc_x + sc_w, sc_y + sc_h)
text_center("Settlement &", sc_x + sc_w // 2, sc_y + 16, font)
text_center("Clearing", sc_x + sc_w // 2, sc_y + 34, font)

# Books & Records box
br_x, br_y, br_w, br_h = br_cx - 95, split_y + 30, 190, 50
rect(br_x, br_y, br_x + br_w, br_y + br_h)
text_center("Books &", br_x + br_w // 2, br_y + 16, font)
text_center("Records", br_x + br_w // 2, br_y + 34, font)

# Cylinders (databases) to the right of each
cyl_w, cyl_h = 50, 55
cylinder(sc_x + sc_w + 15, sc_y - 5, cyl_w, cyl_h)
cylinder(br_x + br_w + 15, br_y - 5, cyl_w, cyl_h)

# Arrows into cylinders
arrow(sc_x + sc_w, sc_y + sc_h // 2, sc_x + sc_w + 15, sc_y + sc_h // 2 - 2)
arrow(br_x + br_w, br_y + br_h // 2, br_x + br_w + 15, br_y + br_h // 2 - 2)

# Below both boxes → Reporting
rp_cx = W // 2
arrow(sc_cx, sc_y + sc_h, sc_cx, sc_y + sc_h + 30)
arrow(br_cx, sc_y + sc_h, br_cx, sc_y + sc_h + 30)
join_y = sc_y + sc_h + 30
draw.line([sc_cx, join_y, br_cx, join_y], fill=BLACK, width=2)
arrow(rp_cx, join_y, rp_cx, join_y + 30)

# Reporting box
rp_x, rp_y, rp_w, rp_h = rp_cx - 80, join_y + 30, 160, 50
rect(rp_x, rp_y, rp_x + rp_w, rp_y + rp_h)
text_center("Reporting", rp_x + rp_w // 2, rp_y + rp_h // 2, font_bold)

# Final cylinder (DB)
cyl2_w, cyl2_h = 55, 60
arrow(rp_x + rp_w // 2, rp_y + rp_h, rp_x + rp_w // 2, rp_y + rp_h + 30)
cylinder(rp_x + rp_w // 2 - cyl2_w // 2, rp_y + rp_h + 30, cyl2_w, cyl2_h)

# ─── Caption ─────────────────────────────────────────────────────────────────
caption = "Figure 13.11: Reporter"
bb = draw.textbbox((0, 0), caption, font=font_bold)
tw = bb[2] - bb[0]
draw.text(((W - tw) // 2, H - 28), caption, fill=BLACK, font=font_bold)

# Save
out = "/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_11.webp"
img.save(out, "WEBP", quality=92)
print(f"Saved: {out}  ({W}x{H})")
