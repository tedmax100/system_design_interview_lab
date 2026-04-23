from PIL import Image, ImageDraw, ImageFont
import math

W, H = 860, 420
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK = (0, 0, 0); GRAY = (180, 180, 180); LIGHT = (245, 245, 245)
DARK = (60, 60, 60); BLUE = (220, 235, 255)

try:
    f = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 16)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
except:
    f = fb = ft = fs = ImageFont.load_default()

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
        tc(lbl,mx,my-10,fs,DARK)

# Exchange big box
box(330,30,830,380,fill=(248,248,255))
tc("Exchange",580,52,ft)

# Three gateways inside Exchange
gw_data = [
    ("App/Web", "Gateway",    370, 90,  200, 60, BLUE),
    ("API Gateway", "(FIX/Non-FIX)", 370, 185, 200, 60, BLUE),
    ("Colo Engine",  "",       370, 280, 200, 60, BLUE),
]
for l1,l2,gx,gy,gw,gh,gcol in gw_data:
    box(gx,gy,gx+gw,gy+gh,fill=gcol)
    tc(l1,gx+gw//2,gy+gh//2-(8 if l2 else 0),fb)
    if l2: tc(l2,gx+gw//2,gy+gh//2+10,f)

# Sharded Services box
box(620,140,820,250,fill=LIGHT)
tc("Sharded",720,182,fb); tc("Services",720,200,fb)

# Arrows from gateways to Sharded Services
arr(570,120,620,182)
arr(570,215,620,200)

# Left-side clients
clients = [
    ("Exchange", "Website/App", 60, 100),
    ("Broker/Dealer",  "",      60, 215),
    ("Other API",  "Users",     60, 310),
]
for l1,l2,cx,cy in clients:
    box(cx-55,cy-25,cx+55,cy+25,fill=LIGHT)
    tc(l1,cx,cy-(8 if l2 else 0),f)
    if l2: tc(l2,cx,cy+10,f)

# Arrows: clients → gateways
arr(115,100,370,120,"HTTP")
arr(115,215,370,215)
arr(115,310,370,310)

# Caption
tc("Figure 13.9: Client gateway",W//2,H-18,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_9.webp","WEBP",quality=92)
print("Saved figure_13_9.webp")
