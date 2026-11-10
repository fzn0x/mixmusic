// Feature tiles that act out what they do (#358). Each tile of the list
// below has a picture, drawn here or already in the page, and a list of
// timed steps. The picture plays while the tile is hovered or has the
// keyboard's focus, and goes back to rest when the pointer or the focus
// leaves. Where there is no pointer to hover with (a touch screen), each
// tile plays once on its own as it scrolls into the window, the tiles that
// come in together one after another. The songs and lyric lines are made
// up. tiles.css has the look.
//
// Under reduced motion nothing plays: the pictures stay at rest, and
// hovering or focusing a tile changes nothing. Without script the page is as
// it was.
//
// An act is { tile, art }. `tile` is the heading the tile has, and `art`
// draws the picture, or takes the one already in the tile, and returns
// { steps, rest }: `steps` is a list of [wait, change] pairs, each change
// made `wait` milliseconds after the one before it, and `rest` puts the
// picture at rest again.
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
      return { steps: [[250, at("100%", "34:10")]], rest: at("62%", "21:40") };
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
      return { steps: [[250, to(100, "5 of 5 songs")]], rest: to(40, "2 of 5 songs") };
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
      return {
        steps: [
          [400, down(0)],
          [260, down(1)],
          [260, down(2)],
          [220, () => (title.textContent = "Slow Static")],
          [300, up],
        ],
        rest: () => {
          up();
          title.textContent = "Neon Tide";
        },
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

  // One running picture for each tile.
  const tiles = [];
  for (const act of acts) {
    const tile = [...document.querySelectorAll(".bento .tile")].find((t) => t.querySelector("h3")?.textContent.trim() === act.tile);
    if (!tile) continue;
    const { steps, rest } = act.art(tile);
    let timers = [];
    const back = () => {
      timers.forEach(clearTimeout);
      timers = [];
      rest();
    };
    const play = (delay = 0) => {
      back();
      let at = delay;
      for (const [wait, change] of steps) {
        at += wait;
        timers.push(setTimeout(change, at));
      }
    };
    tiles.push({ tile, play, back });
  }

  // Under reduced motion the pictures stay at rest, whatever is hovered.
  if (reduced()) return;

  for (const { tile, play, back } of tiles) {
    // A tile that plays takes the keyboard's focus, to play for it too.
    tile.setAttribute("data-act", "");
    tile.tabIndex = 0;
    tile.addEventListener("pointerenter", (e) => e.pointerType !== "touch" && play());
    tile.addEventListener("pointerleave", (e) => e.pointerType !== "touch" && !tile.matches(":focus-visible") && back());
    tile.addEventListener("focus", () => tile.matches(":focus-visible") && play());
    tile.addEventListener("blur", back);
  }

  // Where nothing hovers, each tile plays once, as it scrolls in: the ones
  // that come in together, a little apart.
  if (matchMedia("(hover: none)").matches && window.MixReveal) {
    let together = 0;
    for (const t of tiles) {
      window.MixReveal.watch(t.tile, {
        animate: false,
        onEnter: () => {
          t.play(together++ * 220);
          setTimeout(() => (together = 0), 50);
        },
      });
    }
  }
})();
