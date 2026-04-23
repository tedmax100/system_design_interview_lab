from PIL import Image, ImageDraw, ImageFont
import math

W, H = 780, 520
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); GREEN=(215,245,215)
YELLOW=(255,250,210); GRAY=(160,160,160); DARK=(60,60,60); ORANGE=(255,235,200)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
    fh = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 11)
except:
    f=fb=ft=fs=fh=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,lbl="",color=BLACK):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=9
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)
    if lbl:
        mx,my=(x1+x2)//2,(y1+y2)//2
        tc(lbl,mx,my-12,fs,DARK)

def ring_circle(cx,cy,r=42,lbl=""):
    draw.ellipse([cx-r,cy-r,cx+r,cy+r],fill=ORANGE,outline=BLACK,width=2)
    draw.ellipse([cx-r+10,cy-r+10,cx+r-10,cy+r-10],fill=(240,220,180),outline=GRAY,width=1)
    if lbl: tc(lbl,cx,cy,fh)

def cylinder(x,y,w,h):
    eh=16
    draw.rectangle([x,y+eh//2,x+w,y+h],fill=LIGHT,outline=BLACK,width=2)
    draw.ellipse([x,y,x+w,y+eh],fill=LIGHT,outline=BLACK,width=2)
    draw.ellipse([x,y+h-eh//2,x+w,y+h+eh//2],fill=LIGHT,outline=BLACK,width=2)

# Matching Engine on left
box(30,180,180,250,fill=BLUE)
tc("Matching",105,205,fb); tc("Engine",105,225,f)

# Arrow to MDP
arr(180,215,270,215,"Orders, matched results")

# MDP outer box
mdp_x,mdp_y,mdp_w,mdp_h=270,60,360,350
box(mdp_x,mdp_y,mdp_x+mdp_w,mdp_y+mdp_h,fill=(250,250,255))
tc("MDP",mdp_x+mdp_w//2,mdp_y+18,ft)

# Three Order book boxes
ob_w,ob_h=80,50
for i,(lbl) in enumerate(["Order\nbook","Order\nbook","Order\nbook"]):
    ox=mdp_x+20+i*(ob_w+10); oy=mdp_y+40
    box(ox,oy,ox+ob_w,oy+ob_h,fill=LIGHT)
    for j,line in enumerate(lbl.split("\n")):
        tc(line,ox+ob_w//2,oy+15+j*18,fs)

# Candlestick rings (with ring buffer labels)
cs_labels=["1 min","1 hour","1 day"]
cs_cx=[mdp_x+60, mdp_x+180, mdp_x+300]
cs_y=mdp_y+155
for cx,lbl in zip(cs_cx,cs_labels):
    ring_circle(cx,cs_y,lbl=lbl)

# Ring buffer annotation
tc("Ring buffer",mdp_x+mdp_w+15,cs_y-15,fs,DARK)
tc("Hold recent 100 ticks",mdp_x+mdp_w+15,cs_y+5,fs,DARK)
# Brace line
draw.line([mdp_x+mdp_w,cs_y-40,mdp_x+mdp_w,cs_y+40],fill=GRAY,width=1)
draw.line([mdp_x+mdp_w,cs_y,mdp_x+mdp_w+14,cs_y],fill=GRAY,width=1)

# Persistence box
per_y=mdp_y+230
box(mdp_x+80,per_y,mdp_x+mdp_w-80,per_y+45,fill=LIGHT)
tc("Persistence",mdp_x+mdp_w//2,per_y+22,fb)

# Data Service box
ds_y=mdp_y+305
box(mdp_x+80,ds_y,mdp_x+mdp_w-80,ds_y+40,fill=GREEN)
tc("Data Service",mdp_x+mdp_w//2,ds_y+20,fb)

# Arrows inside MDP
for cx in cs_cx:
    arr(cx,cs_y+42,cx,per_y)
arr(mdp_x+mdp_w//2,per_y+45,mdp_x+mdp_w//2,ds_y)

tc("Figure 13.24: Market Data Publisher",W//2,H-16,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_24.webp","WEBP",quality=92)
print("Saved figure_13_24.webp")
