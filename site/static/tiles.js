// Feature cards that act out what they do (#358): the tiles of "New in" and
// the plugin card of "Make it yours", beside the theme's demo (demos.js).
// For each card below, this draws a
// picture of a small, simplified piece of the app, in place of the still
// one the page has without script, and a list of timed steps. The pictures
// play on their own, again and again, while their card is well in the
// window: the steps, a moment on how they end, back to how they start, and
// a moment there. Cards that come into the window together start a little
// apart, so that they don't move at once. A card out of the window goes
// back to its start, to play from there once back. A card holds where it is
// while it's hovered or has the keyboard's focus, which is how to stop one,
// and goes on once the pointer or the focus leaves. The songs, devices,
// channels and lyric lines are made up. tiles.css has the look.
//
// Under reduced motion nothing plays: each picture shows how it ends, and
// hovering or focusing a card changes nothing. Without script the page is
// as it was.
//
// An act is { card, art }. `card` is the heading the card has, and `art`
// draws its picture and returns { steps, rest, again, end }: `steps` is a
// list of [wait, change] pairs, each change made `wait` milliseconds after
// the one before it; `rest` puts the picture as it starts, at once;
// `again`, if there, are the steps that take it back there as it plays
// again, else `rest` alone; and `end`, if there, draws how it ends, else
// `rest` and every step at once do.
(() => {
  const reduced = () => (window.MixReveal ? window.MixReveal.reduced : matchMedia("(prefers-reduced-motion: reduce)").matches);
  const q = (el, s) => el.querySelector(s);
  const qa = (el, s) => [...el.querySelectorAll(s)];
  const make = (html) => {
    const t = document.createElement("template");
    t.innerHTML = html.trim();
    return t.content.firstElementChild;
  };
  // The marker data-motion is that of the check of reduced motion
  // (motion.css, #360): what moves carries it. The pictures are of the
  // app, in English, so they read left to right on every page.
  const art = (kind, inner) => `<div class="tile-art tile-act ta-${kind}" aria-hidden="true" dir="ltr" data-motion>${inner}</div>`;
  // put puts a picture in a card: in place of its still one, or before
  // `before`, else first.
  const put = (card, html, before) => {
    const el = make(html);
    const still = card.querySelector(":scope > .tile-art");
    if (still) still.replaceWith(el);
    else card.insertBefore(el, before || card.firstElementChild);
    return el;
  };
  // snap makes change at once, without the transitions of el and what it
  // holds.
  const snap = (el, change) => {
    el.classList.add("is-snap");
    change();
    void el.offsetWidth;
    el.classList.remove("is-snap");
  };
  // pop plays el's pop again.
  const pop = (el) => {
    el.classList.remove("is-pop");
    void el.offsetWidth;
    el.classList.add("is-pop");
  };
  // each is n steps, wait apart, the i-th of which calls change(i).
  const each = (n, wait, change) => Array.from({ length: n }, (_, i) => [wait, () => change(i + 1)]);
  const use = (id, size = 12) => `<svg class="icon" width="${size}" height="${size}"><use href="#${id}" /></svg>`;
  const check = '<span class="ta-check"><svg viewBox="0 0 12 12"><path d="M2.5 6.2 5 8.5l4.5-5" /></svg></span>';
  const clock = (s) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
  // seg is a segmented control of labels, whose mark segTo slides to the
  // i-th.
  const seg = (labels) => `<div class="ta-seg" style="--n: ${labels.length}"><span class="ta-seg-mark"></span>${labels.map((l) => `<span>${l}</span>`).join("")}</div>`;
  const segTo = (el, i) => {
    el.style.setProperty("--at", i);
    qa(el, "span:not(.ta-seg-mark)").forEach((s, j) => s.classList.toggle("is-on", j === i));
  };

  const acts = [];

  // A line lit word by word, then the next one rising into its place, with
  // its romaji and its translation.
  acts.push({
    card: "Lyrics in your language",
    art: (card) => {
      const el = put(card, art("lyrics", `
        <div class="ta-lyr-top"><span class="ta-pill">${use("i-lyrics")}Lyrics</span><span class="ta-pill is-accent">Romaji · English</span></div>
        <div class="ta-lyr"><p class="ly ly-past"></p><p class="ly ly-now"></p><p class="ly ly-roman"></p><p class="ly ly-trans"></p></div>`));
      const box = q(el, ".ta-lyr");
      const [past, now, roman, trans] = box.children;
      const lines = [
        { past: "朝ごはんを食べた", words: ["今日は", "いい", "天気"], roman: "kyō wa ii tenki", trans: "The weather is nice today" },
        { past: "今日はいい天気", words: ["散歩に", "行こう"], roman: "sanpo ni ikō", trans: "Let's go for a walk" },
      ];
      const show = (l) => {
        past.textContent = l.past;
        now.innerHTML = l.words.map((w) => `<span class="w">${w}<b>${w}</b></span>`).join("");
        roman.textContent = l.roman;
        trans.textContent = l.trans;
      };
      const sing = (n) => () => qa(now, ".w").forEach((w, i) => w.classList.toggle("is-sung", i < n));
      // The lines leave upwards, and the next ones rise into their place.
      const next = (wait, l) => [
        [wait, () => box.classList.add("is-leaving")],
        [250, () => snap(box, () => {
          box.classList.remove("is-leaving");
          box.classList.add("is-coming");
          show(l);
        })],
        [30, () => box.classList.remove("is-coming")],
      ];
      return {
        rest: () => snap(box, () => {
          box.classList.remove("is-leaving", "is-coming");
          show(lines[0]);
        }),
        steps: [[450, sing(1)], [380, sing(2)], [420, sing(3)], ...next(700, lines[1]), [560, sing(1)], [460, sing(2)]],
        again: next(0, lines[0]),
      };
    },
  });

  // An episode picked up where the phone left it, a little faster.
  acts.push({
    card: "Podcasts",
    art: (card) => {
      const total = 34 * 60 + 10;
      const left = 21 * 60 + 40;
      const el = put(card, art("pod", `
        <div class="ta-row"><span class="ta-cover ta-cover-pod"></span><span class="ta-text"><strong>Small Hours</strong><small>Episode 12 · made up for this page</small></span><span class="ta-chip">1×</span></div>
        <span class="ta-note">${use("i-mic")}<span></span></span>
        <div class="ta-bar"><i></i></div>
        <div class="ta-time"><span></span><span></span></div>`));
      const chip = q(el, ".ta-chip");
      const note = q(el, ".ta-note");
      const said = q(note, "span");
      const bar = q(el, ".ta-bar");
      const [now, rem] = qa(el, ".ta-time span");
      const at = (s) => {
        bar.style.setProperty("--ta-fill", (s / total).toFixed(4));
        now.textContent = clock(s);
        rem.textContent = `-${clock(total - s)}`;
      };
      return {
        rest: () => snap(el, () => {
          at(left);
          chip.textContent = "1×";
          chip.classList.remove("is-pop");
          note.classList.remove("is-on", "is-pop");
          said.textContent = `Left off at ${clock(left)} on your phone`;
        }),
        steps: [
          [700, () => {
            note.classList.add("is-on");
            said.textContent = `Picked up here at ${clock(left)}`;
            pop(note);
          }],
          [500, () => {
            chip.textContent = "1.2×";
            pop(chip);
          }],
          ...each(24, 110, (i) => at(left + i * 3)),
        ],
      };
    },
  });

  // A mix downloading: its songs ticked off one by one, and a bar of ticks
  // filling.
  acts.push({
    card: "Downloads",
    art: (card) => {
      const songs = ["Slow Static", "Kitsune Run", "Glass Garden"];
      const el = put(card, art("dl", `
        <div class="ta-row"><span class="ta-cover ta-cover-mix"></span><span class="ta-text"><strong>Night Drive mix</strong><small data-state></small></span><span class="ta-pct" data-percent></span></div>
        <ul class="ta-list">${songs.map((s) => `<li><span>${s}</span><span class="ta-ring"></span>${check}</li>`).join("")}</ul>
        <div class="ta-ticks">${"<i></i>".repeat(20)}</div>`));
      const state = q(el, "[data-state]");
      const percent = q(el, "[data-percent]");
      const rows = qa(el, ".ta-list li");
      const ticks = qa(el, ".ta-ticks i");
      // to shows n percent: two of the five songs had come at 40%, and each
      // of the others comes with 20% more.
      const to = (n) => {
        const done = Math.floor((n - 40) / 20);
        percent.textContent = `${n}%`;
        ticks.forEach((t, i) => t.classList.toggle("is-on", i < n / 5));
        rows.forEach((r, i) => {
          r.classList.toggle("is-done", i < done);
          r.classList.toggle("is-now", i === done);
        });
        state.textContent = n === 100 ? "Plays without the network" : `${2 + done} of 5 songs`;
      };
      return { rest: () => snap(el, () => to(40)), steps: each(12, 170, (i) => to(40 + i * 5)) };
    },
  });

  // The layouts, one after another, over a sketch of the app in each.
  acts.push({
    card: "Layouts",
    art: (card) => {
      const looks = ["blend", "expressive", "crate", "tiles"];
      const el = put(card, art("layouts", `${seg(["Blend", "Expressive", "Crate", "Tiles"])}
        <div class="ta-looks">${looks.map((l) => `<span class="ta-look" data-l="${l}"><i class="s"></i><i class="h"></i>${'<i class="c"></i>'.repeat(6)}<i class="p"></i></span>`).join("")}</div>`));
      const control = q(el, ".ta-seg");
      const sketches = qa(el, ".ta-look");
      const show = (i) => () => {
        segTo(control, i);
        sketches.forEach((s, j) => s.classList.toggle("is-shown", j === i));
      };
      return {
        rest: () => snap(el, show(0)),
        steps: [[1700, show(1)], [2100, show(2)], [2100, show(3)]],
        again: [[0, show(0)]],
        end: show(0),
      };
    },
  });

  // Stereo, then the 5.1 button and all six channels.
  acts.push({
    card: "5.1 surround",
    art: (card) => {
      const channels = [["L", 0.8, 760], ["R", 0.75, 820], ["C", 0.9, 700], ["LFE", 0.6, 980], ["Ls", 0.55, 880], ["Rs", 0.6, 940]];
      const el = put(card, art("surround", `${seg(["Stereo", "5.1"])}
        <div class="ta-chans">${channels.map(([c, lv, dur], i) => `<span class="ch" style="--lv: ${lv}; --dur: ${dur}ms; --i: ${i}"><b><s></s></b><small>${c}</small></span>`).join("")}</div>`));
      const control = q(el, ".ta-seg");
      const chans = qa(el, ".ch");
      const on = (n) => () => chans.forEach((c, i) => c.classList.toggle("is-on", i < n));
      const stereo = () => {
        segTo(control, 0);
        on(2)();
      };
      return {
        rest: () => snap(el, stereo),
        steps: [[1200, () => segTo(control, 1)], [160, on(3)], [110, on(4)], [110, on(5)], [110, on(6)]],
        again: [[0, stereo]],
      };
    },
  });

  // The devices coming in, the TV picked, and the cast under way.
  const glyphs = {
    laptop: '<svg viewBox="0 0 16 16"><path d="M3 3.5h10v7H3zM1.5 12.5h13" /></svg>',
    tv: '<svg viewBox="0 0 16 16"><path d="M1.5 3h13v8.5h-13zM5.5 14h5" /></svg>',
    speaker: '<svg viewBox="0 0 16 16"><path d="M4 1.5h8v13H4z" /><circle cx="8" cy="9.5" r="2.2" /></svg>',
  };
  acts.push({
    card: "Casting",
    art: (card) => {
      const el = put(card, art("cast", `
        <p class="ta-head">${use("i-cast", 13)}Play on</p>
        <div class="ta-devwrap"><span class="ta-sel"></span><ul class="ta-devs">
          <li>${glyphs.laptop}This computer</li>
          <li>${glyphs.tv}Living room TV<span class="ta-pulse"></span></li>
          <li>${glyphs.speaker}Kitchen speaker</li>
        </ul></div>
        <p class="ta-toast">${use("i-cast", 13)}Casting to Living room TV</p>`));
      const wrap = q(el, ".ta-devwrap");
      const devices = qa(el, ".ta-devs li");
      const toast = q(el, ".ta-toast");
      const pick = (i) => () => {
        wrap.style.setProperty("--at", i);
        devices.forEach((d, j) => d.classList.toggle("is-picked", i > 0 && j === i));
      };
      return {
        rest: () => snap(el, () => {
          wrap.classList.remove("is-in");
          toast.classList.remove("is-in");
          pick(0)();
        }),
        steps: [[350, () => wrap.classList.add("is-in")], [1100, pick(1)], [650, () => toast.classList.add("is-in")]],
        // The devices stay as it plays again: they come in as the card
        // does.
        again: [[0, () => toast.classList.remove("is-in")], [450, pick(0)]],
      };
    },
  });

  // The notch widening with the cover and the bars, then opening into the
  // player.
  acts.push({
    card: "A player around the notch",
    art: (card) => {
      const cover = `${document.documentElement.dataset.root || ""}assets/covers/cover-1.webp`;
      const el = put(card, art("notch", `
        <div class="ta-menubar"><span><i></i><i></i><i></i></span><span><i></i><i></i></span></div>
        <div class="ta-shape">
          <span class="ta-mini"><img src="${cover}" width="18" height="18" alt="" loading="lazy" /><b><s></s><s></s><s></s><s></s></b></span>
          <div class="ta-panel">
            <div class="ta-row"><img src="${cover}" width="36" height="36" alt="" loading="lazy" /><span class="ta-text"><strong>Neon Tide</strong><small>Kaori Ume</small></span></div>
            <div class="ta-bar"><i></i></div>
            <p>A wave of light across the bay</p>
          </div>
        </div>`));
      const shape = q(el, ".ta-shape");
      const bar = q(el, ".ta-bar");
      const fill = (f) => bar.style.setProperty("--ta-fill", f.toFixed(2));
      return {
        rest: () => snap(el, () => {
          shape.classList.remove("is-wide", "is-open");
          fill(0.3);
        }),
        steps: [[700, () => shape.classList.add("is-wide")], [1500, () => shape.classList.add("is-open")], ...each(8, 260, (i) => fill(0.3 + i * 0.03))],
        again: [[0, () => shape.classList.remove("is-open")], [700, () => shape.classList.remove("is-wide")]],
      };
    },
  });

  // A gauge whose needle steps up from normal speed to three times.
  acts.push({
    card: "Speed",
    art: (card) => {
      // The needle's angle for each speed: half speed at the left end, three
      // times at the right.
      const angle = { 0.5: -90, 1: -50, 1.25: -28, 1.5: -8, 2: 30, 3: 90 };
      const ticks = Object.values(angle)
        .map((a) => {
          const r = (a * Math.PI) / 180;
          const at = (d) => `${(75 + d * Math.sin(r)).toFixed(1)} ${(78 - d * Math.cos(r)).toFixed(1)}`;
          return `<path class="tick" d="M${at(50)} L${at(55)}" />`;
        })
        .join("");
      const el = put(card, art("speed", `
        <div class="ta-gauge"><svg viewBox="0 0 150 84"><path class="track" d="M13 78 A62 62 0 0 1 137 78" />${ticks}</svg><span class="ta-needle"></span><span class="ta-hub"></span></div>
        <div class="ta-ends"><span>½×</span><span class="ta-val"></span><span>3×</span></div>`));
      const needle = q(el, ".ta-needle");
      const value = q(el, ".ta-val");
      const set = (s) => {
        needle.style.setProperty("--a", `${angle[s]}deg`);
        value.textContent = `${s}×`;
      };
      const to = (s) => () => {
        set(s);
        pop(value);
      };
      return {
        rest: () => snap(el, () => {
          set(1);
          value.classList.remove("is-pop");
        }),
        steps: [[700, to(1.25)], [650, to(1.5)], [650, to(2)], [650, to(3)]],
        again: [[0, to(1)]],
      };
    },
  });

  // Keys of your own pressed over another app, and the next song.
  acts.push({
    card: "Global shortcuts",
    art: (card) => {
      const el = put(card, art("keysact", `
        <span class="ta-win"></span>
        <div class="ta-keys"><kbd>Ctrl</kbd><kbd>Alt</kbd><kbd>→</kbd></div>
        <div class="ta-np ta-row"><span class="ta-cover"></span><span class="ta-text ta-flip"><strong></strong><small></small></span></div>`));
      const keys = qa(el, "kbd");
      const flip = q(el, ".ta-flip");
      const [title, artist] = flip.children;
      const songs = [["Neon Tide", "Kaori Ume"], ["Slow Static", "Mira Vale"]];
      const set = ([t, a]) => {
        title.textContent = t;
        artist.textContent = a;
      };
      const up = () => keys.forEach((k) => k.classList.remove("is-down"));
      // The keys pressed, and the song after it coming in.
      const press = (song) => [
        [450, () => keys[0].classList.add("is-down")],
        [240, () => keys[1].classList.add("is-down")],
        [240, () => keys[2].classList.add("is-down")],
        [160, () => flip.classList.add("is-leaving")],
        [200, () => snap(flip, () => {
          flip.classList.remove("is-leaving");
          flip.classList.add("is-coming");
          set(song);
        })],
        [30, () => flip.classList.remove("is-coming")],
        [260, up],
      ];
      return {
        rest: () => snap(el, () => {
          up();
          flip.classList.remove("is-leaving", "is-coming");
          set(songs[0]);
        }),
        steps: press(songs[1]),
        again: press(songs[0]),
      };
    },
  });

  // The thumbs-down, and the queue passing over that song.
  const thumb = '<svg viewBox="0 0 24 24"><path d="M16.5 3.5h4v10h-4zM16.5 3.5H7.2a2 2 0 0 0-2 1.6l-1.6 7a2 2 0 0 0 2 2.4H10V18a2.5 2.5 0 0 0 2.5 2.5l4-7Z" /></svg>';
  acts.push({
    card: "Dislikes",
    art: (card) => {
      const songs = [["Glass Garden", "Juniper Lane"], ["Slow Static", "Mira Vale"], ["Kitsune Run", "Hoshino Club"], ["Paper Planets", "Velvet Ferries"]];
      const el = put(card, art("dislike", `
        <div class="ta-qwrap"><span class="ta-now"></span><ul class="ta-queue">${songs.map(([t, a]) => `<li><strong>${t}</strong><small>${a}</small><span class="ta-skip">Skipped</span><span class="ta-thumb">${thumb}</span></li>`).join("")}</ul></div>`));
      const wrap = q(el, ".ta-qwrap");
      const rows = qa(el, ".ta-queue li");
      const playing = (i) => () => {
        wrap.style.setProperty("--at", i);
        wrap.dataset.at = i;
      };
      return {
        rest: () => snap(el, () => {
          playing(0)();
          rows.forEach((r) => r.classList.remove("is-off", "is-skipped"));
        }),
        steps: [[700, () => rows[1].classList.add("is-off")], [900, playing(2)], [80, () => rows[1].classList.add("is-skipped")]],
        again: [[0, () => rows[1].classList.remove("is-off", "is-skipped")], [300, playing(0)]],
      };
    },
  });

  // A radio that plays all day, its picture drifting, its listeners coming.
  const muted = '<svg viewBox="0 0 24 24"><path d="M4 9.5h3.5L12 5.5v13l-4.5-4H4zM16 9.5l5 5M21 9.5l-5 5" /></svg>';
  acts.push({
    card: "Live streams and music videos",
    art: (card) => {
      const el = put(card, art("live", `
        <span class="ta-scene"></span><span class="ta-scene"></span><span class="ta-shade"></span>
        <span class="ta-badge"><i></i>LIVE</span>
        <span class="ta-muted">${muted}</span>
        <div class="ta-foot"><span class="ta-text"><strong>Velvet Ferries Radio · 24/7</strong><small></small></span><span class="ta-eq"><s></s><s></s><s></s><s></s><s></s></span></div>`));
      const listeners = q(el, ".ta-foot small");
      const count = (n) => () => (listeners.textContent = `${n.toLocaleString("en-US")} listening`);
      return {
        rest: count(1204),
        steps: [[1300, count(1207)], [1100, count(1212)], [1400, count(1214)], [1200, count(1219)]],
        again: [[0, count(1204)]],
      };
    },
  });

  // Songs one after another, nothing between them: the playhead fills a
  // song to its end and goes on into the next at once, as its title takes
  // the place of the last one's.
  acts.push({
    card: "No ads",
    art: (card) => {
      const songs = [["Neon Tide", "Kaori Ume", 190, "#52e4ea"], ["Slow Static", "Mira Vale", 178, "#7769d3"], ["Paper Planets", "Velvet Ferries", 204, "#ec8051"]];
      const el = put(card, art("noads", `
        <div class="ta-row"><span class="ta-cover"></span><span class="ta-text"><strong></strong><small></small></span></div>
        <div class="ta-strip">${songs.map(([, , s, c]) => `<span class="ta-part" style="--grow: ${s}; --tint: ${c}"></span>`).join("")}<span class="ta-ph"></span></div>
        <div class="ta-lens">${songs.map(([, , s]) => `<small style="--grow: ${s}">${clock(s)}</small>`).join("")}</div>`));
      const strip = q(el, ".ta-strip");
      const parts = qa(el, ".ta-part");
      const head = q(el, ".ta-ph");
      const cover = q(el, ".ta-cover");
      const [title, artist] = [q(el, ".ta-text strong"), q(el, ".ta-text small")];
      const glide = 1600;
      // fill plays f of song i, gliding there in ms: its part fills, and the
      // playhead moves to where it got, as a share of the strip.
      const fill = (i, f, ms) => () => {
        const at = (parts[i].offsetLeft + f * parts[i].offsetWidth) / Math.max(strip.clientWidth, 1);
        for (const e of [parts[i], head]) e.style.setProperty("--glide", `${ms}ms`);
        parts[i].style.setProperty("--fill", f);
        head.style.setProperty("--at", at);
      };
      const now = (i) => () => {
        [title.textContent, artist.textContent] = songs[i];
        cover.style.setProperty("--tint", songs[i][3]);
        parts.forEach((p, j) => p.classList.toggle("is-now", j === i));
      };
      return {
        rest: () =>
          snap(el, () => {
            now(0)();
            parts.forEach((p) => p.style.setProperty("--fill", 0));
            fill(0, 0, 0)();
          }),
        steps: [[400, fill(0, 1, glide)], [glide, now(1)], [0, fill(1, 1, glide)], [glide, now(2)], [0, fill(2, 0.6, glide)]],
      };
    },
  });

  // The marketplace: Spectrum installed, and its bars on now playing.
  acts.push({
    card: "Add a plugin",
    art: (card) => {
      const items = [["i-puzzle", "Sleep Timer", "Plugin"], ["i-puzzle", "Spectrum", "Plugin"], ["i-palette", "Neon Nights", "Theme"]];
      const bars = Array.from({ length: 18 }, (_, i) => `<s style="--lv: ${(0.35 + ((i * 37) % 60) / 100).toFixed(2)}; --dur: ${520 + ((i * 53) % 400)}ms; animation-delay: -${(i * 90) % 700}ms"></s>`).join("");
      const el = put(card, art("market", `
        <ul class="ta-items">${items.map(([icon, name, kind]) => `<li><span class="ta-ic">${use(icon, 14)}</span><span class="ta-text"><strong>${name}</strong><small>${kind}</small></span><span class="ta-btn"><span class="ta-lab">Install</span><i class="ta-spin"></i>${check}</span></li>`).join("")}</ul>
        <div class="ta-spectrum">${bars}</div>`), card.querySelector(":scope > .link"));
      const list = q(el, ".ta-items");
      const button = qa(el, ".ta-btn")[1];
      const label = q(button, ".ta-lab");
      const spectrum = q(el, ".ta-spectrum");
      const fresh = () => {
        button.classList.remove("is-press", "is-busy", "is-done");
        label.textContent = "Install";
      };
      return {
        rest: () => snap(el, () => {
          list.classList.remove("is-in");
          spectrum.classList.remove("is-in");
          fresh();
        }),
        steps: [
          [300, () => list.classList.add("is-in")],
          [1100, () => button.classList.add("is-press")],
          [160, () => button.classList.replace("is-press", "is-busy")],
          [1000, () => {
            button.classList.replace("is-busy", "is-done");
            label.textContent = "Enabled";
          }],
          [350, () => spectrum.classList.add("is-in")],
        ],
        // The list stays as it plays again: it comes in as the card does.
        again: [[0, () => spectrum.classList.remove("is-in")], [450, () => snap(button, fresh)]],
      };
    },
  });

  // How long a picture shows how it ends, how long it rests before it plays
  // again, and how far apart cards that come into the window together start,
  // in milliseconds.
  const hold = 1800;
  const pause = 900;
  const apart = 300;

  // Each card's picture, and the round it plays again and again.
  const cards = [];
  for (const act of acts) {
    // A page in another language keeps a heading's English in data-en
    // (tools/sitefinish).
    const card = qa(document, ".bento .tile, .columns .column").find((c) => {
      const h = c.querySelector("h3");
      return (h?.dataset.en || h?.textContent.trim()) === act.card;
    });
    if (!card) continue;
    const { steps, rest, again = [[0, rest]], end } = act.art(card);
    rest();
    cards.push({ card, steps, rest, end, round: [...steps, [hold, () => {}], ...again, [pause, () => {}]] });
  }

  // Under reduced motion each picture shows how it ends, whatever is hovered.
  if (reduced()) {
    for (const { steps, rest, end } of cards) {
      if (end) end();
      else {
        rest();
        steps.forEach(([, change]) => change());
      }
    }
    return;
  }

  // A card plays while it is well in the window and the page is shown,
  // unless it's hovered or has the keyboard's focus: then it holds where it
  // is, and goes on from there. Out of the window it goes back to its start.
  const players = new Map();
  for (const { card, rest, round } of cards) {
    let next = 0; // the change of the round that comes next
    let timer = null;
    let delay = 0; // how long it waits before it starts, after the cards that came in with it
    const now = { seen: !("IntersectionObserver" in window), hovered: false, focused: false };
    const step = (extra) => {
      const [wait, change] = round[next];
      timer = setTimeout(() => {
        change();
        next = (next + 1) % round.length;
        step(0);
      }, wait + extra);
    };
    const update = () => {
      card.classList.toggle("is-playing", now.seen && !document.hidden);
      card.classList.toggle("is-held", now.hovered || now.focused);
      const plays = now.seen && !now.hovered && !now.focused && !document.hidden;
      if (plays && !timer) {
        step(next === 0 ? delay : 0);
        delay = 0;
      } else if (!plays && timer) {
        clearTimeout(timer);
        timer = null;
      }
    };
    const reset = () => {
      clearTimeout(timer);
      timer = null;
      next = 0;
      rest();
    };
    players.set(card, { now, update, reset, wait: (ms) => (delay = ms) });

    // A card takes the keyboard's focus, which holds it.
    card.setAttribute("data-act", "");
    card.tabIndex = 0;
    const hover = (on) => (e) => {
      if (e.pointerType === "touch") return;
      now.hovered = on;
      update();
    };
    card.addEventListener("pointerenter", hover(true));
    card.addEventListener("pointerleave", hover(false));
    card.addEventListener("focus", () => {
      now.focused = card.matches(":focus-visible");
      update();
    });
    card.addEventListener("blur", () => {
      now.focused = false;
      update();
    });
    update();
  }

  if ("IntersectionObserver" in window) {
    // Well in: two fifths of the card, or half the window's height of it.
    // Cards that come in together start apart, in the page's order.
    const seen = new IntersectionObserver(
      (entries) => {
        let together = 0;
        for (const e of entries) {
          const p = players.get(e.target);
          const was = p.now.seen;
          p.now.seen = e.isIntersecting && (e.intersectionRatio >= 0.4 || e.intersectionRect.height >= innerHeight / 2);
          if (!e.isIntersecting) p.reset();
          if (p.now.seen && !was) p.wait(together++ * apart);
          p.update();
        }
      },
      { threshold: [0, 0.2, 0.4, 0.6, 0.8] },
    );
    for (const card of players.keys()) seen.observe(card);
  }
  document.addEventListener("visibilitychange", () => players.forEach((p) => p.update()));
})();
