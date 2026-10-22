# Generates dagger-banner.svg. See NOTES.md in this directory.
import sys
from xml.sax.saxutils import escape as esc

W, H = 1200, 656
SANS = "Inter, sans-serif"
DISPLAY = "Inter Display, Inter, sans-serif"
MONO = "JetBrains Mono, monospace"

C = dict(
    bg="#0B1220", panel="#0F1A2C", panel_line="#1E2B42", stripe="#122036",
    grid="#1A2740", text="#E6EDF7", muted="#8A9BB4", dim="#5B6B84",
    ok="#4F8CFF", transient="#F5A524", error="#F25F5C", custom="#A78BFA",
    struct="#2A3A55", struct_line="#3B4D6B",
)

out = []
def add(s): out.append(s)

add(f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" role="img" '
    f'aria-label="dagger: a trace of one workflow with series, a custom parallel step, a retried DNS check, and a routed error">')
add('''  <!--
    dagger README banner: a trace view (Jaeger / Grafana Tempo style) of one dagger workflow.
    Source of truth for assets/banner/dagger-banner.png. See assets/banner/NOTES.md to regenerate.
  -->''')
add(f'  <rect width="{W}" height="{H}" rx="20" fill="{C["bg"]}"/>')

# ---- header: mark + wordmark + tagline + install chip
mx, my = 44, 62
nodes = [(mx, my, C["ok"]), (mx + 26, my - 15, C["custom"]), (mx + 26, my + 15, C["transient"]), (mx + 52, my, C["ok"])]
for a, b in [(0, 1), (0, 2), (1, 3), (2, 3)]:
    x1, y1, _ = nodes[a]; x2, y2, _ = nodes[b]
    add(f'  <path d="M {x1} {y1} L {x2} {y2}" stroke="{C["struct_line"]}" stroke-width="3" fill="none"/>')
for x, y, c in nodes:
    add(f'  <circle cx="{x}" cy="{y}" r="7.5" fill="{c}"/>')
add(f'  <text x="116" y="76" font-family="{DISPLAY}" font-weight="700" font-size="46" fill="{C["text"]}">dagger</text>')
add(f'  <text x="276" y="58" font-family="{SANS}" font-size="19" font-weight="600" fill="{C["text"]}">Typed Go workflows, built from small steps.</text>')
add(f'  <text x="276" y="84" font-family="{SANS}" font-size="17" fill="{C["muted"]}">Trace every step, retry what is transient, route what fails.</text>')
chip = "go get github.com/ajatprabha/dagger"
cw = (len(chip) + 2) * 8.4 + 32
cx = W - 40 - cw
add(f'  <rect x="{cx:.1f}" y="44" width="{cw:.1f}" height="38" rx="9" fill="{C["panel"]}" stroke="{C["panel_line"]}"/>')
add(f'  <text x="{cx + 16:.1f}" y="68" font-family="{MONO}" font-size="14" fill="{C["text"]}"><tspan fill="{C["dim"]}">$ </tspan>{esc(chip)}</text>')

# ---- trace panel
PX, PY, PW = 32, 120, W - 64
ROW_H = 34
rows = [
    # name, label, depth, start, end, kind, bar note, badge
    ("dagger:seriesStep[*Domain]", "", 0, 0, 1520, "struct", "", None),
    ("domain:validateRequest", "", 1, 0, 40, "ok", "40ms", None),
    ("domain:checkCNAME", "", 1, 48, 300, "transient", "CNAME record missing · kind=transient", ("attempt 1", C["transient"])),
    ("domain:checkCNAME", "", 1, 500, 640, "ok", "retried by middleware · 140ms", ("attempt 2", C["ok"])),
    ("dagger:resultStep[*Domain]", "", 1, 650, 1440, "struct", "", None),
    ("app:parallel[*Domain]", "[main]", 2, 655, 1250, "custom", "", ("custom Step", C["custom"])),
    ("domain:issueCertificate", "", 3, 662, 960, "error", "acme: rate limited", None),
    ("domain:configureCDN", "", 3, 662, 1240, "ok", "580ms", None),
    ("domain:pushEdgeConfig", "", 3, 662, 900, "ok", "240ms", None),
    ("domain:scheduleRequeue", "[failure]", 2, 1262, 1330, "ok", "Case(isRateLimited)", None),
    ("domain:recordStatus", "", 1, 1450, 1520, "ok", "70ms", None),
]
HEAD_H, RULER_H = 52, 32
PH = HEAD_H + RULER_H + len(rows) * ROW_H + 12
add(f'  <rect x="{PX}" y="{PY}" width="{PW}" height="{PH}" rx="14" fill="{C["panel"]}" stroke="{C["panel_line"]}"/>')

# panel header
hy = PY + 32
add(f'  <text x="{PX + 20}" y="{hy}" font-family="{SANS}" font-size="16" fill="{C["muted"]}">Trace</text>')
add(f'  <text x="{PX + 70}" y="{hy}" font-family="{MONO}" font-size="16" font-weight="700" fill="{C["text"]}">attach_domain</text>')
add(f'  <text x="{PX + 210}" y="{hy}" font-family="{MONO}" font-size="16" fill="{C["muted"]}">shop.example</text>')
stats = [("1.52s", "duration"), ("11", "spans"), ("1", "retry"), ("1", "error routed")]
sx = PX + PW - 20
parts = []
for v, k in reversed(stats):
    t = f'{v} {k}'
    w = len(v) * 9.6 + len(k) * 8.2 + 8
    sx -= w
    parts.append((sx, v, k))
    sx -= 22
for x, v, k in parts:
    add(f'  <text x="{x:.1f}" y="{hy}" font-family="{SANS}" font-size="15" fill="{C["muted"]}"><tspan font-weight="700" fill="{C["text"]}">{v}</tspan> {k}</text>')
add(f'  <path d="M {PX} {PY + HEAD_H} H {PX + PW}" stroke="{C["panel_line"]}"/>')

# timeline geometry
LX = PX + 20
TX0, TX1 = PX + 470, PX + PW - 28
TMAX = 1600
def tx(ms): return TX0 + (TX1 - TX0) * ms / TMAX

ry = PY + HEAD_H
add(f'  <text x="{LX}" y="{ry + 21}" font-family="{SANS}" font-size="13" font-weight="600" letter-spacing="0.6" fill="{C["dim"]}">STEP</text>')
body_top = ry + RULER_H
body_bot = body_top + len(rows) * ROW_H
for ms, lab in [(0, "0ms"), (400, "400ms"), (800, "800ms"), (1200, "1.2s"), (1600, "1.6s")]:
    x = tx(ms)
    anchor = "start" if ms == 0 else ("end" if ms == TMAX else "middle")
    add(f'  <text x="{x:.1f}" y="{ry + 21}" text-anchor="{anchor}" font-family="{SANS}" font-size="13" fill="{C["dim"]}">{lab}</text>')
    add(f'  <path d="M {x:.1f} {body_top} V {body_bot}" stroke="{C["grid"]}" stroke-width="1"/>')
add(f'  <path d="M {PX} {body_top} H {PX + PW}" stroke="{C["panel_line"]}"/>')
add(f'  <path d="M {TX0 - 16} {ry} V {body_bot + 6}" stroke="{C["panel_line"]}"/>')

INDENT = 20
CH = 9.0  # JetBrains Mono advance at 15px
for i, (name, label, depth, s, e, kind, note, badge) in enumerate(rows):
    y = body_top + i * ROW_H
    cy = y + ROW_H / 2
    if i % 2 == 1:
        add(f'  <rect x="{PX + 1}" y="{y}" width="{PW - 2}" height="{ROW_H}" fill="{C["stripe"]}"/>')
    # tree guides
    for d in range(depth):
        gx = LX + d * INDENT + 5
        add(f'  <path d="M {gx} {y} V {y + ROW_H}" stroke="{C["struct_line"]}" stroke-width="1.2"/>')
    nx = LX + depth * INDENT
    color = {"struct": C["struct_line"]}.get(kind, C[kind])
    add(f'  <circle cx="{nx + 5}" cy="{cy}" r="4.5" fill="{color}"/>')
    tx_name = nx + 18
    ncol = C["muted"] if kind == "struct" else C["text"]
    add(f'  <text x="{tx_name}" y="{cy + 5}" font-family="{MONO}" font-size="15" fill="{ncol}">{esc(name)}</text>')
    if label:
        add(f'  <text x="{tx_name + (len(name) + 1) * CH:.1f}" y="{cy + 5}" font-family="{MONO}" font-size="15" fill="{C["dim"]}">{esc(label)}</text>')
    after = tx_name + (len(name) + (len(label) + 1 if label else 0)) * CH + 10
    if badge:
        btxt, bcol = badge
        bw = len(btxt) * 7.4 + 18
        add(f'  <rect x="{after:.1f}" y="{cy - 11}" width="{bw:.1f}" height="22" rx="11" fill="{bcol}" fill-opacity="0.16" stroke="{bcol}" stroke-opacity="0.55"/>')
        add(f'  <text x="{after + bw / 2:.1f}" y="{cy + 4.5}" text-anchor="middle" font-family="{SANS}" font-size="12.5" font-weight="600" fill="{bcol}">{esc(btxt)}</text>')
        assert after + bw < TX0 - 20, (name, after + bw)
    else:
        assert after < TX0 - 20, (name, after)
    # bar
    x0, x1 = tx(s), tx(e)
    bh = 12 if kind == "struct" else 16
    if kind == "struct":
        add(f'  <rect x="{x0:.1f}" y="{cy - bh / 2}" width="{x1 - x0:.1f}" height="{bh}" rx="3" fill="{C["struct"]}" stroke="{C["struct_line"]}"/>')
    else:
        add(f'  <rect x="{x0:.1f}" y="{cy - bh / 2}" width="{max(x1 - x0, 4):.1f}" height="{bh}" rx="3" fill="{color}"/>')
    if note:
        ncolr = {"transient": C["transient"], "error": C["error"]}.get(kind, C["muted"])
        w = len(note) * 7.3
        if x1 + 10 + w < TX1 + 20:
            add(f'  <text x="{x1 + 10:.1f}" y="{cy + 5}" font-family="{SANS}" font-size="14" fill="{ncolr}">{esc(note)}</text>')
        else:
            add(f'  <text x="{x0 - 10:.1f}" y="{cy + 5}" text-anchor="end" font-family="{SANS}" font-size="14" fill="{ncolr}">{esc(note)}</text>')

# backoff connector between the two checkCNAME attempts
y2 = body_top + 2 * ROW_H + ROW_H / 2
y3 = body_top + 3 * ROW_H + ROW_H / 2
bx0, bx1 = tx(300), tx(500)
add(f'  <path d="M {bx0:.1f} {y2 + 8} C {bx0 + 40:.1f} {y2 + 24}, {bx1 - 40:.1f} {y3 - 20}, {bx1:.1f} {y3 - 8}" '
    f'stroke="{C["transient"]}" stroke-width="1.6" stroke-dasharray="4 4" fill="none"/>')
add(f'  <path d="M {bx1 - 7:.1f} {y3 - 13} L {bx1:.1f} {y3 - 8} L {bx1 - 8:.1f} {y3 - 5}" stroke="{C["transient"]}" stroke-width="1.6" fill="none"/>')

# ---- legend
ly = PY + PH + 34
# widths are measured from a resvg render of Inter at 14.5px
items = [(C["ok"], "step succeeded", 109), (C["transient"], "transient error, retried", 149),
         (C["error"], "error, routed by Switch", 156), (C["custom"], "your own Step type", 133),
         (C["struct_line"], "dagger control flow", 133)]
x = PX + 4
for col, txt, tw in items:
    add(f'  <rect x="{x}" y="{ly - 11}" width="14" height="14" rx="3" fill="{col}"/>')
    add(f'  <text x="{x + 22}" y="{ly + 1}" font-family="{SANS}" font-size="14.5" fill="{C["muted"]}">{esc(txt)}</text>')
    x += 22 + tw + 40
assert x < W, x
add('</svg>')
sys.stdout.write("\n".join(out) + "\n")
