"""Draws KC9K's Windows icon (Assets/kc9k.ico, Assets/kc9k.png): a shield
with a chicken's comb, in a few flat colours, at every size Windows asks
for. Run: python3 tools/make-icon.py (needs Pillow)."""
import os
from PIL import Image, ImageDraw

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "..", "Kc9k.Gui", "Assets")


def draw(size):
    s = size * 4  # drawn large, scaled down smoothly
    img = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    u = s / 64
    # The comb: three red rounded bumps above the shield.
    for cx, cy, r in ((24, 12, 7), (32, 9, 8), (40, 12, 7)):
        d.ellipse([(cx - r) * u, (cy - r) * u, (cx + r) * u, (cy + r) * u], fill=(220, 53, 69, 255))
    # The shield.
    shield = [(10, 14), (54, 14), (54, 34), (32, 60), (10, 34)]
    d.polygon([(x * u, y * u) for x, y in shield], fill=(0, 103, 192, 255))
    inner = [(15, 19), (49, 19), (49, 33), (32, 53), (15, 33)]
    d.polygon([(x * u, y * u) for x, y in inner], fill=(30, 136, 229, 255))
    # The beak: a yellow triangle, and an eye.
    d.polygon([(30 * u, 31 * u), (40 * u, 34 * u), (30 * u, 37 * u)], fill=(255, 193, 7, 255))
    d.ellipse([24 * u, 27 * u, 28 * u, 31 * u], fill=(255, 255, 255, 255))
    return img.resize((size, size), Image.LANCZOS)


os.makedirs(OUT, exist_ok=True)
sizes = [16, 20, 24, 32, 40, 48, 64, 128, 256]
big = draw(256)
big.save(os.path.join(OUT, "kc9k.ico"), sizes=[(n, n) for n in sizes])
big.save(os.path.join(OUT, "kc9k.png"))
print("written to", os.path.normpath(OUT))
