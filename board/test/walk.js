// Evaluates an OWL widget with Owl.js against a fixture and prints every
// node's properties and every Canvas's drawing calls as JSON -- the same walk,
// byte for byte, as the Java harness that produced <name>.expected.json from
// owl.jar, the language's reference implementation. test/owl-test.sh diffs
// the two.
//
//   node walk.js <Owl.js> <widget.owl> <fixture.json>
const fs = require('fs'), Module = require('module'), path = require('path')
function load(file) {
  const m = new Module(file)
  m.filename = file
  m.paths = Module._nodeModulePaths(path.dirname(file))
  // `.pragma library` is QML's; node needs it gone.
  m._compile(fs.readFileSync(file, 'utf8').replace(/^\.pragma library/, '//'), file)
  return m.exports
}
// random() and shuffle(list) draw the same numbers as OwlOracle.java's Fixed.
let x = 0
Math.random = () => (x = (x + 0.6180339887498949) % 1)
const Owl = load(path.resolve(process.argv[2]))
const src = fs.readFileSync(process.argv[3], 'utf8')
const fx = JSON.parse(fs.readFileSync(process.argv[4], 'utf8'))
// The expected files were made by java in en_US, where weeks start on Sunday.
Owl.setWeekStart(0)
const program = Owl.compile(src)
const host = {
  theme: n => fx.theme[n] === undefined ? null : fx.theme[n] >>> 0,
  tile: n => fx.tile[n] === undefined ? null : fx.tile[n],
  act() {}, runInto() {}, post() {}, report: m => process.stderr.write('report: ' + m + '\n'), changed() {}
}
const inst = new Owl.Instance(program, host, {})
inst.time = fx.time
program.globals.forEach((g, slot) => {
  if (g.type !== 'source') return
  const e = fx.sources[g.name]
  if (!e) return
  if ('value' in e) inst.values[slot] = Owl.fromJson(e.value)
  else if ('text' in e) {
    if (g.json) { try { inst.values[slot] = Owl.parseJson(e.text.trim()) } catch (x) { inst.values[slot] = null } }
    else inst.values[slot] = e.text.replace(/\s+$/, '')
  }
})
inst.invalidate()
inst.sample()
const q = s => JSON.stringify(s)
const show = (ev, env) => { try { return q(Owl.show(Owl.run(ev, env))) } catch (e) { return q('ERROR ' + e.message) } }
const truthy = (ev, env) => { try { const v = Owl.run(ev, env); return v !== null && v !== false && v !== 0 && v !== '' && !(Array.isArray(v) && !v.length) && !(v instanceof Map && !v.size) && v === v } catch (e) { return false } }
const items = v => Array.isArray(v) ? v : v instanceof Map ? [...v.values()] : v === null ? [] : [v]
const props = (p, env) => '{' + Object.keys(p).sort().map(k => q(k) + ':' + show(p[k], env)).join(',') + '}'
function nodes(list, env) { return list.map(n => node(n, env)).filter(s => s !== '').join(',') }
function node(n, env) {
  if (n.t === 'lets') return nodes(n.children, env.lets(n.values))
  if (n.t === 'if') return nodes(truthy(n.cond, env) ? n.then : n.orElse, env)
  if (n.t === 'for') {
    let list; try { list = Owl.run(n.list, env) } catch (e) { list = null }
    return items(list).slice(0, 1000).map((it, i) => nodes(n.body, env.child([it, i]))).filter(s => s).join(',')
  }
  if (n.t === 'use') {
    const args = n.args.map(a => { try { return Owl.run(a, env) } catch (e) { return null } })
    const inner = new Owl.Env(env.host, new Owl.Env(env.host, null, []), args)
    if (!Object.keys(n.wrapper).length) return nodes(n.body, inner)
    return '{"kind":"Box","props":' + props(n.wrapper, env) + ',"children":[' + nodes(n.body, inner) + ']}'
  }
  if (n.t === 'el') {
    let s = '{"kind":' + q(n.kind) + ',"props":' + props(n.props, env) + ',"anim":' + props(n.animations, env)
    if (n.kind === 'Canvas') s += ',"draw":[' + draws(n.children, env.child(fx.canvas)) + ']'
    else s += ',"children":[' + nodes(n.children, env) + ']'
    return s + '}'
  }
  return ''
}
function draws(list, env) {
  return list.map(n => {
    if (n.t === 'lets') return draws(n.children, env.lets(n.values))
    if (n.t === 'if') return draws(truthy(n.cond, env) ? n.then : n.orElse, env)
    if (n.t === 'for') {
      let list; try { list = Owl.run(n.list, env) } catch (e) { list = null }
      return items(list).slice(0, 1000).map((it, i) => draws(n.body, env.child([it, i]))).filter(s => s).join(',')
    }
    let s = '{"op":' + q(n.op) + ',"args":[' + n.args.map(a => show(a, env)).join(',') + '],"named":' + props(n.named, env)
    if (n.children) s += ',"ops":[' + draws(n.children, env) + ']'
    return s + '}'
  }).filter(s => s !== '').join(',')
}
// What the file says about itself: where it goes, and a face's status bar.
let out = '{"program":{"face":' + program.isFace + ',"page":' + program.page + ',"order":' + program.order
  + ',"padding":' + q(Owl.show(program.padding)) + ',"background":' + (program.background ? show(program.background, inst.env) : 'null')
  + ',"statusBar":' + (program.statusBar ? show(program.statusBar, inst.env) : 'null') + '},"root":[' + nodes(program.root, inst.env) + ']'
if (program.screen) out += ',"screen":[' + nodes(program.screen, inst.env) + ']'
console.log(out + '}')
