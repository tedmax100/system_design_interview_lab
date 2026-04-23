from PIL import Image, ImageDraw, ImageFont
import math

W, H = 820, 480
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); LIGHT=(245,245,245); BLUE=(215,230,255); GRAY=(170,170,170)
DARK=(60,60,60); GREEN=(215,245,215); RED=(255,220,220); YELLOW=(255,250,220)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 13)
    ft = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 15)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
    fh = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 12)
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

def circle(cx,cy,r,fill=LIGHT,outline=BLACK,w=2):
    draw.ellipse([cx-r,cy-r,cx+r,cy+r],fill=fill,outline=outline,width=w)

def table(x,y,w,h,headers,rows,header_fill=(200,215,245)):
    rh=(h-25)//(len(rows)+1)
    # header
    box(x,y,x+w,y+25,fill=header_fill)
    col_w=w//len(headers)
    for i,hdr in enumerate(headers):
        tc(hdr,x+i*col_w+col_w//2,y+12,fh)
    # rows
    for ri,row in enumerate(rows):
        ry=y+25+ri*rh
        bg=LIGHT if ri%2==0 else (255,255,255)
        box(x,ry,x+w,ry+rh,fill=bg,outline=GRAY,w=1)
        for ci,cell in enumerate(row):
            tc(cell,x+ci*col_w+col_w//2,ry+rh//2,fs)

# ──── Title divider ────
draw.line([W//2,20,W//2,H-40],fill=GRAY,width=1)
tc("Non Event Sourcing",200,22,ft); tc("Event Sourcing",610,22,ft)

# ──── LEFT: state transition diagram ────
r=38
c1=(150,100); c2=(320,100)
circle(*c1,r,fill=BLUE)
tc("Order V1",c1[0],c1[1]-8,fs); tc("New",c1[0],c1[1]+8,fh)
circle(*c2,r,fill=GREEN)
tc("Order V2",c2[0],c2[1]-8,fs); tc("Filled",c2[0],c2[1]+8,fh)
arr(c1[0]+r,c1[1],c2[0]-r,c2[1],"OrderFilledEvent")

# LEFT table
table(60,175,320,160,
      ["Version","OrderStatus"],
      [["V1","New"],["V2","Filled"]])
tc("Order",220,172,fh,DARK)

# Dashed box label
draw.rectangle([60,160,380,345],outline=GRAY,width=1)
tc("Non Event Sourcing",220,340,fs,GRAY)

# ──── RIGHT: event sourcing ────
offset=420
c3=(offset+130,100); c4=(offset+300,100)
circle(*c3,r,fill=BLUE)
tc("Order V1",c3[0],c3[1]-8,fs); tc("New",c3[0],c3[1]+8,fh)
circle(*c4,r,fill=GREEN)
tc("Order V2",c4[0],c4[1]-8,fs); tc("Filled",c4[0],c4[1]+8,fh)
arr(c3[0]+r,c3[1],c4[0]-r,c4[1],"OrderFilledEvent")

# RIGHT: two tables side by side
table(offset+20,175,175,160,
      ["Version","OrderStatus"],
      [["V1","New"],["V2","Filled"]])
tc("Order",offset+107,172,fh,DARK)

table(offset+210,175,195,160,
      ["Event Seq.","Event Type"],
      [["100","NewOrderEvent"],["101","OrderFilledEvent"]])
tc("Event",offset+307,172,fh,DARK)

draw.rectangle([offset+20,160,offset+405,345],outline=GRAY,width=1)
tc("Event Sourcing",offset+212,340,fs,GRAY)

tc("Figure 13.17: Non-event sourcing vs event sourcing",W//2,H-15,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_17.webp","WEBP",quality=92)
print("Saved figure_13_17.webp")
