"""Tiny rasterizer for the mockup SVG subset, so maps can be eyeballed without a browser."""
import os, re, sys
import xml.etree.ElementTree as ET
from PIL import Image, ImageDraw, ImageFont
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from maps2 import CONCEPTS
from mapkit import STATES
from common import THEMES
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "duae-insulae", "preview")
FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
if not os.path.exists(FONT):
    import glob; FONT = (glob.glob("/usr/share/fonts/**/*Mono*.ttf", recursive=True) or [None])[0]
S = 2  # supersample

def col(pal, name, a=1.0):
    h = pal[name].lstrip("#"); return (int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16), int(255 * a))

def styles(el, pal):
    fill = stroke = None
    cls = el.get("class", "")
    for c in cls.split():
        if c.startswith("fl-"): fill = c[3:]
        if c.startswith("st-"): stroke = c[3:]
    if el.get("fill") == "none": fill = None
    elif fill is None and stroke is None and el.tag in ("rect", "polygon", "circle", "ellipse"): fill = "foreground"
    return fill, stroke, float(el.get("opacity", 1)), float(el.get("stroke-width", 1))

def render(svg, pal, w, h):
    root = ET.fromstring(svg)
    img = Image.new("RGBA", (w * S, h * S), col(pal, "background"))
    for el in root.iter():
        tag = el.tag
        if tag not in ("rect", "polygon", "circle", "ellipse", "line", "text"): continue
        fill, stroke, op, sw = styles(el, pal)
        lay = Image.new("RGBA", img.size, (0, 0, 0, 0)); d = ImageDraw.Draw(lay)
        F = lambda k, dflt=0: float(el.get(k, dflt)) * S
        fc = col(pal, fill, op) if fill else None
        sc = col(pal, stroke, op) if stroke else None
        if tag == "rect":
            x, y, ww, hh = F("x"), F("y"), F("width"), F("height")
            if el.get("width", "").endswith("%"): x, y, ww, hh = 0, 0, w * S, h * S
            d.rectangle([x, y, x + ww, y + hh], fill=fc, outline=sc, width=int(sw * S) if sc else 0)
        elif tag == "polygon":
            pts = [tuple(float(v) * S for v in p.split(",")) for p in el.get("points").split()]
            d.polygon(pts, fill=fc, outline=sc)
        elif tag in ("circle", "ellipse"):
            cx, cy = F("cx"), F("cy")
            rx = F("r") if tag == "circle" else F("rx"); ry = F("r") if tag == "circle" else F("ry")
            d.ellipse([cx - rx, cy - ry, cx + rx, cy + ry], fill=fc, outline=sc, width=int(sw * S) if sc else 0)
        elif tag == "line":
            d.line([F("x1"), F("y1"), F("x2"), F("y2")], fill=sc or fc, width=max(1, int(sw * S)))
        elif tag == "text":
            size = float(el.get("font-size", 12)) * S
            font = ImageFont.truetype(FONT, int(size))
            parts = []
            if el.text and el.text.strip(): parts.append((el.text, fill or "foreground"))
            for t in el:
                f2, _, _, _ = styles(t, pal); parts.append((t.text or "", f2 or fill or "foreground"))
            total = sum(font.getlength(p) for p, _ in parts)
            x, y = F("x"), F("y")
            anc = el.get("text-anchor", "start")
            x -= total / 2 if anc == "middle" else total if anc == "end" else 0
            for p, c in parts:
                d.text((x, y), p, font=font, fill=col(pal, c, op), anchor="lm"); x += font.getlength(p)
        img = Image.alpha_composite(img, lay)
    return img.resize((w, h), Image.LANCZOS)

if __name__ == "__main__":
    th = sys.argv[1] if len(sys.argv) > 1 else "tokyo-night"
    pal = THEMES[th]
    only = sys.argv[2:]
    for C in CONCEPTS[1:]:
        if only and C.SLUG not in only:
            continue
        for k, l, s in STATES:
            svg = C.canvas(s).replace('font-family="var(--mono)"', "")
            os.makedirs(OUT, exist_ok=True)
            render(svg, pal, C.WPX, C.HPX).save(os.path.join(OUT, f"{C.SLUG}-{k}-{th}.png"))
    print("ok", FONT)
