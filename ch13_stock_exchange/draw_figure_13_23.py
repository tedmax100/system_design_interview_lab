from PIL import Image, ImageDraw, ImageFont

W, H = 720, 300
img = Image.new("RGB", (W, H), (255, 255, 255))
draw = ImageDraw.Draw(img)

BLACK=(0,0,0); GRAY=(160,160,160); DARK=(60,60,60); BLUE=(100,150,220)

try:
    f  = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 13)
    fb = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)
    fs = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 11)
except:
    f=fb=fs=ImageFont.load_default()

def tc(txt,cx,cy,fnt=f,color=BLACK):
    bb=draw.textbbox((0,0),txt,font=fnt)
    draw.text((cx-(bb[2]-bb[0])//2, cy-(bb[3]-bb[1])//2),txt,fill=color,font=fnt)

def timeline(y,events,r=18):
    x0,x1=50,660
    draw.line([x0,y,x1,y],fill=BLACK,width=2)
    draw.polygon([(x1,y),(x1-10,y-5),(x1-10,y+5)],fill=BLACK)
    tc("Time",x1+15,y,fs,DARK)
    for ex,lbl in events:
        draw.ellipse([ex-r,y-r,ex+r,y+r],fill=(220,230,255),outline=BLUE,width=2)
        tc(lbl,ex,y,fs,DARK)

# Top: events spread unevenly in real time
tc("Real time (events spread unevenly):",60,40,f,DARK)
top_events=[
    (80,  "1"),(170,"2"),(220,"3"),(360,"4"),(440,"5"),(580,"6")
]
timeline(90, top_events)

# Separator
draw.line([50,155,670,155],fill=GRAY,width=1)
tc("→  After replay / recovery (events compressed):",60,175,f,DARK)

# Bottom: events compressed together
bot_events=[(80+i*60,str(i+1)) for i in range(6)]
timeline(225, bot_events)

tc("Figure 13.23: Time in event sourcing",W//2,H-16,fb)

img.save("/home/nathan/Project/system_design_lab/ch13_stock_exchange/figure_13_23.webp","WEBP",quality=92)
print("Saved figure_13_23.webp")
