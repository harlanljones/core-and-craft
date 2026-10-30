// Drives the town's handlers with Owl.js, as the board's touch events do, and
// checks every tap in docs/mockups/duae-insulae/spec.md against the acta
// command (or the popup) it must give. Taps on the map go through the
// Canvas's onTap with px in the Canvas's own dp.
//
//   node taps.js <Owl.js> <widgets/10-town.owl> <snapshot.json>      (TZ=UTC)
const fs = require('fs'), Module = require('module'), path = require('path')
function load(file) {
  const m = new Module(file)
  m.filename = file
  m.paths = Module._nodeModulePaths(path.dirname(file))
  m._compile(fs.readFileSync(file, 'utf8').replace(/^\.pragma library/, '//'), file)
  return m.exports
}
const Owl = load(path.resolve(process.argv[2]))
const src = fs.readFileSync(process.argv[3], 'utf8')
const snap = JSON.parse(fs.readFileSync(process.argv[4], 'utf8'))
const fx = JSON.parse(fs.readFileSync(path.join(__dirname, 'fixture.base.json'), 'utf8'))
// Board.qml's pitch: 570 px wide board, 1 dp = 96/69 px.
const SCREEN_W = 570 / (96 / 69)
const NOW = 1793311200 // 2026-10-29T22:00:00Z

let acts = []
const host = {
  theme: n => fx.theme[n] === undefined ? null : fx.theme[n] >>> 0,
  tile: n => n === 'screenWidth' ? SCREEN_W : n === 'screenHeight' ? 762 / (96 / 69) : fx.tile[n] === undefined ? null : fx.tile[n],
  act(name, args) { acts.push([name, ...args.map(a => String(a))]) },
  runInto() {}, post() {}, report: m => { console.error('report: ' + m); failed++ }, changed() {}
}
let failed = 0, passed = 0
function ok(cond, label, detail) {
  if (cond) passed++
  else { failed++; console.log('FAIL ' + label + (detail === undefined ? '' : ': ' + detail)) }
}

function open(data) {
  const program = Owl.compile(src)
  const inst = new Owl.Instance(program, host, {})
  inst.time = fx.time
  program.globals.forEach((g, slot) => {
    if (g.type !== 'source') return
    if (g.name === 's') inst.values[slot] = Owl.fromJson(data)
    if (g.name === 'now') inst.values[slot] = NOW
  })
  inst.invalidate()
  inst.sample()
  const state = {}
  program.globals.forEach((g, slot) => { if (g.type === 'state') state[g.name] = slot })
  return {
    inst,
    tile: () => inst.render(false),
    screen: () => inst.render(true),
    pop: () => inst.values[state.pop],
    setPop: v => inst.setState(state.pop, v),
    take() { const a = acts; acts = []; return a }
  }
}
function find(list, pred, out = []) {
  for (const n of list || []) { if (pred(n)) out.push(n); find(n.c, pred, out) }
  return out
}
const text = n => n.p && n.p.text !== undefined ? String(n.p.text) : ''
const byText = (tree, t) => find(tree.children, n => n.kind === 'Text' && text(n) === t || n.kind === 'Button' && text(n) === t)
const shown = (tree, kind) => find(tree.children, n => n.kind === kind && n.p.visible !== false)

const ID = id => "'" + id + "'"
const board = '--via board'

// ------------------------------------------------------------------ a busy day
const w = open(snap)

// Town tile on the board -> screen()
w.tile().onTap()
ok(JSON.stringify(w.take()) === '[["screen"]]', 'tile tap opens the screen')

// ...and so does the still of the islands inside it
find(w.tile().children, n => n.kind === 'Canvas')[0].h.onTap(40, 40)
ok(JSON.stringify(w.take()) === '[["screen"]]', "a tap on the tile's still opens the screen")

let sc = w.screen()
// X -> close()
byText(sc, '✕')[0].h.onTap(); ok(JSON.stringify(w.take()) === '[["close"]]', 'X closes the map')

const q = snap.quests.filter(x => !x.done_today)[0]
// quest done / undo
byText(sc, '✓')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest done ${ID(q.id)} ${board}`]]), 'done command')
const d = snap.quests.find(x => x.done_today)
byText(sc, '↺')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest undo ${ID(d.id)} ${board}`]]), 'undo command')

// ⋯ -> snooze 1d/3d/7d, priority up/down, drop
byText(sc, '⋯')[0].h.onTap(); ok(w.pop() === 'menu:' + q.id, 'menu opens', w.pop())
sc = w.screen()
const day = n => new Date((NOW + n * 86400) * 1000).toISOString().slice(0, 10)
for (const [label, n] of [['1d', 1], ['3d', 3], ['7d', 7]]) {
  w.setPop('menu:' + q.id); sc = w.screen()
  byText(sc, label)[0].h.onTap()
  ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest snooze ${ID(q.id)} --until ${day(n)} ${board}`]]), 'snooze ' + label)
}
for (const [label, p] of [['priority ↑', 1], ['priority ↓', 3]]) {
  w.setPop('menu:' + q.id); sc = w.screen()
  byText(sc, label)[0].h.onTap()
  ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest edit ${ID(q.id)} --priority ${p} ${board}`]]), label)
}
w.setPop('menu:' + q.id); sc = w.screen(); byText(sc, 'drop')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest drop ${ID(q.id)} ${board}`]]) && w.pop() === '', 'drop')
w.setPop('')

// proposals: "review" and a boat open the popup; accept / decline run as manual
const p0 = snap.proposed[0]
sc = w.screen(); byText(sc, 'review')[0].h.onTap(); ok(w.pop() === 'prop:' + p0.id, 'review opens the proposal', w.pop())
sc = w.screen(); byText(sc, 'accept')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest accept ${ID(p0.id)} ${board}`]]), 'accept')
w.setPop('prop:' + p0.id); sc = w.screen(); byText(sc, 'decline')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta quest decline ${ID(p0.id)} ${board}`]]), 'decline')
w.setPop('')

// ---- map taps (px at the brief's grid, scaled to this board)
const scale = (SCREEN_W - 24) / 582
const at = (i, j) => [166 + (i - j) * 58, 214 + (i + j) * 29]
const G = 1.32
const spot = { career: [0, 0], outlook: [1, 0], relationships: [0, 1], economic: [1, 1], competence: [2.6, -1], recovery: [3.6, -1], agents: [2.6, 0], faith: [3.6, 0], time: [3.1, 1] }
const tapMap = (x, y) => find(w.screen().children, n => n.kind === 'Canvas')[0].h.onTap(x * scale, y * scale)

// a lot: the diamond's front corner is clear of every tower and sign
for (const id of Object.keys(spot)) {
  w.setPop('')
  const [i, j] = [spot[id][0] * G, spot[id][1] * G]
  const [x, y] = at(i + 0.5, j + 1.05) // low in the lot's grown diamond, on its front edge
  tapMap(x, y)
  ok(w.pop() === 'lot:' + id, 'lot tap ' + id, w.pop())
}
w.setPop('')
tapMap(5, 5); ok(w.pop() === '', 'a tap on open water does nothing', w.pop())

// a boat: the first proposal's, moored off its pillar's lot
{
  const [i, j] = [spot[p0.pillar][0] * G, spot[p0.pillar][1] * G]
  const [bx, by] = at(i + 1.25, j + 1.0)
  tapMap(bx + 18, by + 26)
  ok(w.pop() === 'prop:' + p0.id, 'boat tap opens the proposal', w.pop())
}
w.setPop('')

// ? bubbles open the check-in; send answers it, later leaves it open
const c0 = snap.checkins.find(c => c.open)
sc = w.screen(); byText(sc, '? ' + snap.checkins.filter(c => c.open).length + ' waiting')[0].h.onTap()
ok(w.pop() === 'ask:' + c0.id, '? N waiting opens the check-in', w.pop())
sc = w.screen()
find(sc.children, n => n.kind === 'Input' && n.p.placeholder === 'your answer')[0].h.onChange("22:30 to 06:30  it's fine")
sc = w.screen(); byText(sc, 'later')[0].h.onTap(); ok(w.pop() === '' && w.take().length === 0, 'later closes and runs nothing')
w.setPop('ask:' + c0.id)
sc = w.screen(); byText(sc, 'send')[0].h.onTap()
ok(w.take().length === 0, 'later drops the typed answer: reopened, send runs nothing')
w.setPop('ask:' + c0.id)
find(w.screen().children, n => n.kind === 'Input' && n.p.placeholder === 'your answer')[0].h.onChange("22:30 to 06:30  it's fine")
sc = w.screen(); byText(sc, 'send')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta checkin answer ${ID(c0.id)} --text '22:30 to 06:30  it'\\''s fine' ${board}`]]), 'check-in send', JSON.stringify(acts))
// an answer with quotes and $(...) stays one quoted word; an empty one runs nothing
{
  const evil = `a "b" 'c' $(touch /tmp/pwned) \`id\`; rm -rf x`
  w.setPop('ask:' + c0.id)
  find(w.screen().children, n => n.kind === 'Input' && n.p.placeholder === 'your answer')[0].h.onChange(evil)
  byText(w.screen(), 'send')[0].h.onTap()
  const got = w.take()
  const q = "'" + evil.replace(/'/g, "'\\''") + "'"
  ok(JSON.stringify(got) === JSON.stringify([['run', `acta checkin answer ${ID(c0.id)} --text ${q} ${board}`]]), 'check-in injection stays quoted', JSON.stringify(got))
  ok(w.pop() === '', 'check-in send closes the popup')
  w.setPop('ask:' + c0.id)
  find(w.screen().children, n => n.kind === 'Input' && n.p.placeholder === 'your answer')[0].h.onChange('   ')
  byText(w.screen(), 'send')[0].h.onTap()
  ok(w.take().length === 0 && w.pop() === 'ask:' + c0.id, 'empty answer runs nothing and stays open')
}
w.setPop('')

// the Input: #Pillar picks the pillar, quoting keeps the title whole
function add(text) {
  const sc = w.screen()
  const inp = find(sc.children, n => n.kind === 'Input' && /add a quest/.test(n.p.placeholder))[0]
  inp.h.onChange(text)
  w.screen().children // re-render after the draft changed
  find(w.screen().children, n => n.kind === 'Input' && /add a quest/.test(n.p.placeholder))[0].h.onSubmit(text)
  return w.take()
}
ok(JSON.stringify(add("Buy 'milk' #Eco")) === JSON.stringify([['run', `acta quest add --pillar economic --title ${"'Buy '\\''milk'\\'''"} ${board}`]]), 'add with #Economic', JSON.stringify(acts))
ok(add('Plain title').length === 0, 'add without a tag runs nothing')
ok(add('Call #re').length === 0, 'an ambiguous tag runs nothing')

// a lot's "add a quest" prefills the Input
w.setPop('lot:career'); sc = w.screen(); byText(sc, 'add a quest')[0].h.onTap()
ok(w.pop() === '', 'add a quest closes the popup')
{ const t = find(w.screen().children, n => n.kind === 'Input' && /add a quest/.test(n.p.placeholder))[0].p.text; ok(t === '#Career', 'add a quest prefills the Input', JSON.stringify(t)) }
w.setPop('')

// a lot asks its pillar's agent; the latest ask and its answer show there
w.setPop('lot:time'); sc = w.screen()
ok(byText(sc, 'The Thursday sync: it repeats the Monday notes.').length === 1, "a lot shows its agent's answer")
w.setPop('lot:recovery'); sc = w.screen()
ok(byText(sc, 'waiting for the answer').length === 1, 'a lot shows its open ask as waiting')
byText(sc, 'ask')[0].h.onTap(); ok(w.take().length === 0 && w.pop() === 'lot:recovery', 'ask with no question runs nothing')
find(sc.children, n => n.kind === 'Input' && n.p.placeholder === 'your question')[0].h.onChange("  What's a good wind-down?  ")
sc = w.screen(); byText(sc, 'ask')[0].h.onTap()
ok(JSON.stringify(w.take()) === JSON.stringify([['run', `acta ask --pillar 'recovery' --text 'What'\\''s a good wind-down?' ${board}`]]), 'lot ask', JSON.stringify(acts))
ok(w.pop() === '', 'ask closes the popup')

// typed text belongs to the popup it was typed in: dismissing it, or opening
// another lot or check-in, clears it, so it never goes to another pillar
{
  const input = ph => find(w.screen().children, n => n.kind === 'Input' && n.p.placeholder === ph)[0]
  const dismiss = () => shown(w.screen(), 'Popup').forEach(n => n.h.onDismiss && n.h.onDismiss())
  w.setPop('lot:economic')
  input('your question').h.onChange('Can I afford the trip?')
  ok(input('your question').p.text === 'Can I afford the trip?', 'the question box shows what ask will send', JSON.stringify(input('your question').p.text))
  dismiss(); ok(w.pop() === '', 'dismiss closes the lot', w.pop())
  w.setPop('lot:relationships')
  ok((input('your question').p.text ?? '') === '', 'another lot opens with an empty question', JSON.stringify(input('your question').p.text))
  byText(w.screen(), 'ask')[0].h.onTap()
  ok(w.take().length === 0 && w.pop() === 'lot:relationships', 'ask after dismiss sends nothing to the other lot', JSON.stringify(acts))
  // switching lots with the popup open clears it too
  w.setPop('lot:economic'); input('your question').h.onChange('Can I afford the trip?')
  w.setPop('lot:relationships'); byText(w.screen(), 'ask')[0].h.onTap()
  ok(w.take().length === 0, 'switching lots drops the question')
  // the same for a check-in answer
  const [ca, cb] = snap.checkins.filter(c => c.open)
  w.setPop('ask:' + ca.id); input('your answer').h.onChange('22:30')
  ok(input('your answer').p.text === '22:30', 'the answer box shows what send will send', JSON.stringify(input('your answer').p.text))
  dismiss()
  w.setPop('ask:' + cb.id)
  ok((input('your answer').p.text ?? '') === '', 'another check-in opens with an empty answer')
  byText(w.screen(), 'send')[0].h.onTap()
  ok(w.take().length === 0 && w.pop() === 'ask:' + cb.id, 'send after dismiss answers nothing', JSON.stringify(acts))
  w.setPop('')
}

// ------------------------------------------------------------------ a day not yet kept
const open2 = open(Object.assign({}, snap, { streak: { days: 3, today: false } }))
{
  const sc2 = open2.screen()
  ok(byText(sc2, 'today kept').length === 0 && byText(sc2, 'rest day ›').length === 1, 'rail offers rest day')
  byText(sc2, 'rest day ›')[0].h.onTap()
  ok(JSON.stringify(open2.take()) === JSON.stringify([['run', 'acta rest --via board']]), 'rest day command (rail)')
  // "rest day ›" above the bridge
  const oi = spot.outlook[0] * G, ai = spot.agents[0] * G, oj = spot.outlook[1] * G
  const [mx, my] = at((oi + 1.1 + ai) / 2, oj + 0.5)
  find(open2.screen().children, n => n.kind === 'Canvas')[0].h.onTap((mx - 78) * scale, (my + 15) * scale)
  ok(JSON.stringify(open2.take()) === JSON.stringify([['run', 'acta rest --via board']]), 'rest day command (map)')
}

// ------------------------------------------------------------------ many proposals: every boat stays on the canvas, by its own lot
{
  const pillars = Object.keys(spot)
  const many = Object.assign({}, snap, { proposed: pillars.map((pillar, n) => Object.assign({}, snap.proposed[0], { id: 'q_boat' + n, pillar })) })
  const wm = open(many)
  const cv = find(wm.screen().children, n => n.kind === 'Canvas')[0]
  pillars.forEach((pillar, n) => {
    const [bx, by] = at(spot[pillar][0] * G + 1.25, spot[pillar][1] * G + 1.0)
    ok(bx + 29 <= 582 && by + 32 <= 470, 'boat for ' + pillar + ' is on the canvas', bx + ',' + by)
    wm.setPop('')
    cv.h.onTap((bx + 18) * scale, (by + 26) * scale)
    ok(wm.pop() === 'prop:q_boat' + n, 'boat tap for ' + pillar, wm.pop())
  })
}

console.log(`${failed ? 'FAIL' : 'ok  '} town taps: ${passed} passed, ${failed} failed`)
process.exit(failed ? 1 : 0)
