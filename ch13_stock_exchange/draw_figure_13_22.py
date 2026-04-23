from PIL import Image, ImageDraw, ImageFont

W, H = 760, 200
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); GRAY=(160,160,160); DARK=(60,60,60); LIGHT=(245,245,245)
ELECTION=(200,200,220); NORMAL=(230,230,230); SPLIT=(180,180,200)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
    fh = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 11)
except:
    f=fb=fs=fh=ImageFont.load_default()

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

# Timeline base
tl_y=100; tl_x0=50; tl_x1=700
draw.line([tl_x0,tl_y,tl_x1,tl_y],fill=BLACK,width=2)
# Arrow tip
draw.polygon([(tl_x1,tl_y),(tl_x1-10,tl_y-5),(tl_x1-10,tl_y+5)],fill=BLACK)
tc("time →",tl_x1+10,tl_y,f,DARK)

# Terms layout: (start_x, width, type)
terms = [
    (50,  60,  "E",  "Term 1"),
    (110, 130, "N",  "Term 2"),
    (240, 55,  "E",  "Term 3"),
    (295, 110, "N",  ""),
    (405, 50,  "S",  "Term 4"),
    (455, 70,  "E",  ""),
    (525, 120, "N",  "Term 5"),
]
fills={"E":ELECTION,"N":NORMAL,"S":SPLIT}
th=50
for (sx,sw,tp,lbl) in terms:
    col=fills[tp]
    draw.rectangle([sx,tl_y-th//2,sx+sw,tl_y+th//2],fill=col,outline=BLACK,width=2)
    if lbl:
        tc(lbl,sx+sw//2,tl_y-th//2-14,fs,DARK)

# Legend
leg_items=[
    (ELECTION,"Elections"),
    (NORMAL,  "Normal Operation"),
    (SPLIT,   "Split Vote"),
]
lx=50
for col,label in leg_items:
    draw.rectangle([lx,150,lx+25,170],fill=col,outline=BLACK,width=1)
    tc(label,lx+70,160,fs,DARK)
    lx+=180

tc("Figure 13.22: Raft terms",W//2,H-16,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_22.webp","WEBP",quality=92)
print("Saved figure_13_22.webp")
