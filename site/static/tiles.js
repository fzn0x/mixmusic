// Feature tiles that act out what they do (#358). Each tile of the list
// below has a picture, drawn here or already in the page, and a list of
// timed steps. The pictures play on their own, again and again, while
// they are in the window: the steps, a moment on how they end, back to
// rest, and a moment at rest. They start a little apart, so that they don't
// move together. Out of the window a picture goes back to rest, to start
// from there once back. A tile holds where it is while it's hovered or has
// the keyboard's focus, which is how to stop one, and goes on once the
// pointer or the focus leaves. The songs and lyric lines are made up.
// tiles.css has the look.
//
// Under reduced motion nothing plays: the pictures stay at rest, and
// hovering or focusing a tile changes nothing. Without script the page is as
// it was.
//
// An act is { tile, art }. `tile` is the heading the tile has, and `art`
// draws the picture, or takes the one already in the tile, and returns
// { steps, rest, again }: `steps` is a list of [wait, change] pairs, each
// change made `wait` milliseconds after the one before it, `rest` puts the
// picture at rest at once, and `again`, if there, are the steps that take
// it back to rest as it plays again, else `rest` alone.
(() => {
  const reduced = () => (window.MixReveal ? window.MixReveal.reduced : matchMedia("(prefers-reduced-motion: reduce)").matches);
  const q = (el, s) => el.querySelector(s);
  const make = (html) => {
    const t = document.createElement("template");
    t.innerHTML = html.trim();
    return t.content.firstElementChild;
  };
  // The marker data-motion is that of the check of reduced motion
  // (motion.css, #360): what moves carries it.
  const art = (inner) => `<div class="tile-art tile-act" aria-hidden="true" data-motion>${inner}</div>`;
  const put = (tile, html) => {
    const el = make(html);
    tile.insertBefore(el, tile.firstElementChild);
    return el;
  };
  // snap makes change, to a bar, at once, without the fill's transition: a
  // bar back at rest doesn't run backwards.
  const snap = (bar, change) => () => {
    bar.classList.add("is-snap");
    change();
    void bar.offsetWidth;
    bar.classList.remove("is-snap");
  };

  const acts = [];

  // A line sung, then the song moves on to its next line.
  acts.push({
    tile: "Lyrics in your language",
    art: (tile) => {
      const el = q(tile, ".tile-art-lyrics");
      el.setAttribute("data-motion", "");
      const lines = ["ly-past", "ly-now", "ly-roman", "ly-trans"].map((c) => q(el, "." + c));
      const [past, now, roman, trans] = lines;
      const first = { past: past.textContent, now: now.innerHTML, roman: roman.textContent, trans: trans.textContent };
      const set = (s) => {
        past.textContent = s.past;
        now.innerHTML = s.now;
        roman.textContent = s.roman;
        trans.textContent = s.trans;
      };
      const onward = {
        past: "今日はいい天気",
        now: '<span class="ly-sung">散歩に</span>行こう',
        roman: "sanpo ni ikō",
        trans: "Let us go for a walk",
      };
      return {
        steps: [
          [500, () => (now.innerHTML = '<span class="ly-sung">今日はいい</span>天気')],
          [450, () => (now.innerHTML = '<span class="ly-sung">今日はいい天気</span>')],
          [600, () => lines.forEach((l) => l.classList.add("is-leaving"))],
          [230, () => {
            set(onward);
            lines.forEach((l) => l.classList.remove("is-leaving"));
          }],
        ],
        rest: () => {
          set(first);
          lines.forEach((l) => l.classList.remove("is-leaving"));
        },
        // The song's first lines come back as the others came.
        again: [
          [0, () => lines.forEach((l) => l.classList.add("is-leaving"))],
          [230, () => {
            set(first);
            lines.forEach((l) => l.classList.remove("is-leaving"));
          }],
        ],
      };
    },
  });

  // An episode starts where you left it, and plays on.
  acts.push({
    tile: "Podcasts",
    art: (tile) => {
      const el = put(tile, art(`
        <div class="ta-row"><span class="ta-cover"></span><span class="ta-text"><strong>Small Hours, episode 12</strong><small>Made up for this page</small></span></div>
        <div class="ta-bar" style="--ta-fill: 62%"><i></i></div>
        <div class="ta-time"><span data-now>21:40</span><span>34:10</span></div>`));
      const bar = q(el, ".ta-bar");
      const now = q(el, "[data-now]");
      const at = (fill, time) => () => {
        bar.style.setProperty("--ta-fill", fill);
        now.textContent = time;
      };
      return { steps: [[250, at("100%", "34:10")]], rest: snap(bar, at("62%", "21:40")) };
    },
  });

  // A download fills.
  acts.push({
    tile: "Downloads",
    art: (tile) => {
      const el = put(tile, art(`
        <div class="ta-row"><span class="ta-cover"></span><span class="ta-text"><strong>Night Drive mix</strong><small data-state>2 of 5 songs</small></span></div>
        <div class="ta-bar" style="--ta-fill: 40%"><i></i></div>
        <div class="ta-time"><span data-percent>40%</span><span>Plays without the network</span></div>`));
      const bar = q(el, ".ta-bar");
      const state = q(el, "[data-state]");
      const percent = q(el, "[data-percent]");
      const to = (n, text) => () => {
        bar.style.setProperty("--ta-fill", `${n}%`);
        state.textContent = text;
        percent.textContent = `${n}%`;
      };
      return { steps: [[250, to(100, "5 of 5 songs")]], rest: snap(bar, to(40, "2 of 5 songs")) };
    },
  });

  // The selection steps down and passes over the song that was disliked.
  const thumb = '<svg viewBox="0 0 24 24"><path d="M16.5 3.5h4v10h-4zM16.5 3.5H7.2a2 2 0 0 0-2 1.6l-1.6 7a2 2 0 0 0 2 2.4H10V18a2.5 2.5 0 0 0 2.5 2.5l4-7Z" /></svg>';
  acts.push({
    tile: "Dislikes",
    art: (tile) => {
      const el = put(tile, art(`
        <div class="ta-song ta-row is-on"><strong>Glass Garden</strong></div>
        <div class="ta-song ta-row is-off"><strong>Slow Static</strong>${thumb}</div>
        <div class="ta-song ta-row"><strong>Kitsune Run</strong></div>`));
      const songs = [...el.querySelectorAll(".ta-song")];
      const on = (n) => () => songs.forEach((s, i) => s.classList.toggle("is-on", i === n));
      return { steps: [[500, on(1)], [450, on(2)]], rest: on(0) };
    },
  });

  // Keys of your own are pressed, and the next song plays.
  acts.push({
    tile: "Global shortcuts",
    art: (tile) => {
      const el = put(tile, art(`
        <div class="ta-keys"><kbd>Ctrl</kbd><kbd>Alt</kbd><kbd>→</kbd></div>
        <div class="ta-row"><span class="ta-cover"></span><span class="ta-text"><strong data-title>Neon Tide</strong><small>Kaori Ume</small></span></div>`));
      const keys = [...el.querySelectorAll("kbd")];
      const title = q(el, "[data-title]");
      const down = (n) => () => keys[n].classList.add("is-down");
      const up = () => keys.forEach((k) => k.classList.remove("is-down"));
      // The keys pressed, and the song after it plays.
      const press = (song) => [
        [400, down(0)],
        [260, down(1)],
        [260, down(2)],
        [220, () => (title.textContent = song)],
        [300, up],
      ];
      return {
        steps: press("Slow Static"),
        rest: () => {
          up();
          title.textContent = "Neon Tide";
        },
        // Pressed again, they bring the first song back.
        again: press("Neon Tide"),
      };
    },
  });

  // The speed button's chip goes up a step at a time.
  acts.push({
    tile: "Speed",
    art: (tile) => {
      const chip = q(tile, ".tile-art-chip span");
      q(tile, ".tile-art-chip").setAttribute("data-motion", "");
      const set = (t) => () => (chip.textContent = t);
      return { steps: [[450, set("1.5×")], [450, set("2×")], [450, set("3×")]], rest: set("1.25×") };
    },
  });

  // How long a picture shows how it ends, how long it rests before it plays
  // again, and how far apart the tiles start, in milliseconds.
  const hold = 1800;
  const pause = 900;
  const apart = 350;

  // One picture for each tile, and the round it plays again and again.
  const tiles = [];
  for (const act of acts) {
    const tile = [...document.querySelectorAll(".bento .tile")].find((t) => t.querySelector("h3")?.textContent.trim() === act.tile);
    if (!tile) continue;
    const { steps, rest, again = [[0, rest]] } = act.art(tile);
    tiles.push({ tile, rest, round: [...steps, [hold, () => {}], ...again, [pause, () => {}]] });
  }

  // Under reduced motion the pictures stay at rest, whatever is hovered.
  if (reduced()) return;

  // A tile plays while it is in the window and the page is shown, unless
  // it's hovered or has the keyboard's focus: then it holds where it is, and
  // goes on from there. Out of the window it goes back to rest, and plays
  // from the start once back.
  const players = new Map();
  tiles.forEach(({ tile, rest, round }, n) => {
    let next = 0; // the change of the round that comes next
    let timer = null;
    let first = n * apart;
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
      const plays = now.seen && !now.hovered && !now.focused && !document.hidden;
      if (plays && !timer) {
        step(first);
        first = 0;
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
    players.set(tile, { now, update, reset });

    // A tile takes the keyboard's focus, which holds it.
    tile.setAttribute("data-act", "");
    tile.tabIndex = 0;
    const hover = (on) => (e) => {
      if (e.pointerType === "touch") return;
      now.hovered = on;
      update();
    };
    tile.addEventListener("pointerenter", hover(true));
    tile.addEventListener("pointerleave", hover(false));
    tile.addEventListener("focus", () => {
      now.focused = tile.matches(":focus-visible");
      update();
    });
    tile.addEventListener("blur", () => {
      now.focused = false;
      update();
    });
    update();
  });

  if ("IntersectionObserver" in window) {
    const seen = new IntersectionObserver((entries) => {
      for (const e of entries) {
        const p = players.get(e.target);
        p.now.seen = e.isIntersecting;
        if (!p.now.seen) p.reset();
        p.update();
      }
    });
    for (const tile of players.keys()) seen.observe(tile);
  }
  document.addEventListener("visibilitychange", () => players.forEach((p) => p.update()));
})();
