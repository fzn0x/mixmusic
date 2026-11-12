// The small demos beside the main features (#357): each is drawn here, in a
// few lines of HTML, into the panel or column of its feature, and plays
// once as it scrolls into the window. It stops while the pointer is over it
// and goes on when the pointer leaves. The songs and the lyric lines are
// made up for this page. demos.css has the look.
//
// Under reduced motion a demo shows the state it ends in at once, with
// nothing playing, and without script the page simply has no demos: what
// they show is said in the text beside them.
//
// A demo is { where, html, steps }. `where` is a selector of the element
// the demo goes before, and `html` what it holds. `steps(el)` is a list of
// [wait, change] pairs: the change is made `wait` milliseconds after the one
// before it, and the last one leaves the state the demo ends in.
(() => {
  const mark = (service) => `<span class="mark mark-${service}" aria-hidden="true"></span>`;
  const icon = (id, size = 18) => `<svg class="icon" width="${size}" height="${size}" aria-hidden="true"><use href="#${id}" /></svg>`;
  const row = (title, artist, service, c1, c2) =>
    `<li class="d-row" style="--c1:${c1};--c2:${c2}"><b></b><span><strong>${title}</strong><small>${artist}</small></span>${mark(service)}</li>`;

  const demos = [];

  // Search: a few letters typed, the suggestions of both services, one
  // stepped to and played.
  demos.push({
    where: "#panel-search .note",
    html: () => `
      <div class="demo demo-search" data-demo="search">
        <div class="d-box">${icon("i-search")}<span class="d-typed" data-hint="Search Spotify and YouTube Music"></span></div>
        <ul class="d-list is-empty">
          ${row("Neon Tide", "Kaori Ume", "spotify", "#52e4ea", "#5181ec")}
          ${row("Neon Tide (live)", "Kaori Ume", "youtube", "#52e4ea", "#d96472")}
          ${row("Neon Tokyo", "Hoshino Club", "youtube", "#e99453", "#d96472")}
        </ul>
        <p class="d-status"></p>
      </div>`,
    steps: (el) => {
      const typed = el.querySelector(".d-typed");
      const list = el.querySelector(".d-list");
      const rows = [...list.children];
      const status = el.querySelector(".d-status");
      const select = (n) => rows.forEach((r, i) => r.classList.toggle("is-on", i === n));
      const text = [..."neon ti"].map((_, i, a) => a.slice(0, i + 1).join(""));
      return [
        [500, () => typed.classList.add("is-typing")],
        ...text.map((t) => [110, () => (typed.textContent = t)]),
        [260, () => list.classList.remove("is-empty")],
        [800, () => select(0)],
        [650, () => select(1)],
        [750, () => {
          typed.classList.remove("is-typing");
          status.classList.add("is-on");
          status.textContent = "Playing Neon Tide (live) from YouTube Music";
        }],
      ];
    },
  });

  // Queue: tracks of both services in one list, and one added to play next.
  demos.push({
    where: "#panel-queue .note",
    html: () => `
      <div class="demo demo-queue" data-demo="queue">
        <p class="d-status is-on" style="color: var(--ink-2)">Up next</p>
        <ul class="d-list" style="min-height: 0">
          ${row("Paper Planets", "Velvet Ferries", "spotify", "#ec8051", "#d96472")}
          ${row("Neon Tide", "Kaori Ume", "youtube", "#52e4ea", "#5181ec")}
          ${row("Slow Static", "Mira Vale", "spotify", "#7769d3", "#51a8ec")}
          ${row("Kitsune Run", "Hoshino Club", "youtube", "#d96472", "#e99453")}
        </ul>
      </div>`,
    steps: (el) => {
      const list = el.querySelector(".d-list");
      return [
        [900, () => {
          const li = document.createElement("template");
          li.innerHTML = row("Glass Garden", "Juniper Lane", "spotify", "#92d369", "#51a8ec");
          const added = li.content.firstElementChild;
          added.classList.add("is-new", "is-on");
          list.insertBefore(added, list.children[1]);
          // The row is in place at nothing tall, then grows.
          void added.offsetHeight;
          requestAnimationFrame(() => added.classList.remove("is-new"));
        }],
        [1500, () => list.querySelector(".is-on")?.classList.remove("is-on")],
      ];
    },
  });

  // Downloads: a song's bar fills, and says when it is done.
  demos.push({
    where: "#panel-library .note",
    html: () => `
      <div class="demo demo-download" data-demo="download">
        <ul class="d-list" style="min-height: 0">${row("Slow Static", "Mira Vale", "spotify", "#7769d3", "#51a8ec")}</ul>
        <div class="d-bar"><i></i></div>
        <p class="d-meta"><span data-state>Downloading</span><span data-percent>0%</span></p>
      </div>`,
    steps: (el) => {
      const fill = el.querySelector(".d-bar i");
      const state = el.querySelector("[data-state]");
      const percent = el.querySelector("[data-percent]");
      const to = (n) => () => {
        fill.style.width = `${n}%`;
        percent.textContent = `${n}%`;
      };
      return [
        [600, to(18)],
        [700, to(41)],
        [700, to(67)],
        [800, to(89)],
        [700, () => {
          to(100)();
          state.textContent = "Downloaded";
          state.classList.add("is-done");
        }],
      ];
    },
  });

  // Lyrics: the lines are made up. The one sung now is bright, and its
  // words light as they are sung.
  const lines = ["Paper boats on a copper sea", "carry every word to me", "light the lantern, let it show"];
  demos.push({
    where: "#panel-lyrics .note",
    html: () => `
      <div class="demo d-lyrics" data-demo="lyrics">
        ${lines.map((l) => `<p class="d-line">${l.split(" ").map((w) => `<i>${w}</i>`).join(" ")}</p>`).join("")}
      </div>`,
    steps: (el) => {
      const ps = [...el.querySelectorAll(".d-line")];
      const out = [];
      ps.forEach((p, n) => {
        out.push([n ? 260 : 600, () => {
          ps.forEach((q, k) => {
            q.classList.toggle("is-past", k < n);
            q.classList.toggle("is-now", k === n);
          });
        }]);
        p.querySelectorAll("i").forEach((w) => out.push([270, () => w.classList.add("is-sung")]));
      });
      return out;
    },
  });

  // Themes: one small player in the colours of four themes in turn. The
  // colours are those of the chips below the screenshot, the accent of Sakura
  // a little deeper so that its button's words read.
  const looks = [
    ["Mix", "#000000", "#ffffff", "#00bde3", "#000000"],
    ["Sakura", "#fff5f7", "#3a1b25", "#c42a66", "#ffffff"],
    ["Terminal", "#030603", "#9dff8f", "#39ff14", "#030603"],
    ["Walnut", "#24160b", "#fbeedf", "#fa8927", "#24160b"],
  ];
  demos.push({
    where: "#customize .columns .column .link",
    html: () => `
      <div class="demo d-theme" data-demo="theme">
        <div class="d-inner">
          <div class="d-art"></div>
          <div class="d-bar"><i></i></div>
          <span class="d-go">Play</span>
          <span class="d-name" data-name>${looks[0][0]}</span>
        </div>
      </div>`,
    steps: (el) => {
      const set = (l) => () => {
        el.style.setProperty("--t-bg", l[1]);
        el.style.setProperty("--t-ink", l[2]);
        el.style.setProperty("--t-accent", l[3]);
        el.style.setProperty("--t-on", l[4]);
        el.querySelector("[data-name]").textContent = `${l[0]} theme`;
      };
      return looks.slice(1).map((l, i) => [i ? 1300 : 900, set(l)]);
    },
  });

  // The mini player: play is pressed, and the bar and the time go on.
  demos.push({
    where: "#panel-keyboard .note",
    html: () => `
      <div class="demo d-mini" data-demo="mini">
        <span class="d-cover"></span>
        <span class="d-text"><strong>Slow Static</strong><small>Mira Vale</small></span>
        <span class="d-pp"><svg class="i-play" viewBox="0 0 24 24"><path d="M7 4.5v15l12.5-7.5z" fill="currentColor" /></svg><svg class="i-pause" viewBox="0 0 24 24"><path d="M7 4.5v15M17 4.5v15" /></svg></span>
        <div class="d-bar"><i style="width: 0"></i></div>
        <small class="d-time" style="grid-column: 1 / -1; display: flex; justify-content: space-between"><span data-now>0:00</span><span>3:12</span></small>
      </div>`,
    steps: (el) => {
      const pp = el.querySelector(".d-pp");
      const fill = el.querySelector(".d-bar i");
      const now = el.querySelector("[data-now]");
      const at = (s) => () => {
        fill.style.width = `${((s / 192) * 100).toFixed(1)}%`;
        now.textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
      };
      return [
        [700, () => pp.classList.add("is-pressed")],
        [160, () => {
          pp.classList.remove("is-pressed");
          el.classList.add("is-playing");
        }],
        [500, at(18)],
        [800, at(40)],
        [800, at(63)],
        [800, at(88)],
      ];
    },
  });

  // A timeline that can wait: start, hold while the pointer is over the
  // demo, and finish at once.
  const timeline = (steps, done) => {
    let i = 0;
    let timer = 0;
    let due = 0;
    let left = 0;
    let held = false;
    let started = false;
    const arm = () => {
      if (i >= steps.length) return;
      due = performance.now() + left;
      timer = setTimeout(() => {
        steps[i++][1]();
        if (i < steps.length) {
          left = steps[i][0];
          arm();
        } else done();
      }, left);
    };
    return {
      start() {
        if (started) return;
        started = true;
        left = steps[0][0];
        if (!held) arm();
      },
      hold() {
        held = true;
        if (!started || i >= steps.length) return;
        clearTimeout(timer);
        left = Math.max(0, due - performance.now());
      },
      go() {
        if (!held) return;
        held = false;
        if (started && i < steps.length) arm();
      },
      finish() {
        clearTimeout(timer);
        started = true;
        while (i < steps.length) steps[i++][1]();
        done();
      },
    };
  };

  for (const demo of demos) {
    const host = document.querySelector(demo.where);
    if (!host) continue;
    const template = document.createElement("template");
    template.innerHTML = demo.html().trim();
    const el = template.content.firstElementChild;
    // Decoration for what the text says; the marker is that of the checks
    // of reduced motion (motion.css, #360). Like the app it shows, it's in
    // English, left to right on every page.
    el.setAttribute("aria-hidden", "true");
    el.setAttribute("dir", "ltr");
    el.setAttribute("data-motion", "");
    host.before(el);
    // data-done is there once it has played to its end, for the checks too.
    const run = timeline(demo.steps(el), () => el.setAttribute("data-done", ""));
    // Hover pauses a demo; a finger on a touch screen doesn't.
    el.addEventListener("pointerenter", (e) => e.pointerType !== "touch" && run.hold());
    el.addEventListener("pointerleave", () => run.go());
    const reduced = window.MixReveal ? () => window.MixReveal.reduced : () => matchMedia("(prefers-reduced-motion: reduce)").matches;
    const enter = () => (reduced() ? run.finish() : run.start());
    if (window.MixReveal) window.MixReveal.watch(el, { animate: false, onEnter: enter });
    else enter();
  }
})();
