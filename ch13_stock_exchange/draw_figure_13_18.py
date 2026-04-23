from PIL import Image, ImageDraw, ImageFont
import math

W, H = 920, 620
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); GREEN=(215,245,215)
YELLOW=(255,250,210); GRAY=(160,160,160); DARK=(60,60,60); ORANGE=(255,235,200)
PURPLE=(235,220,255)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 12)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 13)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 10)
    fh = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 11)
except:
    f=fb=ft=fs=fh=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,lbl="",color=BLACK,lpos="top"):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=8
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)
    if lbl:
        mx,my=(x1+x2)//2,(y1+y2)//2
        off=-12 if lpos=="top" else 12
        tc(lbl,mx,my+off,fs,DARK)

def queue(x,y,w,h):
    seg=w//8
    for i in range(8):
        draw.rectangle([x+i*seg,y,x+(i+1)*seg,y+h],fill=(230,230,230),outline=GRAY,width=1)

# ── Domain dividers ──
draw.line([200,0,200,H-35],fill=(200,200,200),width=1)
draw.line([200,160,W,160],fill=(200,200,200),width=1)
tc("External Domain",100,20,ft,GRAY)
tc("(FIX)",100,38,f,GRAY)
tc("Trading Domain (SBE)",520,20,ft,DARK)

# ── Gateway ──
box(40,60,170,120,fill=BLUE)
tc("Gateway",105,80,fb); tc("Event Store",105,96,f); tc("Client",105,112,fs)

# ── Event Store (mmap) ──
es_x,es_y,es_w,es_h=220,60,500,50
box(es_x,es_y,es_x+es_w,es_y+es_h,fill=YELLOW,outline=(160,130,0),w=2)
tc("Event Store (mmap)",es_x+es_w//2,es_y+es_h//2,fb,(100,80,0))
queue(es_x+10,es_y+8,es_w-20,34)

# Arrow: Gateway → Event Store
arr(170,90,220,85,"NewOrderEvent")

# ── Matching Engine outer box ──
me_outer_x,me_outer_y=220,180
me_outer_w,me_outer_h=500,260
box(me_outer_x,me_outer_y,me_outer_x+me_outer_w,me_outer_y+me_outer_h,fill=(250,250,255))
tc("Matching Engine",me_outer_x+me_outer_w//2,me_outer_y+18,ft)

# Order Manager inner box
om_x,om_y,om_w,om_h=me_outer_x+20,me_outer_y+40,200,100
box(om_x,om_y,om_x+om_w,om_y+om_h,fill=BLUE)
tc("Order Manager",om_x+om_w//2,om_y+18,fb)
tc("Order State",om_x+om_w//2,om_y+45,f)
# app loop
box(om_x+20,om_y+62,om_x+om_w-20,om_y+om_h-8,fill=(200,215,245))
tc("App loop",om_x+om_w//2,om_y+80,fs)
queue(om_x+25,om_y+68,om_w-50,20)

# Matching Core inner box
mc_x,mc_y,mc_w,mc_h=me_outer_x+260,me_outer_y+40,200,100
box(mc_x,mc_y,mc_x+mc_w,mc_y+mc_h,fill=GREEN)
tc("Matching Core",mc_x+mc_w//2,mc_y+35,fb)

# Arrow: Order Manager → Matching Core
arr(om_x+om_w,om_y+om_h//2,mc_x,mc_y+mc_h//2,"Send to matching")

# Event Store Client inside ME
esc_x,esc_y,esc_w,esc_h=me_outer_x+140,me_outer_y+165,180,55
box(esc_x,esc_y,esc_x+esc_w,esc_y+esc_h,fill=ORANGE)
tc("Event Store Client",esc_x+esc_w//2,esc_y+18,f)
tc("(App loop)",esc_x+esc_w//2,esc_y+36,fs)

# Arrow: Event Store (mmap) → ESC inside ME (pull)
arr(es_x+es_w//2,es_y+es_h,esc_x+esc_w//2,esc_y,"Pull events")
# Arrow: ME → Event Store (OrderFilledEvent)
arr(mc_x+mc_w,mc_y+mc_h//2,es_x+es_w,es_y+es_h//2,"OrderFilledEvent")

# ── Market Data Publisher ──
mdp_x,mdp_y,mdp_w,mdp_h=760,180,130,80
box(mdp_x,mdp_y,mdp_x+mdp_w,mdp_y+mdp_h,fill=PURPLE)
tc("Market Data",mdp_x+mdp_w//2,mdp_y+25,fb)
tc("Publisher",mdp_x+mdp_w//2,mdp_y+45,fb)
arr(es_x+es_w,es_y+25,mdp_x,mdp_y+40,"Events")

# ── Reporter ──
rep_x,rep_y,rep_w,rep_h=330,480,200,60
box(rep_x,rep_y,rep_x+rep_w,rep_y+rep_h,fill=(240,240,255))
tc("Reporter",rep_x+rep_w//2,rep_y+20,fb)
box(rep_x+30,rep_y+33,rep_x+rep_w-30,rep_y+rep_h-8,fill=(225,225,245))
tc("Order Manager",rep_x+rep_w//2,rep_y+46,fs)
arr(es_x+es_w//2,es_y+es_h+120,rep_x+rep_w//2,rep_y,"Events")

# Domain boundary labels
tc("Reporting Domain (Your choice)",rep_x+rep_w//2,rep_y+rep_h+18,fs,GRAY)

# FIX arrow into Gateway
arr(15,90,40,90,"FIX")

tc("Figure 13.18: An event sourcing design",W//2,H-15,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_18.webp","WEBP",quality=92)
print("Saved figure_13_18.webp")
