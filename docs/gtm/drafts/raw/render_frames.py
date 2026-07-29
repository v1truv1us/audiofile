import sys
from PIL import Image, ImageDraw, ImageFont

W, H, prefix = int(sys.argv[1]), int(sys.argv[2]), sys.argv[3]
BG = (14, 16, 20)
TXT = (245, 245, 247)
ACC = (242, 169, 59)
FP = "/System/Library/Fonts/Supplemental/Arial Rounded Bold.ttf"

def f(sz):
    return ImageFont.truetype(FP, sz)

beats = [
    [("Every digger’s worst fear…", 64, TXT)],
    [("Buying a record", 84, TXT), ("you already own.", 84, TXT)],
    [("Your collection app", 84, TXT), ("won’t load in the store.", 64, TXT)],
    [("AudioFile", 140, ACC), ("Own it? Check in 1 second.", 56, TXT)],
    [("Add it.", 84, TXT), ("Share the hunt.", 84, ACC)],
    [("audiofile.app", 96, ACC), ("Free", 56, TXT)],
]
gap = 24
for i, lines in enumerate(beats, 1):
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    total = sum(sz for _, sz, _ in lines) + gap * (len(lines) - 1)
    y0 = (H - total) / 2
    cy = y0
    for text, sz, col in lines:
        d.text((W / 2, cy + sz / 2), text, font=f(sz), fill=col, anchor="mm")
        cy += sz + gap
    img.save(f"docs/gtm/drafts/raw/{prefix}{i}.png")
print(f"{prefix}frames done")
