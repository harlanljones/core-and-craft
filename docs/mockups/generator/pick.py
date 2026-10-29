"""The pick on its own page: Duae Insulae, in both themes and all three states, with its taps and notes."""
import os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import maps_page2 as page
from maps2 import DuaeInsulae

page.CONCEPTS[:] = [DuaeInsulae]
page.D[:] = [dict(c=DuaeInsulae)]
page.OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "duae-insulae", "index.html")

if __name__ == "__main__":
    page.main()
    html = open(page.OUT).read()
    for a, b in [
        ("<title>CORE &amp; CRAFT Town Views</title>", "<title>Duae Insulae Town</title>"),
        ("the town from above · three concepts</div>\n  <h1>The town from above</h1>",
         "the town, full screen · the pick</div>\n  <h1>Duae Insulae<small style=\"font:400 20px var(--serif);color:var(--ink-2);margin-left:.4em;font-style:italic\">two islands in isometric</small></h1>"),
        ("Fit three", "Fit"),
        ("<p>Forum with Castra, seen from above the way Stardew Valley shows a town, or in isometric. Each concept keeps Forum's nine buildings and gives each its Castra block with the whole week, but draws them a different way, clusters them differently and adds something of its own.</p>",
         "<p>The pick, Oct 29. The town as an isometric map that fills the board: C.O.R.E. and C.R.A.F.T. as two islands joined by a bridge, each pillar a tower that gains a storey per level, its Castra week laid in front as seven floor tiles. This page is the reference for the OWL build; spec.md beside it says how to build it.</p>"),
        ("<p>A is drawn in the board's mono glyphs, so the house style stays exactly as it is. B and C are drawn with Canvas shapes: B as isometric towers, C as 16 px tiles like Stardew itself. They look more like a game and less like the typing tile next to them.</p>",
         "<p>It's drawn with Canvas shapes, not glyphs, so it looks more like a game and less like the typing tile next to it. The rail under the map stays in the mono font.</p>"),
        ("<p>Two of these leave the glyph style for Canvas shapes. The comparison says which, and what it costs.</p>", ""),
        ('<h2>Comparison</h2>', '<h2>Summary</h2>'),
    ]:
        html = html.replace(a, b)
    open(page.OUT, "w").write(html)
    print(page.OUT, len(html))
