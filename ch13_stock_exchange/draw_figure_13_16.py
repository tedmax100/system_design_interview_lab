from PIL import Image, ImageDraw, ImageFont
import math

W, H = 720, 500
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); CPU_COL=(255,240,200)
GRAY=(160,160,160); DARK=(60,60,60); GREEN=(215,245,215)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
except:
    f=fb=ft=fs=ImageFont.load_default()

def box(x1,y1,x2,y2,fill=LIGHT,outline=BLACK,w=2):
    draw.rectangle([x1,y1,x2,y2],fill=fill,outline=outline,width=w)

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def arr(x1,y1,x2,y2,lbl="",lbl_side="right",color=BLACK):
    draw.line([x1,y1,x2,y2],fill=color,width=2)
    a=math.atan2(y2-y1,x2-x1); s=9
    for dx,dy in [(math.cos(a-0.4),math.sin(a-0.4)),(math.cos(a+0.4),math.sin(a+0.4))]:
        draw.line([x2,y2,int(x2-s*dx),int(y2-s*dy)],fill=color,width=2)
    if lbl:
        mx,my=(x1+x2)//2,(y1+y2)//2
        off=18 if lbl_side=="right" else -60
        tc(lbl,mx+off,my,fs,DARK)

# Outer Order Manager dashed box
draw.rectangle([80,50,620,440],fill=(250,250,255),outline=GRAY,width=1)
for i in range(0,540,8):
    draw.point((80+i//4*4, 50), fill=GRAY)
tc("Order Manager",350,68,ft)

# "orders" label at top
tc("orders",350,30,f)
arr(350,42,350,100)

# Input Thread/Netloop box
box(220,100,480,150,fill=BLUE)
tc("Input Thread / Netloop",350,125,fb)

# dispatch arrow down
arr(350,150,350,200,"dispatch","right")

# Application Loop Thread dashed box
box(160,200,480,310,fill=(240,240,255),outline=DARK,w=2)
tc("Application Loop Thread",320,218,fb)

# Queue strip inside loop
seg_w=30
for i in range(8):
    draw.rectangle([168+i*seg_w,235,168+(i+1)*seg_w,268],fill=(230,230,230),outline=GRAY,width=1)
tc("...",168+8*seg_w+15,251,f,GRAY)

# "pin to CPU 1" label
tc("pin to CPU 1",390,252,fs,DARK)

# CPU grid
cpu_x,cpu_y=490,215
for r in range(4):
    for c in range(2):
        cx2=cpu_x+c*28; cy2=cpu_y+r*28
        draw.rectangle([cx2,cy2,cx2+26,cy2+26],fill=CPU_COL,outline=GRAY,width=1)
        tc(str(r*2+c),cx2+13,cy2+13,fs)
# arrow to CPU grid
arr(480,252,490,252,"","right")

# "update" arrow to right → Order State
arr(480,260,560,260,"update","right")
box(560,235,660,285,fill=GREEN)
tc("Order",610,252,f); tc("State",610,270,f)

# dispatch arrow down from loop
arr(350,310,350,360,"dispatch","right")

# Output Thread box
box(220,360,480,410,fill=BLUE)
tc("Output Thread / Netloop",350,385,fb)

# orders label at bottom
arr(350,410,350,455)
tc("orders",350,468,f)

tc("Figure 13.16: Application loop thread in Order Manager",W//2,H-12,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_16.webp","WEBP",quality=92)
print("Saved figure_13_16.webp")
