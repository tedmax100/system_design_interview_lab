from PIL import Image, ImageDraw, ImageFont
import math

W, H = 700, 380
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); GREEN=(215,245,215)
YELLOW=(255,250,210); GRAY=(160,160,160); DARK=(60,60,60); RED=(255,220,220)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
except:
    f=fb=fs=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,lbl="",lpos="top",color=BLACK):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=9
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)
    if lbl:
        mx,my=(x1+x2)//2,(y1+y2)//2
        off=-12 if lpos=="top" else 14
        tc(lbl,mx,my+off,fs,DARK)

def queue_bar(x,y,w,h):
    seg=w//12
    for i in range(12):
        draw.rectangle([x+i*seg,y,x+(i+1)*seg,y+h],fill=(230,230,230),outline=GRAY,width=1)

# Matching Engine (Hot) - solid border
box(80,50,320,160,fill=BLUE,outline=BLACK,w=3)
tc("Matching Engine",200,80,fb); tc("(Hot)",200,100,f)

# Matching Engine (Warm) - gray dashed
box(420,50,660,160,fill=RED,outline=GRAY,w=2)
tc("Matching Engine",540,80,fb,DARK); tc("(Warm)",540,100,f,DARK)

# Event Store (mmap) bar
es_y=250
box(60,es_y,680,es_y+65,fill=YELLOW,outline=(160,130,0),w=2)
tc("Event Store (mmap)",370,es_y+32,fb,(100,80,0))
queue_bar(70,es_y+8,600,48)

# Arrows Hot → Event Store
arr(200,160,200,es_y,"NewOrderEvent","top")
arr(200,160,260,160)  # horizontal then down
arr(260,160,260,es_y,"OrderFilledEvent","top")

# Arrow Event Store → Warm (NewOrderEvent only)
arr(540,es_y,540,160,"NewOrderEvent","top")

# Labels
tc("(processes & outputs)",200,140,fs,(80,80,80))
tc("(syncs state, no output)",540,140,fs,(80,80,80))

# Heartbeat note
draw.line([320,105,420,105],fill=GRAY,width=1,)
for i in range(0,100,6):
    draw.point((320+i,105),fill=GRAY)
tc("heartbeat",370,92,fs,GRAY)

tc("Figure 13.20: Hot-warm matching engine",W//2,H-16,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_20.webp","WEBP",quality=92)
print("Saved figure_13_20.webp")
