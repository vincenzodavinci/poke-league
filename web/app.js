// League Board front end. Plain DOM APIs, no framework, no innerHTML with
// server data: everything goes through el() so names and policy messages
// can never inject markup.
"use strict";

const TOWNS = {
  "pallet-town": { title: "Pallet Town", mode: "audit", blurb: "dev · gyms report, nothing is blocked" },
  "indigo-plateau": { title: "Indigo Plateau", mode: "enforce", blurb: "prod · a failed gym blocks admission" },
  "team-rocket": { title: "Team Rocket Hideout", mode: "audit", blurb: "lab · deliberately insecure workloads" },
};

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") node.className = v;
    else if (k === "style") Object.assign(node.style, v);
    else if (k.startsWith("on")) node.addEventListener(k.slice(2), v);
    else node.setAttribute(k, v);
  }
  for (const c of children.flat()) {
    if (c == null || c === false) continue;
    node.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return node;
}

const art = (id) => `/sprites/pokemon/${id}.png`;
const badgeImg = (n) => `/sprites/badges/${n}.png`;
const typePill = (t) => el("span", { class: "type", style: { background: `var(--t-${t})` } }, t);
// "kind-registry:5000/poke-league/dex-api@sha256:4d58ad4e…" -> "dex-api@sha256:4d58ad4e"
const shortImage = (img) => img.replace(/^.*\//, "").replace(/@sha256:(\w{8})\w+/, "@sha256:$1");
const title = (s) => s.replace(/-/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

let dex = [];
let gyms = [];
let league = null;

// ---------- tabs ----------
for (const btn of document.querySelectorAll("nav button")) {
  btn.addEventListener("click", () => {
    for (const b of document.querySelectorAll("nav button")) b.setAttribute("aria-selected", b === btn);
    for (const s of document.querySelectorAll(".tab")) s.hidden = s.id !== btn.dataset.tab;
    location.hash = btn.dataset.tab;
  });
}
const start = document.querySelector(`nav button[data-tab="${location.hash.slice(1)}"]`);
if (start) start.click();

// ---------- league ----------
function hpColor(hp) {
  if (hp > 50) return "var(--ok)";
  if (hp > 20) return "var(--warn)";
  return "var(--bad)";
}

function workloadCard(w) {
  const name = dex.find((p) => p.id === w.pokemon)?.name ?? `#${w.pokemon}`;
  const fainted = w.hp === 0;
  const hp = w.hp == null
    ? el("div", { class: "hp" }, el("span", { class: "label" }, "HP"), el("div", { class: "bar" }), el("span", {}, "??"))
    : el("div", { class: "hp" },
        el("span", { class: "label" }, "HP"),
        el("div", { class: "bar" }, el("div", { class: "fill", style: { width: `${w.hp}%`, background: hpColor(w.hp) } })),
        el("span", {}, `${w.hp}/100`));
  const vulns = w.vulns
    ? el("div", { class: "vulns" }, `${w.vulns.critical} critical · ${w.vulns.high} high · ${w.vulns.medium} medium CVEs`)
    : el("div", { class: "vulns" }, "awaiting Trivy scan");

  const badges = el("div", { class: "badges" }, w.badges.map((b, i) => {
    const g = gyms[i];
    return el("div", { class: `badge-wrap ${b.status}`, title: `${g.badge} Badge (${g.leader}): ${b.status}` },
      el("img", { src: badgeImg(g.badgeId), alt: `${g.badge} Badge ${b.status}`, class: b.status }));
  }));

  const failures = w.badges.filter((b) => b.status === "failed");
  const why = failures.length
    ? el("ul", { class: "why" }, failures.map((b) => {
        const g = gyms.find((x) => x.key === b.gym);
        // Policy messages already start with "<Leader>: "; don't say it twice.
        const msg = (b.message || g.lesson).replace(new RegExp(`^${g.leader}:\\s*`), "");
        return el("li", {}, el("b", {}, `${g.leader}: `), msg);
      }))
    : null;

  return el("article", { class: `card ${w.shiny ? "shiny" : ""} ${fainted ? "fainted" : ""}` },
    el("img", { class: "art", src: art(w.pokemon), alt: name, loading: "lazy" }),
    el("h3", {}, w.name),
    el("div", { class: "img", title: w.image }, `${title(name)} · ${shortImage(w.image)}`),
    hp, vulns, badges,
    el("div", { class: "earned-count" }, `${w.earned}/8 badges${fainted ? " · fainted" : ""}`),
    why);
}

function renderLeague() {
  const root = document.getElementById("towns");
  if (!league) return;
  const byNs = {};
  for (const w of league.workloads) (byNs[w.namespace] ??= []).push(w);
  const towns = Object.keys(TOWNS).filter((ns) => byNs[ns]?.length);
  if (!towns.length) {
    root.replaceChildren(el("p", { class: "empty" }, "No trainers on the route yet. Run ", el("code", {}, "make deploy"), "."));
    return;
  }
  root.replaceChildren(...towns.map((ns) => {
    const t = TOWNS[ns];
    return el("section", { class: "town" },
      el("div", { class: "town-head" },
        el("h2", {}, t.title), el("span", { class: `mode ${t.mode}` }, t.mode.toUpperCase()),
        el("span", { class: "ns" }, `ns/${ns} · ${t.blurb}`)),
      el("div", { class: "grid cards" }, byNs[ns].map(workloadCard)));
  }));
}

function renderGyms() {
  const grid = document.getElementById("gym-grid");
  grid.replaceChildren(...gyms.map((g, i) => {
    const judged = league ? league.workloads.map((w) => w.badges[i]).filter((b) => b.status !== "unknown") : [];
    const earned = judged.filter((b) => b.status === "earned").length;
    return el("article", { class: "card gym" },
      el("img", { class: "badge-big", src: badgeImg(g.badgeId), alt: `${g.badge} Badge` }),
      el("div", {},
        el("div", { class: "num" }, `Gym ${i + 1} `, typePill(g.type)),
        el("h3", {}, g.leader),
        el("div", { class: "num" }, `${g.badge} Badge`)),
      el("img", { class: "ace", src: art(g.pokemon), alt: `${g.leader}'s ace`, loading: "lazy" }),
      el("code", { class: "policy" }, g.policy),
      el("p", {}, g.lesson),
      el("div", { class: "score" }, judged.length ? `${earned}/${judged.length} workloads earned this badge` : "no workloads judged yet"));
  }));
}

async function refreshLeague() {
  try {
    const res = await fetch("/api/league");
    if (!res.ok) throw new Error(await res.text());
    league = await res.json();
    gyms = league.gyms;
    renderLeague();
    renderGyms();
  } catch (err) {
    document.getElementById("towns").replaceChildren(el("p", { class: "empty" }, `League unreachable: ${err.message}`));
  }
}

// ---------- pokédex ----------
let activeType = "";

function renderDex() {
  const q = document.getElementById("q").value.trim().toLowerCase();
  const list = dex.filter((p) => (!q || p.name.includes(q)) && (!activeType || p.types.includes(activeType)));
  document.getElementById("dex-grid").replaceChildren(...list.map((p) =>
    el("article", { class: "card mon", onclick: () => showDetail(p), tabindex: "0", onkeydown: (e) => e.key === "Enter" && showDetail(p) },
      el("div", { class: "no" }, `#${String(p.id).padStart(3, "0")}`),
      el("img", { src: art(p.id), alt: p.name, loading: "lazy" }),
      el("h3", {}, title(p.name)),
      el("div", { class: "types" }, p.types.map(typePill)))));
}

function showDetail(p) {
  const labels = { hp: "HP", attack: "Attack", defense: "Defense", "special-attack": "Sp. Atk", "special-defense": "Sp. Def", speed: "Speed" };
  document.getElementById("detail-body").replaceChildren(
    el("div", { class: "detail" },
      el("img", { src: art(p.id), alt: p.name }),
      el("div", {},
        el("div", { class: "no" }, `#${String(p.id).padStart(3, "0")}`),
        el("h2", {}, title(p.name)),
        el("div", { class: "types" }, p.types.map(typePill)),
        el("p", { class: "no" }, `${(p.height / 10).toFixed(1)} m · ${(p.weight / 10).toFixed(1)} kg`)),
      el("p", { class: "flavor" }, p.flavor),
      el("div", { class: "stats" }, Object.entries(labels).flatMap(([k, label]) => [
        el("span", {}, label), el("b", {}, p.stats[k]),
        el("div", { class: "bar" }, el("i", { style: { width: `${Math.min(100, p.stats[k] / 2)}%` } }))]))));
  document.getElementById("detail").showModal();
}

document.getElementById("close").addEventListener("click", () => document.getElementById("detail").close());
document.getElementById("q").addEventListener("input", renderDex);

async function loadDex() {
  const [pokemon, types] = await Promise.all([
    fetch("/api/pokemon").then((r) => r.json()),
    fetch("/api/types").then((r) => r.json()),
  ]);
  dex = pokemon;
  document.getElementById("types").replaceChildren(...types.map((t) => {
    const chip = typePill(t);
    chip.addEventListener("click", () => {
      activeType = activeType === t ? "" : t;
      for (const c of document.querySelectorAll("#types .type")) c.classList.toggle("on", c.textContent === activeType);
      renderDex();
    });
    return chip;
  }));
  renderDex();
}

loadDex().catch(() => {}).finally(() => {
  refreshLeague();
  setInterval(refreshLeague, 10000);
});
