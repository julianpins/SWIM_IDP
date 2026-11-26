#!/usr/bin/env python3
"""Turn JSON trace into a standalone HTML animation.

    go run ./cmd/swimsim -nodes 5 -crash n3@2s -json > trace.json
    python3 scripts/trace_animation.py trace.json -o trace.html

Standard library only. The page shows messages in flight (Ping, Ack, PingReq),
crashes, partitions and, per node, the worst state any live node assigns to it.
Real flights last milliseconds, so each message is drawn for at least
--min-flight protocol periods; the timeline itself is not stretched.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

MAX_NODES = 10
NS_PER_S = 1e9


def seconds(ns: int) -> float:
    return ns / NS_PER_S


def extract(run: dict[str, Any]) -> dict[str, Any]:
    """Reduce the full trace to what the animation draws."""
    nodes: list[str] = run["Config"]["Nodes"]
    if len(nodes) > MAX_NODES:
        raise ValueError(f"{len(nodes)} nodes; the animation supports at most {MAX_NODES}")

    messages: list[dict[str, Any]] = []
    log: list[dict[str, Any]] = []
    crashes: dict[str, float] = {}
    partitions: list[dict[str, Any]] = []  # {t, groups}; groups == [] means healed

    for entry in run["Entries"]:
        event = entry["Event"]
        at = seconds(event["At"])
        kind = event["Kind"]
        node = event["Node"]
        if kind == "Deliver":
            msg = event["Msg"]
            messages.append({
                "sent": seconds(event["Sent"]),
                "at": at,
                "from": event["From"],
                "to": node,
                "kind": msg["Kind"],
                "probe": msg["Probe"],
                "target": msg["Target"],
                "updates": len(msg["Updates"] or []),
            })
        elif kind == "Boot":
            log.append({"t": at, "kind": "boot", "text": f"{node} boots"})
        elif kind == "Inject":
            fault = event["Fault"]
            if fault["Kind"] == "Crash":
                crashes[fault["Node"]] = at
                log.append({"t": at, "kind": "crash", "text": f"{fault['Node']} crashes"})
            elif fault["Kind"] == "Partition":
                partitions.append({"t": at, "groups": fault["Groups"]})
                log.append({"t": at, "kind": "fault", "text": f"partition {fault['Groups']}"})
            elif fault["Kind"] == "HealPartition":
                partitions.append({"t": at, "groups": []})
                log.append({"t": at, "kind": "fault", "text": "partition healed"})
        for change in entry["Changes"] or []:
            before = change["From"] if change["Known"] else "new"
            log.append({
                "t": at,
                "kind": "change",
                "observer": node,
                "member": change["Member"],
                "state": change["To"],
                "text": f"{node}: {change['Member']} {before} → {change['To']} (inc {change['Incarnation']})",
            })

    for m in messages:  # arrived at a crashed node: drawn, but marked as dropped
        crash = crashes.get(m["to"])
        m["dropped"] = crash is not None and m["at"] >= crash

    return {
        "nodes": nodes,
        "period": seconds(run["Config"]["Protocol"]["ProtocolPeriod"]),
        "until": seconds(run["Until"]),
        "seed": run["Seed"],
        "hash": run["Hash"],
        "messages": messages,
        "log": log,
        "crashes": crashes,
        "partitions": partitions,
    }


def render(data: dict[str, Any], min_flight: float) -> str:
    payload = json.dumps(data).replace("</", "<\\/")  # never close the script tag early
    return PAGE.replace("__DATA__", payload).replace("__MIN_FLIGHT__", repr(min_flight))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("trace", help="JSON from swimsim -json, or - for stdin")
    parser.add_argument("-o", "--output", default="trace.html", help="HTML file to write (default trace.html)")
    parser.add_argument("--min-flight", type=float, default=0.15,
                        help="minimum drawn flight time, in protocol periods (default 0.15)")
    args = parser.parse_args()

    raw = sys.stdin.read() if args.trace == "-" else Path(args.trace).read_text()
    try:
        data = extract(json.loads(raw))
    except (ValueError, KeyError) as err:
        print(f"trace_animation: {err}", file=sys.stderr)
        return 1
    Path(args.output).write_text(render(data, args.min_flight))
    print(f"wrote {args.output}: {len(data['nodes'])} nodes, {len(data['messages'])} messages, "
          f"{data['until']:.1f}s")
    return 0


PAGE = r"""<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>SWIM trace</title>
<style>
  :root {
    --bg: #fafaf9; --fg: #1c1917; --muted: #78716c; --line: #e7e5e4; --panel: #ffffff;
    --alive: #16a34a; --suspect: #ea580c; --dead: #dc2626; --crashed: #57534e;
    --ping: #2563eb; --ack: #16a34a; --pingreq: #9333ea;
  }
  @media (prefers-color-scheme: dark) {
    :root { --bg: #1c1917; --fg: #f5f5f4; --muted: #a8a29e; --line: #3a3532; --panel: #292524; }
  }
  * { box-sizing: border-box; }
  body { margin: 0; background: var(--bg); color: var(--fg);
         font: 14px/1.4 system-ui, sans-serif; display: flex; flex-direction: column; height: 100vh; }
  header { padding: 10px 16px; display: flex; gap: 12px; align-items: center; flex-wrap: wrap;
           border-bottom: 1px solid var(--line); }
  header .meta { color: var(--muted); font-size: 12px; margin-left: auto; font-family: ui-monospace, monospace; }
  button, select { font: inherit; padding: 4px 10px; border: 1px solid var(--line); border-radius: 6px;
                   background: var(--panel); color: var(--fg); cursor: pointer; }
  #slider { flex: 1; min-width: 200px; }
  #clock { font-family: ui-monospace, monospace; min-width: 90px; }
  main { flex: 1; display: flex; min-height: 0; }
  #stage { flex: 1; min-width: 0; position: relative; }
  canvas { position: absolute; inset: 0; width: 100%; height: 100%; }
  aside { width: 360px; border-left: 1px solid var(--line); padding: 12px; overflow: hidden;
          display: flex; flex-direction: column; gap: 10px; }
  .legend { display: grid; grid-template-columns: 1fr 1fr; gap: 4px 12px; font-size: 12px; color: var(--muted); }
  .legend span::before { content: ""; display: inline-block; width: 10px; height: 10px; border-radius: 50%;
                         margin-right: 6px; vertical-align: -1px; background: var(--c); }
  #log { flex: 1; min-height: 0; overflow-y: auto; font: 12px/1.5 ui-monospace, monospace; }
  #log div { padding-left: 2em; text-indent: -2em; overflow-wrap: anywhere; }
  #log .t { color: var(--muted); margin-right: 8px; }
  #log .crash { color: var(--dead); font-weight: 600; }
  #log .fault { color: var(--pingreq); }
  #log .boot { color: var(--muted); }
  h2 { margin: 0; font-size: 12px; text-transform: uppercase; letter-spacing: .05em; color: var(--muted); }
</style>
</head>
<body>
<header>
  <button id="play">▶ Play</button>
  <button id="restart">⏮</button>
  <label>speed <select id="speed">
    <option value="0.05">0.05×</option><option value="0.1">0.1×</option>
    <option value="0.25" selected>0.25×</option><option value="0.5">0.5×</option>
    <option value="1">1×</option><option value="2">2×</option><option value="5">5×</option>
  </select></label>
  <span id="clock">0.000 s</span>
  <input id="slider" type="range" min="0" step="0.001">
  <span class="meta" id="meta"></span>
</header>
<main>
  <div id="stage"><canvas id="canvas"></canvas></div>
  <aside>
    <h2>Legend</h2>
    <div class="legend">
      <span style="--c: var(--ping)">Ping</span><span style="--c: var(--alive)">Alive (all views)</span>
      <span style="--c: var(--ack)">Ack</span><span style="--c: var(--suspect)">Suspect (some view)</span>
      <span style="--c: var(--pingreq)">PingReq</span><span style="--c: var(--dead)">Dead (some view)</span>
      <span style="--c: transparent; outline: 1px solid var(--muted); border-radius: 50%">not booted</span>
      <span style="--c: var(--crashed)">crashed</span>
    </div>
    <h2>Events</h2>
    <div id="log"></div>
  </aside>
</main>
<script>
const DATA = __DATA__;
const MIN_FLIGHT = __MIN_FLIGHT__ * DATA.period;  // seconds a message is drawn at least
const RANK = { Alive: 0, Suspect: 1, Dead: 2 };

const canvas = document.getElementById("canvas");
const ctx = canvas.getContext("2d");
const slider = document.getElementById("slider");
const clock = document.getElementById("clock");
const playButton = document.getElementById("play");
const speedSelect = document.getElementById("speed");
const logBox = document.getElementById("log");
slider.max = DATA.until;
document.getElementById("meta").textContent =
  `${DATA.nodes.length} nodes · seed ${DATA.seed} · ${DATA.messages.length} messages · hash ${DATA.hash.slice(0, 12)}`;

const boots = {};
for (const e of DATA.log) if (e.kind === "boot") boots[e.text.split(" ")[0]] = e.t;
const changes = DATA.log.filter(e => e.kind === "change");

let t = 0, playing = false, lastFrame = null;

function css(name) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }

function layout() {
  const w = canvas.clientWidth, h = canvas.clientHeight;
  const r = Math.min(w, h) * 0.36;
  const pos = {};
  DATA.nodes.forEach((id, i) => {
    const a = -Math.PI / 2 + (2 * Math.PI * i) / DATA.nodes.length;
    pos[id] = { x: w / 2 + r * Math.cos(a), y: h / 2 + r * Math.sin(a) };
  });
  return pos;
}

function crashed(id, at) { return DATA.crashes[id] !== undefined && DATA.crashes[id] <= at; }

// worst state any live observer assigns to each member at time `at`, plus how many observers.
function views(at) {
  const view = {};  // observer -> member -> state
  for (const c of changes) {
    if (c.t > at) break;
    (view[c.observer] ??= {})[c.member] = c.state;
  }
  const result = {};
  for (const id of DATA.nodes) result[id] = { state: "Alive", Suspect: 0, Dead: 0 };
  for (const [observer, members] of Object.entries(view)) {
    if (crashed(observer, at)) continue;
    for (const [member, state] of Object.entries(members)) {
      const r = result[member];
      if (!r) continue;
      if (state !== "Alive") r[state]++;
      if (RANK[state] > RANK[r.state]) r.state = state;
    }
  }
  return result;
}

function blockedLinks(at) {
  let groups = [];
  for (const p of DATA.partitions) if (p.t <= at) groups = p.groups;
  const group = {};
  groups.forEach((g, i) => g.forEach(id => (group[id] = i)));
  return (a, b) => groups.length > 0 && group[a] !== group[b];
}

function draw() {
  const dpr = window.devicePixelRatio || 1;
  const w = canvas.clientWidth, h = canvas.clientHeight;
  if (canvas.width !== w * dpr || canvas.height !== h * dpr) {
    canvas.width = w * dpr; canvas.height = h * dpr;
  }
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);
  const pos = layout();
  const blocked = blockedLinks(t);

  // links
  for (let i = 0; i < DATA.nodes.length; i++) {
    for (let j = i + 1; j < DATA.nodes.length; j++) {
      const a = DATA.nodes[i], b = DATA.nodes[j];
      ctx.beginPath();
      ctx.moveTo(pos[a].x, pos[a].y); ctx.lineTo(pos[b].x, pos[b].y);
      if (blocked(a, b)) { ctx.strokeStyle = css("--dead"); ctx.setLineDash([4, 4]); ctx.lineWidth = 1; }
      else { ctx.strokeStyle = css("--line"); ctx.setLineDash([]); ctx.lineWidth = 1; }
      ctx.stroke();
    }
  }
  ctx.setLineDash([]);

  // messages in flight
  const colors = { Ping: css("--ping"), Ack: css("--ack"), PingReq: css("--pingreq") };
  for (const m of DATA.messages) {
    const duration = Math.max(m.at - m.sent, MIN_FLIGHT);
    if (t < m.sent || t > m.sent + duration) continue;
    const f = (t - m.sent) / duration;
    const a = pos[m.from], b = pos[m.to];
    const x = a.x + (b.x - a.x) * f, y = a.y + (b.y - a.y) * f;
    ctx.strokeStyle = colors[m.kind]; ctx.globalAlpha = 0.35; ctx.lineWidth = 2;
    ctx.beginPath(); ctx.moveTo(a.x, a.y); ctx.lineTo(x, y); ctx.stroke();
    ctx.globalAlpha = m.dropped ? 0.35 : 1;
    ctx.fillStyle = colors[m.kind];
    ctx.beginPath(); ctx.arc(x, y, m.updates > 0 ? 6 : 4.5, 0, 2 * Math.PI); ctx.fill();
    if (m.updates > 0) {  // ring: the message piggybacks membership updates
      ctx.strokeStyle = css("--fg"); ctx.lineWidth = 1;
      ctx.beginPath(); ctx.arc(x, y, 8, 0, 2 * Math.PI); ctx.stroke();
    }
    ctx.globalAlpha = 1;
  }

  // nodes
  const v = views(t);
  const radius = Math.max(16, Math.min(28, Math.min(w, h) / 22));
  for (const id of DATA.nodes) {
    const p = pos[id];
    const booted = boots[id] !== undefined && boots[id] <= t;
    ctx.beginPath(); ctx.arc(p.x, p.y, radius, 0, 2 * Math.PI);
    if (crashed(id, t)) {
      ctx.fillStyle = css("--crashed"); ctx.fill();
      ctx.strokeStyle = css("--bg"); ctx.lineWidth = 3;
      const d = radius * 0.45;
      ctx.beginPath(); ctx.moveTo(p.x - d, p.y - d); ctx.lineTo(p.x + d, p.y + d);
      ctx.moveTo(p.x + d, p.y - d); ctx.lineTo(p.x - d, p.y + d); ctx.stroke();
    } else if (!booted) {
      ctx.strokeStyle = css("--muted"); ctx.lineWidth = 2; ctx.setLineDash([3, 3]); ctx.stroke(); ctx.setLineDash([]);
    } else {
      ctx.fillStyle = css("--" + v[id].state.toLowerCase()); ctx.fill();
    }
    // ring: what other nodes think of this one, even after it crashed
    if (crashed(id, t) && v[id].state !== "Alive") {
      ctx.strokeStyle = css("--" + v[id].state.toLowerCase()); ctx.lineWidth = 4;
      ctx.beginPath(); ctx.arc(p.x, p.y, radius + 5, 0, 2 * Math.PI); ctx.stroke();
    }
    ctx.fillStyle = css("--fg"); ctx.font = "600 13px system-ui"; ctx.textAlign = "center";
    ctx.fillText(id, p.x, p.y + radius + 20);
    const notes = [];
    if (v[id].Suspect) notes.push(`suspected by ${v[id].Suspect}`);
    if (v[id].Dead) notes.push(`dead for ${v[id].Dead}`);
    if (notes.length) {
      ctx.fillStyle = css("--muted"); ctx.font = "11px system-ui";
      ctx.fillText(notes.join(" · "), p.x, p.y + radius + 35);
    }
  }

  // event log up to t; rebuilt only when it changes, follows the end unless scrolled up
  let shown = 0;
  while (shown < DATA.log.length && DATA.log[shown].t <= t) shown++;
  if (shown !== logBox.childElementCount) {
    const atEnd = logBox.scrollTop + logBox.clientHeight >= logBox.scrollHeight - 4;
    while (logBox.childElementCount > shown) logBox.lastChild.remove();
    for (const e of DATA.log.slice(logBox.childElementCount, shown)) {
      const div = document.createElement("div");
      div.className = e.kind;
      const ts = document.createElement("span");
      ts.className = "t"; ts.textContent = e.t.toFixed(3);
      div.append(ts, document.createTextNode(e.text));
      logBox.append(div);
    }
    if (atEnd) logBox.scrollTop = logBox.scrollHeight;
  }

  clock.textContent = `${t.toFixed(3)} s`;
  slider.value = t;
}

function frame(now) {
  if (playing) {
    if (lastFrame !== null) t += ((now - lastFrame) / 1000) * Number(speedSelect.value);
    if (t >= DATA.until) { t = DATA.until; setPlaying(false); }
    lastFrame = now;
    draw();
    requestAnimationFrame(frame);
  }
}

function setPlaying(p) {
  playing = p; lastFrame = null;
  playButton.textContent = p ? "⏸ Pause" : "▶ Play";
  if (p) { if (t >= DATA.until) t = 0; requestAnimationFrame(frame); }
}

playButton.onclick = () => setPlaying(!playing);
document.getElementById("restart").onclick = () => { t = 0; draw(); };
slider.oninput = () => { t = Number(slider.value); draw(); };
document.addEventListener("keydown", e => {
  if (e.code === "Space") { e.preventDefault(); setPlaying(!playing); }
});
window.addEventListener("resize", draw);
draw();
</script>
</body>
</html>
"""

if __name__ == "__main__":
    sys.exit(main())
