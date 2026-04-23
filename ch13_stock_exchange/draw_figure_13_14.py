from PIL import Image, ImageDraw, ImageFont
import math

W, H = 1020, 660
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK  = (0,   0,   0)
GRAY   = (130, 130, 130)
LGRAY  = (220, 220, 220)
RED    = (190,  40,  40)
BLUE   = ( 30,  80, 180)
GREEN  = ( 30, 140,  60)
ORANGE = (210, 110,  20)
BOX_F  = (245, 248, 255)
HL_RED = (255, 230, 230)
HL_GRN = (230, 255, 230)

try:
    f10 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 10)
    f11 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
    f12 = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 12)
    fb12= ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 12)
    fb14= ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fb15= ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    fmono=ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 11)
except:
    f10=f11=f12=fb12=fb14=fb15=fmono=ImageFont.load_default()

def tc(txt, cx, cy, fnt=f12, color=BLACK):
    bb = draw.textbbox((0,0), txt, font=fnt)
    tw, th = bb[2]-bb[0], bb[3]-bb[1]
    draw.text((int(cx-tw//2), int(cy-th//2)), txt, fill=color, font=fnt)

def tl(txt, x, y, fnt=f12, color=BLACK):
    draw.text((int(x), int(y)), txt, fill=color, font=fnt)

def rect(x,y,w,h,fill=BOX_F,outline=BLACK,lw=1):
    draw.rectangle([int(x),int(y),int(x+w),int(y+h)], fill=fill, outline=outline, width=lw)

def ah(x1, y, x2, color=BLACK, lw=1):  # horizontal arrow
    draw.line([int(x1),int(y),int(x2),int(y)], fill=color, width=lw)
    sz=7
    if x2>x1: draw.polygon([(int(x2),int(y)),(int(x2-sz),int(y-3)),(int(x2-sz),int(y+3))],fill=color)
    else:      draw.polygon([(int(x2),int(y)),(int(x2+sz),int(y-3)),(int(x2+sz),int(y+3))],fill=color)

def av(x, y1, y2, color=BLACK, lw=1):  # vertical arrow
    draw.line([int(x),int(y1),int(x),int(y2)], fill=color, width=lw)
    sz=7
    if y2>y1: draw.polygon([(int(x),int(y2)),(int(x-3),int(y2-sz)),(int(x+3),int(y2-sz))],fill=color)
    else:     draw.polygon([(int(x),int(y2)),(int(x-3),int(y2+sz)),(int(x+3),int(y2+sz))],fill=color)

def cross(x,y,w,h):
    draw.line([int(x+2),int(y+2),int(x+w-2),int(y+h-2)],fill=RED,width=2)
    draw.line([int(x+w-2),int(y+2),int(x+2),int(y+h-2)],fill=RED,width=2)

NW, NH = 88, 60  # node width/height
GAP = 10

def map_node(x, y, key, bn=False, an=False):
    """limitMap entry: 4 lines"""
    rect(x, y, NW, NH, fill=BOX_F)
    lh = NH//4
    tc("before"+(("=null")if bn else ""), x+NW//2, y+lh*0+lh//2, f10)
    tc("after" +(("=null")if an else ""), x+NW//2, y+lh*1+lh//2, f10)
    tc(f"key={key}",                      x+NW//2, y+lh*2+lh//2, f10)
    tc("value",                           x+NW//2, y+lh*3+lh//2, f10)

def order_node(x, y, qty, fill=BOX_F, cancelled=False, lw=1):
    rect(x, y, NW, NH, fill=fill, lw=lw)
    lh = NH//4
    tc("before", x+NW//2, y+lh*0+lh//2, f10)
    tc("after",  x+NW//2, y+lh*1+lh//2, f10)
    tc("Order",  x+NW//2, y+lh*2+lh//2, f10)
    tc(f"qty={qty}", x+NW//2, y+lh*3+lh//2, f10)
    if cancelled:
        cross(x, y, NW, NH)

def bidirectional(x1, y, x2):
    """Double arrow between two nodes"""
    ah(x1, y-6, x2)
    ah(x2, y+6, x1)

# ── Title ─────────────────────────────────────────────────────────────────
tl("Buy Book", 12, 8, fb14)
tl("LinkedHashMap<Price, PriceLevel> limitMap", 12, 26, f10, GRAY)

# ── Row centers (vertical) ────────────────────────────────────────────────
ROW_CY = [100, 265, 435]   # center y of each price level row
MX = 15                    # left edge of limitMap entries
DLL_X0 = MX + NW + 30     # start x of DoublyLinkedList nodes

# ─────────────────────────────────────────────────────────────────────────
# ROW 0  ── 100.08  (MATCH: qty=500 removed from head)
# ─────────────────────────────────────────────────────────────────────────
cy = ROW_CY[0]
map_node(MX, cy-NH//2, "100.08", bn=True)
# value→ arrow
ah(MX+NW, cy, DLL_X0-5, BLUE)

# head / tail labels + arrows
tl("head", DLL_X0+14, cy-NH//2-22, f10, GRAY)
av(DLL_X0+22, cy-NH//2-8, cy-NH//2, GRAY)
tl("tail", DLL_X0+2*(NW+GAP)+14, cy-NH//2-22, f10, GRAY)
av(DLL_X0+2*(NW+GAP)+22, cy-NH//2-8, cy-NH//2, GRAY)

# DoublyLinkedList label
tl("DoublyLinkedList<Order> orders", DLL_X0, cy-NH//2-40, f10, BLUE)

# Nodes: 500(matched/head), 600, 900
NX0 = [DLL_X0, DLL_X0+NW+GAP, DLL_X0+2*(NW+GAP)]
order_node(NX0[0], cy-NH//2, 500, fill=HL_RED, lw=2)
order_node(NX0[1], cy-NH//2, 600)
order_node(NX0[2], cy-NH//2, 900)
for i in range(2):
    bidirectional(NX0[i]+NW, cy, NX0[i+1])

# Operation ② callout
op2x = NX0[2]+NW+22;  op2y = cy-NH//2-5
rect(op2x, op2y, 200, 68, fill=(255,252,220), outline=ORANGE, lw=1)
tl("②  Buy order is matched", op2x+5, op2y+5, f10, ORANGE)
tl("and removed from the",   op2x+5, op2y+19, f10, ORANGE)
tl("PriceLevel",              op2x+5, op2y+33, f10, ORANGE)
tl("price=100.08, qty=500",  op2x+5, op2y+50, f10, ORANGE)
# line to matched node
draw.line([int(op2x),int(op2y+68),int(NX0[0]+NW//2),int(cy+NH//2)], fill=ORANGE, width=1)

# ─────────────────────────────────────────────────────────────────────────
# ROW 1  ── 100.07  (PLACE: qty=200 appended to tail)
# ─────────────────────────────────────────────────────────────────────────
cy = ROW_CY[1]
map_node(MX, cy-NH//2, "100.07")
ah(MX+NW, cy, DLL_X0-5, BLUE)
tl("DoublyLinkedList<Order> orders", DLL_X0, cy-NH//2-22, f10, BLUE)
tl("head", DLL_X0+14, cy-NH//2-18, f10, GRAY)
tl("tail", DLL_X0+2*(NW+GAP)+14, cy-NH//2-18, f10, GRAY)

NX1 = [DLL_X0, DLL_X0+NW+GAP, DLL_X0+2*(NW+GAP)]
order_node(NX1[0], cy-NH//2, 100)
order_node(NX1[1], cy-NH//2, 700)
order_node(NX1[2], cy-NH//2, 200, fill=HL_GRN, lw=2)
for i in range(2):
    bidirectional(NX1[i]+NW, cy, NX1[i+1])

# Operation ① callout
op1x = NX1[2]+NW+22;  op1y = cy-20
rect(op1x, op1y, 190, 45, fill=(230,255,230), outline=GREEN, lw=1)
tl("①  Placing a new buy order", op1x+5, op1y+6, f10, GREEN)
tl("price=100.07, qty=200",     op1x+5, op1y+22, f10, GREEN)
draw.line([int(op1x),int(op1y+22),int(NX1[2]+NW),int(cy)], fill=GREEN, width=1)

# ─────────────────────────────────────────────────────────────────────────
# ROW 2  ── 100.06  (CANCEL: qty=400 removed from middle)
# ─────────────────────────────────────────────────────────────────────────
cy = ROW_CY[2]
map_node(MX, cy-NH//2, "100.06", an=True)
ah(MX+NW, cy, DLL_X0-5, BLUE)
tl("DoublyLinkedList<Order> orders", DLL_X0, cy-NH//2-22, f10, BLUE)

NX2 = [DLL_X0, DLL_X0+NW+GAP, DLL_X0+2*(NW+GAP), DLL_X0+3*(NW+GAP)]
order_node(NX2[0], cy-NH//2, 1100)
order_node(NX2[1], cy-NH//2, 400, fill=HL_RED, cancelled=True, lw=2)
order_node(NX2[2], cy-NH//2, 300)
order_node(NX2[3], cy-NH//2, 200)
for i in range(3):
    bidirectional(NX2[i]+NW, cy, NX2[i+1])

# "Step 2" label
tl("Step 2", NX2[1]+18, cy+NH//2+4, f10, RED)

# HashMap<OrderID,Order> orderMap box
om_x = MX;  om_y = cy+NH//2+35
rect(om_x, om_y, 210, 28, fill=BOX_F)
tc("HashMap<OrderID, Order> orderMap", om_x+105, om_y+14, f10)
tl("Step 1", om_x+215, om_y+8, f10, BLACK)
# step1 arrow: orderMap → cancelled node
step1_end_x = NX2[1]+NW//2
ah(om_x+210, om_y+14, step1_end_x, BLACK)
draw.line([int(step1_end_x),int(om_y+14),int(step1_end_x),int(cy+NH//2+4)], fill=BLACK, width=1)
draw.polygon([(int(step1_end_x),int(cy+NH//2+4)),
              (int(step1_end_x-3),int(cy+NH//2+12)),
              (int(step1_end_x+3),int(cy+NH//2+12))], fill=BLACK)

# Operation ③ callout
op3x = NX2[3]+NW+18;  op3y = cy-30
rect(op3x, op3y, 200, 102, fill=(255,238,238), outline=RED, lw=1)
tl("③  Cancel an order",        op3x+5, op3y+5,  f10, RED)
tl("price=100.06, qty=400",     op3x+5, op3y+19, f10, RED)
draw.line([int(op3x),int(op3y+34),int(op3x+198),int(op3y+34)], fill=RED, width=1)
tl("Step 1. Find the Order in", op3x+5, op3y+40, f10, GRAY)
tl("orderMap via OrderID",       op3x+5, op3y+54, f10, GRAY)
tl("Step 2. Remove the Order",  op3x+5, op3y+68, f10, GRAY)
tl("element from the PriceLevel",op3x+5,op3y+82, f10, GRAY)

# ── Vertical chain between limitMap entries ────────────────────────────────
for r in range(2):
    bot = ROW_CY[r]   + NH//2
    top = ROW_CY[r+1] - NH//2
    xc  = MX + NW//2
    av(xc-6,  bot+2, top-2, GRAY)  # down
    av(xc+6,  top-2, bot+2, GRAY)  # up

# ── Caption ───────────────────────────────────────────────────────────────
caption = "Figure 13.14: Place, match, and cancel an order in O(1)"
bb = draw.textbbox((0,0), caption, font=fb14)
tc(caption, W//2, H-18, fb14)

out="/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_14.webp"
img.save(out, "WEBP", quality=92)
print(f"Saved: {out}  ({W}x{H})")
