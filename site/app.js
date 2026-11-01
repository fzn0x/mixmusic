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

  // The player in the hero: a working copy of MixMusic's window in its
  // default theme, playing Creative Commons songs from this site, with its
  // mini player for narrow screens and for the mini player button.
  const wrap = $("[data-demo-wrap]");
  if (wrap) {
    const SONGS = [
      { title: "Local Forecast - Elevator", file: "local-forecast-elevator", dur: 189, cover: 10, art: "#cf6e74" },
      { title: "Carefree", file: "carefree", dur: 205, cover: 14, art: "#51b4ec" },
      { title: "Wallpaper", file: "wallpaper", dur: 220, cover: 11, art: "#d864a4" },
      { title: "Inspired", file: "inspired", dur: 286, cover: 13, art: "#df6d5d" },
      { title: "Fluffing a Duck", file: "fluffing-a-duck", dur: 67, cover: 9, art: "#8cc0bd" },
      { title: "Monkeys Spinning Monkeys", file: "monkeys-spinning-monkeys", dur: 125, cover: 15, art: "#db9361" },
    ];
    const ARTIST = "Kevin MacLeod";
    const audio = $("[data-audio]", wrap);
    const all = (sel) => $$(sel, wrap);
    const clock = (s) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, "0")}`;
    const st = { order: SONGS.map((_, i) => i), ordered: null, pos: 0, repeat: "off", liked: new Set(), panel: "queue", query: "" };
    const song = () => SONGS[st.order[st.pos]];
    const duration = () => (Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : song().dur);
    let loaded = -1;

    function drawTime() {
      const total = duration();
      const ratio = Math.min(audio.currentTime / total, 1) || 0;
      all("[data-fill]").forEach((f) => (f.style.width = `${ratio * 100}%`));
      all("[data-fill-line]").forEach((f) => (f.style.width = `${ratio * 100}%`));
      all("[data-elapsed]").forEach((e) => (e.textContent = clock(audio.currentTime)));
      all("[data-total]").forEach((e) => (e.textContent = clock(total)));
      all("[data-seek]").forEach((s) => {
        s.setAttribute("aria-valuenow", String(Math.round(ratio * 100)));
        s.setAttribute("aria-valuetext", `${clock(audio.currentTime)} of ${clock(total)}`);
      });
    }

    function drawSong() {
      const t = song();
      const cover = `assets/covers/cover-${t.cover}.webp`;
      all("[data-cover]").forEach((img) => (img.src = cover));
      all("[data-cover-alt]").forEach((img) => (img.alt = `A cover made for ${t.title} on this page`));
      all("[data-title]").forEach((e) => (e.textContent = t.title));
      all("[data-title-lower]").forEach((e) => (e.textContent = t.title.toLowerCase()));
      all("[data-artist]").forEach((e) => (e.textContent = ARTIST));
      const next = st.pos < st.order.length - 1 ? SONGS[st.order[st.pos + 1]] : st.repeat === "all" ? SONGS[st.order[0]] : null;
      all("[data-next-up]").forEach((e) => (e.textContent = next ? `next · ${next.title} — ${ARTIST}` : ""));
      all("[data-like]").forEach((b) => {
        const on = st.liked.has(t.file);
        b.setAttribute("aria-pressed", String(on));
        b.setAttribute("aria-label", on ? "Remove from liked songs" : "Like");
      });
      wrap.style.setProperty("--art", t.art);
      drawTime();
    }

    function drawPlay() {
      const playing = !audio.paused;
      all("[data-play]").forEach((b) => {
        b.classList.toggle("is-playing", playing);
        b.setAttribute("aria-label", playing ? "Pause" : "Play");
      });
    }

    function row(i, current, onPick) {
      const t = SONGS[i];
      const li = document.createElement("li");
      if (current) li.classList.add("is-current");
      const b = document.createElement("button");
      b.type = "button";
      b.innerHTML = `<img alt="" width="40" height="40" /><span class="r-text"><b></b><small></small></span><span class="r-time"></span>`;
      $("img", b).src = `assets/covers/cover-${t.cover}.webp`;
      $("b", b).textContent = t.title;
      $("small", b).textContent = ARTIST;
      $(".r-time", b).textContent = clock(t.dur);
      b.setAttribute("aria-label", `${current ? "Playing: " : "Play "}${t.title} by ${ARTIST}`);
      if (current) b.setAttribute("aria-current", "true");
      b.addEventListener("click", onPick);
      li.append(b);
      return li;
    }

    function drawLists() {
      all("[data-queue]").forEach((list) => {
        list.replaceChildren();
        st.order.forEach((i, at) => {
          if (at < st.pos) return;
          list.append(row(i, at === st.pos, () => play(at)));
        });
      });
      const q = st.query.trim().toLowerCase();
      $("[data-page='now']", wrap).hidden = !!q;
      $("[data-page='search']", wrap).hidden = !q;
      if (!q) return;
      $("[data-query]", wrap).textContent = q;
      const results = $("[data-results]", wrap);
      results.replaceChildren();
      const found = SONGS.map((t, i) => [t, i]).filter(([t]) => `${t.title} ${ARTIST}`.toLowerCase().includes(q));
      if (!found.length) {
        const li = document.createElement("li");
        li.className = "app-empty";
        li.textContent = `Nothing here matches “${st.query.trim()}”. These six songs are all by Kevin MacLeod.`;
        results.append(li);
        return;
      }
      for (const [, i] of found) {
        results.append(row(i, i === st.order[st.pos], () => {
          const at = st.order.indexOf(i);
          play(at);
        }));
      }
    }

    function load() {
      if (loaded === st.order[st.pos]) return;
      loaded = st.order[st.pos];
      audio.src = `assets/music/${song().file}.m4a`;
    }
    function play(at) {
      if (at !== undefined) {
        st.pos = at;
        audio.pause();
        audio.currentTime = 0;
      }
      load();
      listen();
      audio.play().catch(() => {});
      drawSong();
      drawLists();
    }
    function toggle() {
      if (audio.paused) play();
      else audio.pause();
    }
    function next(auto) {
      if (auto && st.repeat === "one") return play(st.pos);
      if (st.pos < st.order.length - 1) return play(st.pos + 1);
      if (st.repeat === "all" || !auto) return play(0);
      audio.pause();
    }
    function prev() {
      if (audio.currentTime > 3 || st.pos === 0) {
        audio.currentTime = 0;
        return drawTime();
      }
      play(st.pos - 1);
    }

    all("[data-play]").forEach((b) => b.addEventListener("click", toggle));
    all("[data-next]").forEach((b) => b.addEventListener("click", () => next(false)));
    all("[data-prev]").forEach((b) => b.addEventListener("click", prev));
    all("[data-like]").forEach((b) =>
      b.addEventListener("click", () => {
        const f = song().file;
        st.liked.has(f) ? st.liked.delete(f) : st.liked.add(f);
        drawSong();
      }),
    );
    const repeat = $("[data-repeat]", wrap);
    repeat.addEventListener("click", () => {
      st.repeat = { off: "all", all: "one", one: "off" }[st.repeat];
      repeat.dataset.mode = st.repeat;
      repeat.setAttribute("aria-pressed", String(st.repeat !== "off"));
      repeat.setAttribute("aria-label", `Repeat: ${st.repeat}`);
      drawSong();
    });
    const shuffle = $("[data-shuffle]", wrap);
    shuffle.addEventListener("click", () => {
      const head = st.order.slice(0, st.pos + 1);
      if (!st.ordered) {
        // Shuffle what is still to come; turning it off puts it back.
        st.ordered = st.order.slice(st.pos + 1);
        const rest = st.ordered.slice();
        for (let k = rest.length - 1; k > 0; k--) {
          const j = Math.floor(Math.random() * (k + 1));
          [rest[k], rest[j]] = [rest[j], rest[k]];
        }
        st.order = head.concat(rest);
      } else {
        st.order = head.concat(st.ordered);
        st.ordered = null;
      }
      shuffle.setAttribute("aria-pressed", String(!!st.ordered));
      drawSong();
      drawLists();
    });

    // Seeking and the volume: a click or a drag on the slider, or the arrows.
    function slider(el, set, step) {
      const at = (x) => {
        const r = el.getBoundingClientRect();
        return Math.min(Math.max((x - r.left) / r.width, 0), 1);
      };
      el.addEventListener("pointerdown", (e) => {
        el.setPointerCapture(e.pointerId);
        set(at(e.clientX));
        const move = (ev) => set(at(ev.clientX));
        el.addEventListener("pointermove", move);
        el.addEventListener("pointerup", () => el.removeEventListener("pointermove", move), { once: true });
      });
      el.addEventListener("keydown", (e) => {
        const d = { ArrowRight: 1, ArrowUp: 1, ArrowLeft: -1, ArrowDown: -1 }[e.key];
        if (!d) return;
        e.preventDefault();
        step(d);
      });
    }
    all("[data-seek]").forEach((el) =>
      slider(el, (r) => {
        load();
        audio.currentTime = r * duration();
        drawTime();
      }, (d) => {
        audio.currentTime = Math.min(Math.max(audio.currentTime + d * 5, 0), duration());
        drawTime();
      }),
    );
    const vol = $("[data-volume]", wrap);
    const mute = $("[data-mute]", wrap);
    const setVolume = (v) => {
      audio.volume = v;
      audio.muted = false;
      drawVolume();
    };
    const drawVolume = () => {
      const v = audio.muted ? 0 : audio.volume;
      $(".a-fill", vol).style.width = `${v * 100}%`;
      vol.setAttribute("aria-valuenow", String(Math.round(v * 100)));
      mute.classList.toggle("is-muted", v === 0);
      mute.setAttribute("aria-label", audio.muted ? "Unmute" : "Mute");
    };
    audio.volume = 0.8;
    slider(vol, setVolume, (d) => setVolume(Math.min(Math.max(audio.volume + d * 0.05, 0), 1)));
    mute.addEventListener("click", () => {
      audio.muted = !audio.muted;
      drawVolume();
    });

    // The side panel shows up next or the lyrics; these songs have none.
    const showPanel = (p) => {
      st.panel = p;
      $("[data-panel-title]", wrap).textContent = p === "lyrics" ? "lyrics" : "up next";
      $("[data-panel-queue]", wrap).hidden = p !== "queue";
      $("[data-panel-lyrics]", wrap).hidden = p !== "lyrics";
      all("[data-show]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.show === p)));
    };
    all("[data-show]").forEach((b) => b.addEventListener("click", () => showPanel(b.dataset.show)));

    // Search: typing turns the pane into the search page, as in the app.
    const search = $("[data-search]", wrap);
    search.addEventListener("input", () => {
      st.query = search.value;
      drawLists();
    });
    search.addEventListener("keydown", (e) => {
      if (e.key !== "Escape") return;
      search.value = st.query = "";
      drawLists();
    });
    $("[data-back]", wrap).addEventListener("click", () => {
      search.value = st.query = "";
      drawLists();
    });

    // The mini player, and back.
    all("[data-to-mini]").forEach((b) => b.addEventListener("click", () => setView("mini")));
    all("[data-to-full]").forEach((b) => b.addEventListener("click", () => setView("full")));
    function setView(v) {
      wrap.dataset.view = v;
      fit();
    }

    audio.addEventListener("timeupdate", drawTime);
    audio.addEventListener("loadedmetadata", drawTime);
    audio.addEventListener("play", drawPlay);
    audio.addEventListener("pause", drawPlay);
    audio.addEventListener("ended", () => next(true));

    // The visualizer: the spectrum of what plays over the foot of the
    // sleeve, 48 bars in the accent, as the app draws it.
    const canvas = $("[data-viz]", wrap);
    const ctx = canvas.getContext("2d");
    let analyser = null;
    let bins = null;
    function listen() {
      if (analyser || reduced.matches) return;
      try {
        const ac = new (window.AudioContext || window.webkitAudioContext)();
        const source = ac.createMediaElementSource(audio);
        analyser = ac.createAnalyser();
        analyser.fftSize = 256;
        analyser.smoothingTimeConstant = 0.8;
        source.connect(analyser);
        analyser.connect(ac.destination);
        bins = new Uint8Array(analyser.frequencyBinCount);
        audio.addEventListener("play", () => ac.resume());
        requestAnimationFrame(frame);
      } catch {
        analyser = null;
      }
    }
    function frame() {
      requestAnimationFrame(frame);
      const w = canvas.width;
      const h = canvas.height;
      ctx.clearRect(0, 0, w, h);
      if (audio.paused) return;
      analyser.getByteFrequencyData(bins);
      const bars = 48;
      const bw = w / bars;
      ctx.fillStyle = getComputedStyle(wrap).getPropertyValue("--art").trim() || "#00bde3";
      ctx.globalAlpha = 0.85;
      for (let i = 0; i < bars; i++) {
        // The lower bins carry the music: spread them over the bars.
        const v = bins[Math.floor((i * bins.length) / (bars * 2))] / 255;
        if (!v) continue;
        const bh = Math.max(2, v * h);
        ctx.fillRect(i * bw + 1, h - bh, bw - 2, bh);
      }
    }

    // The window is drawn at the app's 1280x760 and scaled to the column;
    // it lies down flat as it scrolls up to the middle.
    const frameEl = $("[data-app-frame]", wrap);
    const mini = $("[data-mini]", wrap);
    function fit() {
      const w = wrap.clientWidth;
      frameEl.style.setProperty("--fit", (w / 1280).toFixed(4));
      const narrow = w <= 700;
      mini.style.setProperty("--mfit", (narrow ? w / 420 : Math.min(1.5, w / 420)).toFixed(4));
    }
    addEventListener("resize", fit);
    fit();
    if (!reduced.matches) {
      const app = $("[data-app]", wrap);
      let ticking = false;
      const tilt = () => {
        ticking = false;
        const top = wrap.getBoundingClientRect().top;
        const t = Math.min(Math.max((top - innerHeight * 0.12) / (innerHeight * 0.6), 0), 1);
        app.style.setProperty("--tilt", `${(t * 10).toFixed(2)}deg`);
        app.style.setProperty("--tilt-scale", (1 - t * 0.03).toFixed(4));
      };
      addEventListener("scroll", () => {
        if (!ticking) {
          ticking = true;
          requestAnimationFrame(tilt);
        }
      }, { passive: true });
      tilt();
    }

    drawSong();
    drawPlay();
    drawLists();
    drawVolume();
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
      // lanes, one kept left and one kept right.
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
        const ey = top + k * step;
        // On phones each row keeps its place in the queue and only moves
        // sideways, out of its service's lane, so no two rows ever cross.
        const sy = stacked ? ey : startTop + i * step;
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
