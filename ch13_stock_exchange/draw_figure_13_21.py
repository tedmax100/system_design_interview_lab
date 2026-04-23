from PIL import Image, ImageDraw, ImageFont
import math

W, H = 820, 560
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); RED=(255,220,220)
YELLOW=(255,250,210); GRAY=(160,160,160); DARK=(60,60,60)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 12)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 13)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 10)
except:
    f=fb=ft=fs=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,lbl="",color=BLACK):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=8
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)
    if lbl:
        mx,my=(x1+x2)//2,(y1+y2)//2
        tc(lbl,mx+8,my,fs,DARK)

def queue_bar(x,y,w,h):
    seg=w//10
    for i in range(10):
        draw.rectangle([x+i*seg,y,x+(i+1)*seg,y+h],fill=(230,230,230),outline=GRAY,width=1)

# Top: Hot and Warm Matching Engines
box(60,30,300,100,fill=BLUE,outline=BLACK,w=3)
tc("Matching Engine",180,55,fb); tc("(Hot)",180,75,f)

box(460,30,700,100,fill=RED,outline=GRAY,w=2)
tc("Matching Engine",580,55,fb,DARK); tc("(Warm)",580,75,f,DARK)

# Hot arrows (NewOrderEvent ↓, OrderFilledEvent ↑)
arr(150,100,150,175,"NewOrderEvent")
arr(200,175,200,100,"OrderFilledEvent")

# Warm arrow (NewOrderEvent ↓)
arr(580,175,580,100,"NewOrderEvent")

# 5 Event Store rows
es_x0,es_w,es_h=60,700,42
es_starts=[175,255,320,385,450]
for i,esy in enumerate(es_starts):
    fill=YELLOW if i==0 else (248,245,230)
    box(es_x0,esy,es_x0+es_w,esy+es_h,fill=fill,outline=(160,130,0),w=(2 if i==0 else 1))
    queue_bar(es_x0+8,esy+5,es_w-16,es_h-10)
    if i==0:
        tc("Event Store (mmap)  [Leader]",es_x0+es_w//2,esy+es_h//2,fb,(100,80,0))
    else:
        tc(f"Event Store (mmap)  [Follower {i}]",es_x0+es_w//2,esy+es_h//2,fs,DARK)

# AppendEntries RPC lines from leader to followers
for esy in es_starts[1:]:
    # left side curvy arrow
    sx,sy=es_x0+35,es_starts[0]+es_h
    ex,ey=es_x0+35,esy
    draw.line([sx,sy,ex,ey],fill=(100,100,200),width=2)
    # arrowhead
    a=math.atan2(ey-sy,0); s=7
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([ex,ey,int(ex-s*dx),int(ey-s*dy)],fill=(100,100,200),width=2)

tc("AppendEntries RPCs",es_x0-5,340,fs,(100,100,200))

tc("Figure 13.21: Event replication in Raft cluster",W//2,H-16,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_21.webp","WEBP",quality=92)
print("Saved figure_13_21.webp")
