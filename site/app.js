// The MixMusic site: feature tabs, the walkthrough's chapters, the theme
// gallery, and download buttons named for the visitor's system. Every
// download goes to the latest release on GitHub.
(() => {
  "use strict";

  const REPO = "fzn0x/mixmusic";
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");

  // The app bar rises once the page scrolls under it.
  const appbar = $("[data-appbar]");
  const raise = () => appbar.classList.toggle("is-raised", scrollY > 8);
  addEventListener("scroll", raise, { passive: true });
  raise();

  // Two images stacked in one frame: the next one fades in over the shown
  // one once it has loaded, so the frame never flashes empty.
  function crossfader(first) {
    const second = first.cloneNode();
    second.classList.remove("is-shown");
    second.removeAttribute("fetchpriority");
    second.alt = "";
    second.setAttribute("aria-hidden", "true");
    let shown = first;
    let mounted = false;
    return (src, alt) => {
      if (!mounted) {
        first.after(second);
        mounted = true;
      }
      const next = shown === first ? second : first;
      next.src = src;
      const reveal = () => {
        next.alt = alt;
        next.removeAttribute("aria-hidden");
        next.classList.add("is-shown");
        shown.classList.remove("is-shown");
        shown.setAttribute("aria-hidden", "true");
        shown.alt = "";
        shown = next;
      };
      next.complete && next.naturalWidth ? reveal() : next.addEventListener("load", reveal, { once: true });
    };
  }

  // Feature tabs, with the keys of the tabs pattern: the arrows move
  // between tabs, Home and End go to the ends.
  const tabs = $("[data-tabs]");
  if (tabs) {
    const list = $$("[role=tab]", tabs);
    const show = crossfader($("[data-stage-img]", tabs));
    const select = (tab, focus) => {
      list.forEach((t) => {
        const on = t === tab;
        t.setAttribute("aria-selected", String(on));
        t.tabIndex = on ? 0 : -1;
        $(`#${t.getAttribute("aria-controls")}`).hidden = !on;
      });
      const panel = $(`#${tab.getAttribute("aria-controls")}`);
      tabs.style.setProperty("--glow", tab.dataset.accent);
      show(`assets/shots/${panel.dataset.shot}.webp`, panel.dataset.alt);
      if (focus) tab.focus();
    };
    list.forEach((tab, i) => {
      tab.addEventListener("click", () => select(tab));
      tab.addEventListener("keydown", (e) => {
        const to = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: list.length - 1 }[e.key];
        if (to === undefined) return;
        e.preventDefault();
        select(list[(to + list.length) % list.length], true);
      });
      // Warm the next screenshot on the way to it.
      const pre = new Image();
      tab.addEventListener("pointerenter", () => {
        pre.src = `assets/shots/${$(`#${tab.getAttribute("aria-controls")}`).dataset.shot}.webp`;
      }, { once: true });
    });
  }

  // The walkthrough: each chapter seeks the video to its page, and the
  // chapter of the page playing lights up.
  const video = $("[data-walkthrough]");
  const chapterRow = $("[data-chapters]");
  if (video && chapterRow) {
    const chapters = $$("[data-at]", chapterRow);
    const starts = chapters.map((c) => Number(c.dataset.at));
    let chapter = 0;
    const mark = () => {
      let index = 0;
      starts.forEach((t, i) => {
        if (video.currentTime >= t) index = i;
      });
      if (index === chapter) return;
      chapter = index;
      chapters.forEach((c, i) => {
        c.classList.toggle("is-current", i === index);
        if (i === index) c.setAttribute("aria-current", "true");
        else c.removeAttribute("aria-current");
      });
    };
    chapters.forEach((c, i) =>
      c.addEventListener("click", () => {
        video.currentTime = starts[i];
        video.play().catch(() => {});
      }),
    );
    video.addEventListener("timeupdate", mark);
    video.addEventListener("seeked", mark);

    // The progress bar under it follows the video, and seeks it: a click
    // or a drag anywhere on the bar, or the arrows, Home and End.
    const scrub = $("[data-scrub]");
    const fill = $("[data-scrub-fill]");
    const clock = (s) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
    const time = $("[data-time]");
    const draw = () => {
      const total = video.duration || 60;
      fill.style.width = `${(100 * video.currentTime) / total}%`;
      time.textContent = `${clock(video.currentTime)} / ${clock(total)}`;
      scrub.setAttribute("aria-valuemax", String(Math.round(total)));
      scrub.setAttribute("aria-valuenow", String(Math.round(video.currentTime)));
      scrub.setAttribute("aria-valuetext", `${clock(video.currentTime)} of ${clock(total)}`);
    };
    const seekTo = (x) => {
      const r = scrub.getBoundingClientRect();
      video.currentTime = Math.min(Math.max((x - r.left) / r.width, 0), 1) * (video.duration || 60);
      draw();
    };
    video.addEventListener("timeupdate", draw);
    video.addEventListener("loadedmetadata", draw);

    // The round button and a click on the video play and pause it.
    const play = $("[data-play]");
    const toggle = () => (video.paused ? video.play().catch(() => {}) : video.pause());
    const state = () => {
      play.classList.toggle("is-playing", !video.paused);
      play.setAttribute("aria-label", video.paused ? "Play the walkthrough" : "Pause the walkthrough");
    };
    play.addEventListener("click", toggle);
    video.addEventListener("click", toggle);
    video.addEventListener("play", state);
    video.addEventListener("pause", state);
    scrub.addEventListener("pointerdown", (e) => {
      scrub.setPointerCapture(e.pointerId);
      seekTo(e.clientX);
      const move = (ev) => seekTo(ev.clientX);
      scrub.addEventListener("pointermove", move);
      scrub.addEventListener("pointerup", () => scrub.removeEventListener("pointermove", move), { once: true });
    });
    scrub.addEventListener("keydown", (e) => {
      const total = video.duration || 60;
      const to = { ArrowRight: video.currentTime + 5, ArrowUp: video.currentTime + 5, ArrowLeft: video.currentTime - 5, ArrowDown: video.currentTime - 5, Home: 0, End: total }[e.key];
      if (to === undefined) return;
      e.preventDefault();
      video.currentTime = Math.min(Math.max(to, 0), total);
      draw();
    });
  }

  // The theme gallery: each chip shows the home page in its theme, on a
  // glow of the theme's accent.
  const gallery = $("[data-gallery]");
  if (gallery) {
    const show = crossfader($(".gallery-img", gallery));
    const name = $("[data-gallery-name]", gallery);
    const desc = $("[data-gallery-desc]", gallery);
    const chips = $$(".chip-theme", gallery);
    chips.forEach((chip) => {
      const pre = new Image();
      chip.addEventListener("pointerenter", () => (pre.src = `assets/shots/theme-${chip.dataset.theme}.webp`), { once: true });
      chip.addEventListener("click", () => {
        if (chip.classList.contains("is-current")) return;
        chips.forEach((c) => {
          c.classList.toggle("is-current", c === chip);
          c.setAttribute("aria-pressed", String(c === chip));
        });
        const accent = $(".dots i", chip)?.style.background;
        if (accent) gallery.style.setProperty("--glow", accent);
        show(`assets/shots/theme-${chip.dataset.theme}.webp`, `The home page in the ${chip.dataset.name} theme.`);
        name.textContent = chip.dataset.meta === "community" ? `${chip.dataset.name}, from the community` : chip.dataset.name;
        desc.textContent = chip.dataset.desc;
      });
    });
  }

  // The demo player: a small copy of MixMusic's player with made-up
  // tracks of both services. It plays its queue silently, in real time.
  const demo = $("[data-demo]");
  if (demo) {
    const CATALOG = [
      { title: "Paper Planets", artist: "Velvet Ferries", src: "spotify", dur: 187, cover: 2, art: "#ec8051" },
      { title: "Neon Tide", artist: "Kaori Ume", src: "youtube", dur: 214, cover: 1, art: "#52e4ea" },
      { title: "Slow Static", artist: "Mira Vale", src: "spotify", dur: 243, cover: 3, art: "#7769d3" },
      { title: "Kitsune Run", artist: "Hoshino Club", src: "youtube", dur: 198, cover: 4, art: "#d96472" },
      { title: "Glass Garden", artist: "Juniper Lane", src: "spotify", dur: 226, cover: 5, art: "#92d369" },
      { title: "Afterglow Avenue", artist: "Oriel Bloom", src: "youtube", dur: 261, cover: 6, art: "#e99453" },
      { title: "Little Satellites", artist: "Coral & the Comets", src: "spotify", dur: 175, cover: 7, art: "#5181ec" },
      { title: "Summer Signal", artist: "Aoi Kanata", src: "youtube", dur: 232, cover: 8, art: "#51a8ec" },
      // Only in search: the same artists on the other service.
      { title: "Tidewater", artist: "Kaori Ume", src: "spotify", dur: 201, cover: 9, art: "#8cc0bd" },
      { title: "Paper Moon Radio", artist: "Velvet Ferries", src: "youtube", dur: 219, cover: 10, art: "#cf6e74" },
      { title: "Static Bloom", artist: "Mira Vale", src: "youtube", dur: 236, cover: 11, art: "#d864a4" },
      { title: "Fox Lantern", artist: "Hoshino Club", src: "spotify", dur: 207, cover: 12, art: "#d5b168" },
    ];
    const SERVICE = { spotify: "Spotify", youtube: "YouTube Music" };
    const wrap = demo.closest("[data-demo-wrap]");
    const ui = {
      cover: $("[data-demo-cover]", demo),
      title: $("[data-demo-title]", demo),
      artist: $("[data-demo-artist]", demo),
      mark: $("[data-demo-mark]", demo),
      elapsed: $("[data-demo-elapsed]", demo),
      total: $("[data-demo-total]", demo),
      bar: $("[data-demo-bar]", demo),
      fill: $("[data-demo-fill]", demo),
      play: $("[data-demo-play]", demo),
      shuffle: $("[data-demo-shuffle]", demo),
      repeat: $("[data-demo-repeat]", demo),
      list: $("[data-demo-list]", demo),
      heading: $("[data-demo-heading]", demo),
      filter: $("[data-demo-filter]", demo),
      search: $("[data-demo-search]", demo),
      status: $("[data-demo-status]", demo),
    };
    const clock = (s) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
    const st = { queue: [0, 1, 2, 3, 4, 5, 6, 7], ordered: null, pos: 0, elapsed: 0, playing: false, repeat: false, query: "", source: "both" };
    const track = () => CATALOG[st.queue[st.pos]];

    const mark = (src) => {
      const m = document.createElement("span");
      m.className = "mark";
      m.dataset.src = src;
      m.setAttribute("role", "img");
      m.setAttribute("aria-label", SERVICE[src]);
      return m;
    };
    const say = (text) => (ui.status.textContent = text);

    function drawTime() {
      const t = track();
      const ratio = Math.min(st.elapsed / t.dur, 1);
      ui.fill.style.width = `${ratio * 100}%`;
      ui.elapsed.textContent = clock(st.elapsed);
      ui.bar.setAttribute("aria-valuenow", String(Math.round(ratio * 100)));
      ui.bar.setAttribute("aria-valuetext", `${clock(st.elapsed)} of ${clock(t.dur)}`);
    }

    function drawNow() {
      const t = track();
      ui.cover.src = `assets/covers/cover-${t.cover}.webp`;
      ui.cover.alt = `The cover of ${t.title}, made up for this demo`;
      ui.title.textContent = t.title;
      ui.artist.textContent = t.artist;
      ui.mark.dataset.src = t.src;
      ui.mark.setAttribute("role", "img");
      ui.mark.setAttribute("aria-label", SERVICE[t.src]);
      ui.total.textContent = clock(t.dur);
      wrap.style.setProperty("--art", t.art);
      drawTime();
    }

    function drawPlay() {
      demo.classList.toggle("is-playing", st.playing);
      ui.play.classList.toggle("is-playing", st.playing);
      ui.play.setAttribute("aria-label", st.playing ? "Pause" : "Play");
    }

    function row(i, current, addable) {
      const t = CATALOG[i];
      const li = document.createElement("li");
      if (current) li.classList.add("is-current");
      const b = document.createElement("button");
      b.type = "button";
      b.className = "demo-row";
      const img = document.createElement("img");
      img.src = `assets/covers/cover-${t.cover}.webp`;
      img.alt = "";
      img.width = img.height = 44;
      const text = document.createElement("span");
      text.className = "demo-row-text";
      const strong = document.createElement("strong");
      strong.textContent = t.title;
      const sub = document.createElement("span");
      sub.append(mark(t.src), document.createTextNode(t.artist));
      text.append(strong, sub);
      const time = document.createElement("span");
      time.className = "demo-row-time";
      if (current) {
        time.innerHTML = '<span class="eq" aria-hidden="true"><i></i><i></i><i></i></span>';
        b.setAttribute("aria-current", "true");
      } else {
        time.textContent = clock(t.dur);
      }
      b.append(img, text, time);
      b.setAttribute("aria-label", `${current ? "Playing: " : "Play "}${t.title} by ${t.artist}, on ${SERVICE[t.src]}`);
      li.append(b);
      if (addable) {
        const add = document.createElement("button");
        add.type = "button";
        add.className = "demo-icon demo-add";
        add.setAttribute("aria-label", `Play ${t.title} next`);
        add.innerHTML = '<svg class="icon" width="20" height="20" aria-hidden="true"><use href="#i-plus" /></svg>';
        add.addEventListener("click", () => playNext(i));
        li.append(add);
      }
      return { li, b };
    }

    function drawList() {
      ui.list.replaceChildren();
      const q = st.query.trim().toLowerCase();
      ui.filter.hidden = !q;
      if (!q) {
        ui.heading.textContent = "Up next";
        st.queue.forEach((i, at) => {
          if (at < st.pos) return;
          const { li, b } = row(i, at === st.pos, false);
          b.addEventListener("click", () => jump(at));
          ui.list.append(li);
        });
        return;
      }
      ui.heading.textContent = `Results for “${st.query.trim()}”`;
      const found = CATALOG.map((t, i) => [t, i]).filter(([t]) =>
        (st.source === "both" || t.src === st.source) &&
        (t.title.toLowerCase().includes(q) || t.artist.toLowerCase().includes(q)));
      if (!found.length) {
        const li = document.createElement("li");
        li.className = "demo-empty";
        li.textContent = `Nothing in this demo matches “${st.query.trim()}”. Try “paper”, “mira” or “fox”.`;
        ui.list.append(li);
        return;
      }
      for (const [, i] of found) {
        const { li, b } = row(i, i === st.queue[st.pos], true);
        b.addEventListener("click", () => playNow(i));
        ui.list.append(li);
      }
    }

    function jump(at) {
      st.pos = at;
      st.elapsed = 0;
      st.playing = true;
      drawNow();
      drawPlay();
      drawList();
    }
    function playNext(i) {
      st.queue.splice(st.pos + 1, 0, i);
      if (st.ordered) st.ordered.push(i);
      say(`${CATALOG[i].title} plays next, from ${SERVICE[CATALOG[i].src]}.`);
      drawList();
    }
    function playNow(i) {
      st.queue.splice(st.pos + 1, 0, i);
      jump(st.pos + 1);
      say(`Playing ${CATALOG[i].title}, from ${SERVICE[CATALOG[i].src]}.`);
    }
    function next(auto) {
      if (st.pos < st.queue.length - 1) return jump(st.pos + 1);
      if (st.repeat) return jump(0);
      if (auto) {
        st.playing = false;
        st.elapsed = 0;
        drawTime();
        drawPlay();
        say("The end of the queue.");
      }
    }
    function prev() {
      if (st.elapsed > 3 || st.pos === 0) {
        st.elapsed = 0;
        drawTime();
        return;
      }
      jump(st.pos - 1);
    }

    ui.play.addEventListener("click", () => {
      st.playing = !st.playing;
      drawPlay();
    });
    $("[data-demo-next]", demo).addEventListener("click", () => next(false));
    $("[data-demo-prev]", demo).addEventListener("click", prev);
    ui.repeat.addEventListener("click", () => {
      st.repeat = !st.repeat;
      ui.repeat.setAttribute("aria-pressed", String(st.repeat));
    });
    ui.shuffle.addEventListener("click", () => {
      const head = st.queue.slice(0, st.pos + 1);
      if (!st.ordered) {
        // Shuffle what is still to come; turning it off puts it back.
        st.ordered = st.queue.slice(st.pos + 1);
        const rest = st.ordered.slice();
        for (let k = rest.length - 1; k > 0; k--) {
          const j = Math.floor(Math.random() * (k + 1));
          [rest[k], rest[j]] = [rest[j], rest[k]];
        }
        st.queue = head.concat(rest);
      } else {
        st.queue = head.concat(st.ordered.filter((i) => !head.includes(i)));
        st.ordered = null;
      }
      ui.shuffle.setAttribute("aria-pressed", String(!!st.ordered));
      drawList();
    });

    const seek = (x) => {
      const r = ui.bar.getBoundingClientRect();
      st.elapsed = Math.min(Math.max((x - r.left) / r.width, 0), 1) * track().dur;
      drawTime();
    };
    ui.bar.addEventListener("pointerdown", (e) => {
      ui.bar.setPointerCapture(e.pointerId);
      seek(e.clientX);
      const move = (ev) => seek(ev.clientX);
      ui.bar.addEventListener("pointermove", move);
      ui.bar.addEventListener("pointerup", () => ui.bar.removeEventListener("pointermove", move), { once: true });
    });
    ui.bar.addEventListener("keydown", (e) => {
      const step = { ArrowRight: 5, ArrowUp: 5, ArrowLeft: -5, ArrowDown: -5 }[e.key];
      if (step === undefined) return;
      e.preventDefault();
      st.elapsed = Math.min(Math.max(st.elapsed + step, 0), track().dur);
      drawTime();
    });

    const narrow = matchMedia("(max-width: 520px)");
    const placeholder = () => (ui.search.placeholder = narrow.matches ? "Search both services" : "Search Spotify and YouTube Music");
    narrow.addEventListener("change", placeholder);
    placeholder();

    ui.search.addEventListener("input", () => {
      st.query = ui.search.value;
      drawList();
    });
    $$("button", ui.filter).forEach((b) =>
      b.addEventListener("click", () => {
        st.source = b.dataset.src;
        $$("button", ui.filter).forEach((o) => {
          o.classList.toggle("is-on", o === b);
          o.setAttribute("aria-pressed", String(o === b));
        });
        drawList();
      }),
    );

    let last = performance.now();
    setInterval(() => {
      const now = performance.now();
      const dt = (now - last) / 1000;
      last = now;
      if (!st.playing) return;
      st.elapsed += dt;
      if (st.elapsed >= track().dur) next(true);
      else drawTime();
    }, 250);

    drawNow();
    drawPlay();
    drawList();

    // The player lies down flat as it scrolls up to the middle.
    if (!reduced.matches) {
      let ticking = false;
      const tilt = () => {
        ticking = false;
        const top = demo.getBoundingClientRect().top;
        const t = Math.min(Math.max((top - innerHeight * 0.12) / (innerHeight * 0.6), 0), 1);
        demo.style.setProperty("--tilt", `${(t * 12).toFixed(2)}deg`);
        demo.style.setProperty("--tilt-scale", (1 - t * 0.04).toFixed(4));
      };
      addEventListener("scroll", () => {
        if (!ticking) {
          ticking = true;
          requestAnimationFrame(tilt);
        }
      }, { passive: true });
      addEventListener("resize", tilt);
      tilt();
    }
  }

  // Two services, one queue: the columns of each service slide together
  // into one queue as the section scrolls by.
  const merge = $("[data-merge]");
  if (merge && !reduced.matches) {
    merge.classList.add("is-live");
    const stage = $("[data-merge-stage]", merge);
    const rows = $$(".merge-row", merge);
    const labels = {
      spotify: $(".merge-label-spotify", merge),
      youtube: $(".merge-label-youtube", merge),
      queue: $(".merge-label-queue", merge),
    };
    const ease = (x) => 1 - Math.pow(1 - x, 3);
    const clamp = (x) => Math.min(Math.max(x, 0), 1);
    let ticking = false;
    const place = () => {
      ticking = false;
      const w = stage.clientWidth;
      const h = stage.clientHeight;
      const top = 40;
      const step = Math.min(64, (h - top) / rows.length);
      const rowH = step - 8;
      const endW = Math.min(440, w);
      const xEnd = 0;
      const gap = Math.min(96, w * 0.08);
      // Phones have no room for two columns: the services start as two
      // stacks, one kept left and one kept right, then interleave.
      const stacked = (w - gap) / 2 < 220;
      const colW = stacked ? endW * 0.84 : Math.min(380, (w - gap) / 2);
      const x0 = 0;
      const x1 = stacked ? endW - colW : colW + gap;
      const startTop = top;
      const r = merge.getBoundingClientRect();
      const span = merge.offsetHeight - (innerHeight - 64);
      const q = clamp((-r.top / Math.max(span, 1) - 0.12) / 0.7);
      const seen = { spotify: 0, youtube: 0 };
      rows.forEach((row, k) => {
        const src = row.dataset.src;
        const i = seen[src]++;
        const sx = src === "spotify" ? x0 : x1;
        const sy = startTop + (stacked && src === "youtube" ? rows.length / 2 + i : i) * step;
        const ey = top + k * step;
        const e = ease(clamp(q * 1.35 - k * 0.05));
        const x = sx + (xEnd - sx) * e;
        const y = sy + (ey - sy) * e;
        row.style.width = `${colW + (endW - colW) * e}px`;
        row.style.height = `${rowH}px`;
        row.style.transform = `translate(${x}px, ${y}px)`;
        row.classList.toggle("is-merged", e > 0.98);
      });
      labels.spotify.style.transform = `translate(${x0}px, ${startTop - 34}px)`;
      const yw = labels.youtube.offsetWidth;
      labels.youtube.style.transform = stacked
        ? `translate(${endW - yw}px, ${startTop - 34}px)`
        : `translate(${x1}px, ${startTop - 34}px)`;
      labels.queue.style.transform = `translate(${xEnd}px, 4px)`;
      labels.spotify.style.opacity = labels.youtube.style.opacity = String(clamp(1 - q * 2.5));
      labels.queue.style.opacity = String(clamp(q * 2.5 - 1.5));
    };
    const ask = () => {
      if (!ticking) {
        ticking = true;
        requestAnimationFrame(place);
      }
    };
    addEventListener("scroll", ask, { passive: true });
    addEventListener("resize", ask);
    place();
  }

  // Sections ease in as they scroll into the window (styles.css).
  $$(".section .section-head").forEach((el) =>
    el.classList.add("reveal"),
  );

  // The visitor's system names the download buttons and marks its card.
  // The buttons all open the latest release, where every file is.
  const ua = navigator.userAgent;
  const platform = navigator.userAgentData?.platform || navigator.platform || ua;
  const os = /Mac/.test(platform) && !/iPhone|iPad/.test(ua)
    ? "mac"
    : /Win/.test(platform)
      ? "windows"
      : /Linux|X11/.test(platform) && !/Android/.test(ua)
        ? "linux"
        : "other";
  const names = { mac: "macOS", windows: "Windows", linux: "Linux" };
  if (names[os]) {
    $$("[data-download-label]").forEach((label) => (label.textContent = `Download for ${names[os]}`));
    $(`.system[data-os="${os}"]`)?.classList.add("is-yours");
  }
  if (os === "mac") $$("kbd[data-mod]").forEach((k) => (k.textContent = "⌘"));

  // The latest release's version, when it is newer than the page's.
  fetch(`https://api.github.com/repos/${REPO}/releases/latest`, { headers: { Accept: "application/vnd.github+json" } })
    .then((res) => (res.ok ? res.json() : null))
    .then((rel) => {
      const version = String(rel?.tag_name || "").replace(/^v/, "");
      if (version) $$("[data-version]").forEach((v) => (v.textContent = version));
    })
    .catch(() => {});
})();
