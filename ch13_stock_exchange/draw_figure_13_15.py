from PIL import Image, ImageDraw, ImageFont
import math

W, H = 860, 480
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); GREEN=(215,245,215)
YELLOW=(255,250,220); GRAY=(170,170,170); DARK=(60,60,60)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 16)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
except:
    f=fb=ft=fs=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,color=BLACK):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=9
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)

def queue_strip(x,y,w,h):
    """Draw a mini ring-buffer strip"""
    seg=w//6
    for i in range(6):
        draw.rectangle([x+i*seg,y,x+(i+1)*seg,y+h],fill=(235,235,235),outline=GRAY,width=1)

# Outer server box
box(30,30,830,440,fill=(252,252,255))
tc("One Single Server",W//2,52,ft)

# Three critical-path components
comps = [
    ("Order Manager",       130, 85),
    ("Matching Engine",     410, 85),
    ("Market Data",         690, 85),
]
comp_w, comp_h = 200, 130
for label,cx,cy in comps:
    box(cx-comp_w//2, cy, cx+comp_w//2, cy+comp_h, fill=BLUE)
    tc(label, cx, cy+22, fb)
    if label=="Market Data":
        tc("Publisher", cx, cy+40, fb)
    # App Loop sub-box
    alx=cx-80; aly=cy+55; alw=160; alh=45
    box(alx,aly,alx+alw,aly+alh,fill=(200,215,245))
    tc("Application Loop",cx,aly+alh//2,fs)
    queue_strip(cx-70,aly+28,140,14)

# mmap bar
mmap_y=240; mmap_h=36
box(60,mmap_y,800,mmap_y+mmap_h,fill=(255,245,200),outline=(180,140,0),w=2)
tc("mmap",W//2,mmap_y+mmap_h//2,fb,(120,80,0))

# Arrows: components ↕ mmap
for cx,_ in [(130,0),(410,0),(690,0)]:
    arr(cx,215,cx,mmap_y)
    arr(cx,mmap_y+mmap_h,cx,mmap_y+mmap_h+20)

# Non-critical components below mmap
nc = [
    ("Reporter",              110, 330),
    ("Logging",               270, 330),
    ("Aggregated\nRisk Check",440, 330),
    ("Position\nKeeper",      630, 330),
]
nc_w, nc_h = 150, 55
for label,cx,cy in nc:
    box(cx-nc_w//2,cy,cx+nc_w//2,cy+nc_h,fill=GREEN)
    lines=label.split("\n")
    if len(lines)==1:
        tc(label,cx,cy+nc_h//2,f)
    else:
        tc(lines[0],cx,cy+nc_h//2-9,f)
        tc(lines[1],cx,cy+nc_h//2+9,f)
    # small upward arrow from mmap
    draw.line([cx,mmap_y+mmap_h,cx,cy],fill=GRAY,width=1)

tc("Figure 13.15: A low latency single server exchange design",W//2,H-18,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_15.webp","WEBP",quality=92)
print("Saved figure_13_15.webp")
