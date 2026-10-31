// The MixMusic site: feature tabs, the walkthrough's chapters, the theme
// gallery, and download buttons named for the visitor's system. Every
// download goes to the latest release on GitHub.
(() => {
  "use strict";

  const REPO = "fzn0x/mixmusic";
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

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
