from PIL import Image, ImageDraw, ImageFont
import math

W, H = 700, 440
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); YELLOW=(255,250,210)
GRAY=(160,160,160); DARK=(60,60,60); GREEN=(215,245,215)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
    fh = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 11)
except:
    f=fb=fs=fh=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,lbl="",lpos="left",color=BLACK):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=9
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)
    if lbl:
        mx,my=(x1+x2)//2,(y1+y2)//2
        xoff=-75 if lpos=="left" else 10
        tc(lbl,mx+xoff,my,fs,DARK)

def ring(cx,cy,r=35):
    draw.ellipse([cx-r,cy-r,cx+r,cy+r],fill=(235,235,235),outline=BLACK,width=2)
    # inner
    draw.ellipse([cx-r+8,cy-r+8,cx+r-8,cy+r-8],fill=(220,220,220),outline=GRAY,width=1)
    tc("ring",cx,cy-8,fs); tc("buffer",cx,cy+8,fs)

def queue_bar(x,y,w,h):
    seg=w//10
    for i in range(10):
        draw.rectangle([x+i*seg,y,x+(i+1)*seg,y+h],fill=(230,230,230),outline=GRAY,width=1)

# Gateway box
box(60,40,240,110,fill=BLUE)
tc("Gateway",150,75,fb)

# Matching Engine box
box(460,40,640,110,fill=GREEN)
tc("Matching",550,65,fb); tc("Engine",550,83,fb)

# Ring buffers
ring(150,185)
ring(550,185)

# Sequencer box
box(270,155,430,215,fill=LIGHT)
tc("Sequencer",350,185,fb)

# Event Store (mmap)
es_y=310
box(60,es_y,640,es_y+55,fill=YELLOW,outline=(160,130,0),w=2)
tc("Event Store (mmap)",350,es_y+28,fb,(100,80,0))
queue_bar(70,es_y+8,560,38)

# Step labels with circles
def step_circle(cx,cy,n,lbl):
    draw.ellipse([cx-14,cy-14,cx+14,cy+14],fill=(100,150,255),outline=BLACK,width=1)
    tc(str(n),cx,cy,fb,(255,255,255))
    tc(lbl,cx+70,cy,fs,DARK)

# Arrows
# 1. Gateway/ME write to ring buffer
arr(150,110,150,150,"","left")
arr(550,110,550,150,"","left")
step_circle(180,130,1,"Write to ring buffer")

# 2. Sequencer pulls from ring buffers
arr(185,185,270,185,"","left")
arr(430,185,515,185,"","left")
step_circle(240,210,2,"Sequencer pulls data from ring buffer")

# 3. Sequencer writes to Event Store
arr(350,215,350,310,"","left")
step_circle(420,260,3,"Sequencer writes to Event Store")

tc("Figure 13.19: Sample design of Sequencer",W//2,H-16,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_19.webp","WEBP",quality=92)
print("Saved figure_13_19.webp")
