// The MixMusic site: the page plays like a queue. The up next column lists
// the sections, the player bar names the one in view, its progress line is
// the scroll position, and its play button plays the page section by section.
(() => {
  "use strict";

  const REPO = "fzn0x/mixmusic";
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];

  const tracks = $$("[data-track]");
  const list = $("[data-upnext]");
  const playerText = $(".player-text");
  const playerTitle = $("[data-player-title]");
  const playerSub = $("[data-player-sub]");
  const playerThumb = $("[data-player-thumb]");
  const fill = $("[data-progress-fill]");
  const progress = $("[data-progress]");
  const playButton = $("[data-play]");

  const iconUse = (name) =>
    `<svg width="22" height="22" aria-hidden="true"><use href="#i-${name}" /></svg>`;

  // The up next column: one queue row per section.
  const rows = tracks.map((section) => {
    const li = document.createElement("li");
    const a = document.createElement("a");
    a.href = `#${section.id}`;
    a.innerHTML = `
      <span class="tile tile-thumb" data-tone="${section.dataset.tone}" aria-hidden="true">${iconUse(section.dataset.icon)}</span>
      <span><strong></strong><small><span class="mark mark-mix" aria-hidden="true"></span><span></span></small></span>`;
    $("strong", a).textContent = section.dataset.track;
    $("small span:last-child", a).textContent = section.dataset.caption;
    li.append(a);
    list.append(li);
    return a;
  });

  // Short windows: the column steps down its rows, captions last, until the
  // whole queue, down to its end line, is in view.
  const column = list.closest(".upnext");
  function fit() {
    if (column.offsetParent === null) return;
    for (let level = 0; level <= 3; level++) {
      for (let n = 1; n <= 3; n++) column.classList.toggle(`fit-${n}`, n <= level);
      if (column.scrollHeight <= column.clientHeight) return;
    }
  }
  fit();
  addEventListener("resize", fit);
  document.fonts?.ready.then(fit);

  let current = -1;

  function setCurrent(index) {
    if (index === current) return;
    current = index;
    rows.forEach((row, i) => {
      row.toggleAttribute("aria-current", i === index);
      if (i === index) row.setAttribute("aria-current", "true");
      row.classList.toggle("is-played", i < index);
    });
    const section = tracks[index];
    const next = tracks[index + 1];
    playerTitle.textContent = section.dataset.track;
    playerSub.textContent = next ? `next: ${next.dataset.track}` : "the end of the queue";
    playerThumb.dataset.tone = section.dataset.tone;
    playerThumb.innerHTML = iconUse(section.dataset.icon);
    if (!reduced.matches) {
      playerText.classList.remove("is-turning");
      void playerText.offsetWidth;
      playerText.classList.add("is-turning");
    }
    // Keep the playing row in view inside the column, scrolling only as needed.
    const row = rows[index];
    const box = row.closest(".upnext");
    if (box && box.offsetParent !== null) {
      const top = row.offsetTop; // the sticky column is the rows' offset parent
      const bottom = top + row.offsetHeight;
      let to = null;
      if (top < box.scrollTop) to = Math.max(0, top - 80);
      else if (bottom > box.scrollTop + box.clientHeight) to = bottom - box.clientHeight + 16;
      if (to !== null) box.scrollTo({ top: to, behavior: reduced.matches ? "auto" : "smooth" });
    }
  }

  // The section in view is the last one whose top passed 40% of the viewport.
  function onScroll() {
    const line = innerHeight * 0.4;
    let index = 0;
    tracks.forEach((section, i) => {
      if (section.getBoundingClientRect().top <= line) index = i;
    });
    if (innerHeight + scrollY >= document.documentElement.scrollHeight - 4) index = tracks.length - 1;
    setCurrent(index);

    const max = document.documentElement.scrollHeight - innerHeight;
    const ratio = max > 0 ? Math.min(1, Math.max(0, scrollY / max)) : 0;
    fill.style.width = `${ratio * 100}%`;
    progress.setAttribute("aria-valuenow", String(Math.round(ratio * 100)));

    slideWords();
  }

  // Each giant word runs past its column; as it scrolls up through the
  // window, it slides left to show its cropped tail, as the app's page words do.
  const words = $$(".word");
  function slideWords() {
    if (reduced.matches) return;
    for (const word of words) {
      const frame = word.parentElement;
      const overflow = word.scrollWidth - frame.clientWidth;
      if (overflow <= 0) {
        word.style.setProperty("--slide", "0px");
        continue;
      }
      // The page word slides over the first 280px of scroll; a section's
      // word slides as it rises from 60% of the window to its top fifth.
      const t = word.classList.contains("word-hero")
        ? scrollY / 280
        : (innerHeight * 0.6 - frame.getBoundingClientRect().top) / (innerHeight * 0.4);
      word.style.setProperty("--slide", `${(overflow + 24) * Math.min(1, Math.max(0, t))}px`);
    }
  }

  let ticking = false;
  const schedule = () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(() => {
      ticking = false;
      onScroll();
    });
  };
  addEventListener("scroll", schedule, { passive: true });
  addEventListener("resize", schedule);

  // Sections swing in once, when they first arrive.
  const seen = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (e.isIntersecting) {
          e.target.classList.add("is-seen");
          seen.unobserve(e.target);
        }
      }
    },
    { rootMargin: "0px 0px -15% 0px" },
  );
  $$(".track").forEach((t) => seen.observe(t));

  // Transport: previous, next, and playing the page.
  function go(index) {
    const target = tracks[Math.max(0, Math.min(tracks.length - 1, index))];
    target.scrollIntoView({ behavior: reduced.matches ? "auto" : "smooth", block: "start" });
  }
  $("[data-prev]").addEventListener("click", () => {
    stop();
    go(current - 1);
  });
  $("[data-next]").addEventListener("click", () => {
    stop();
    go(current + 1);
  });

  let timer = 0;
  function stop() {
    clearInterval(timer);
    timer = 0;
    playButton.setAttribute("aria-pressed", "false");
    playButton.setAttribute("aria-label", "Play the page, one section after another");
  }
  function play() {
    playButton.setAttribute("aria-pressed", "true");
    playButton.setAttribute("aria-label", "Pause the page");
    if (current >= tracks.length - 1) go(0);
    else go(current + 1);
    timer = setInterval(() => {
      if (current >= tracks.length - 1) return stop();
      go(current + 1);
    }, 6500);
  }
  playButton.addEventListener("click", () => (timer ? stop() : play()));
  // The visitor taking the wheel pauses the tour.
  for (const type of ["wheel", "touchstart", "keydown"]) {
    addEventListener(type, (e) => {
      if (!timer) return;
      if (type === "keydown" && (e.target === playButton || e.key === "Tab")) return;
      stop();
    }, { passive: true });
  }

  // Seeking on the progress line.
  function seek(ratio) {
    const max = document.documentElement.scrollHeight - innerHeight;
    scrollTo({ top: max * ratio, behavior: reduced.matches ? "auto" : "smooth" });
  }
  progress.addEventListener("click", (e) => {
    const r = progress.getBoundingClientRect();
    stop();
    seek((e.clientX - r.left) / r.width);
  });
  progress.addEventListener("keydown", (e) => {
    const step = { ArrowRight: 1, ArrowUp: 1, ArrowLeft: -1, ArrowDown: -1 }[e.key];
    if (!step) return;
    e.preventDefault();
    go(current + step);
  });

  // Theme gallery: preview each theme's home page.
  const gallery = $("[data-gallery]");
  if (gallery) {
    // The second layer of the crossfade is made on the first switch, so no
    // hidden image loads before anyone asks for a theme.
    const a = $(".gallery-img", gallery);
    const b = a.cloneNode();
    b.classList.remove("is-shown");
    b.removeAttribute("fetchpriority");
    b.alt = "";
    b.setAttribute("aria-hidden", "true");
    let shown = a;
    let mounted = false;
    const name = $("[data-gallery-name]", gallery);
    const desc = $("[data-gallery-desc]", gallery);
    $$(".theme-card", gallery).forEach((card) => {
      const pre = new Image();
      card.addEventListener("pointerenter", () => (pre.src = `assets/shots/theme-${card.dataset.theme}.webp`), { once: true });
      card.addEventListener("click", () => {
        if (card.classList.contains("is-current")) return;
        $$(".theme-card", gallery).forEach((c) => {
          c.classList.toggle("is-current", c === card);
          c.setAttribute("aria-pressed", String(c === card));
        });
        if (!mounted) {
          a.after(b);
          mounted = true;
        }
        const next = shown === a ? b : a;
        next.src = `assets/shots/theme-${card.dataset.theme}.webp`;
        next.alt = `The home page in the ${card.dataset.name} theme.`;
        const reveal = () => {
          next.classList.add("is-shown");
          next.removeAttribute("aria-hidden");
          shown.classList.remove("is-shown");
          shown.setAttribute("aria-hidden", "true");
          shown.alt = "";
          shown = next;
        };
        next.complete ? reveal() : next.addEventListener("load", reveal, { once: true });
        name.textContent = card.dataset.name;
        desc.textContent = card.dataset.desc;
      });
    });
  }

  // The visitor's system picks the download.
  const ua = navigator.userAgent;
  const os = /Mac|iPhone|iPad/.test(navigator.platform || ua) && !/iPhone|iPad/.test(ua)
    ? "mac"
    : /Win/.test(navigator.platform || ua)
      ? "windows"
      : /Linux|X11/.test(ua) && !/Android/.test(ua)
        ? "linux"
        : "other";
  if (os === "mac") $$("kbd[data-mod]").forEach((k) => (k.textContent = "⌘"));

  const labels = { mac: "download for macos", windows: "download for windows", linux: "download for linux" };
  const pickAsset = async () => {
    if (os === "mac") return "macos-universal.dmg";
    let arm = /aarch64|arm64/i.test(ua);
    try {
      const hints = await navigator.userAgentData?.getHighEntropyValues?.(["architecture"]);
      if (hints?.architecture) arm = hints.architecture === "arm";
    } catch {}
    if (os === "windows") return arm ? "windows-arm64-setup.exe" : "windows-amd64-setup.exe";
    if (os === "linux") return arm ? "linux-arm64.deb" : "linux-amd64.deb";
    return null;
  };

  function assetLink(asset) {
    return $(`.dl-file[data-asset="${asset}"]`)?.href;
  }

  async function wireDownloads() {
    const asset = await pickAsset();
    const href = asset && assetLink(asset);
    $$("[data-download]").forEach((btn) => {
      if (href) {
        btn.href = href;
        $("[data-download-label]", btn).textContent = labels[os];
      }
    });
    const tile = $(`.dl[data-os="${os}"]`);
    if (tile) tile.classList.add("is-yours");
    const metaSize = asset && $(`.dl-file[data-asset="${asset}"] .size`)?.textContent;
    const meta = $("[data-download-meta]");
    if (meta && metaSize) meta.innerHTML = `Version <span data-version>${$("[data-version]").textContent}</span>, ${metaSize}`;
  }

  // The latest release, when it is newer than the one the page was built with.
  async function latestRelease() {
    try {
      const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, { headers: { Accept: "application/vnd.github+json" } });
      if (!res.ok) return;
      const rel = await res.json();
      const version = String(rel.tag_name || "").replace(/^v/, "");
      if (!version) return;
      // The page was built with the release in data-version; a newer one
      // carries what the page marks as the next release.
      if (version !== $("[data-version]").textContent.trim()) $$("[data-soon]").forEach((el) => el.remove());
      for (const file of $$(".dl-file[data-asset]")) {
        const suffix = file.dataset.asset;
        const found = (rel.assets || []).find((x) => x.name === suffix || x.name.endsWith(`-${suffix}`));
        if (!found) continue;
        file.href = found.browser_download_url;
        const mb = found.size / 1e6;
        $(".size", file).textContent = mb >= 1 ? `${mb.toFixed(1)} MB` : `${Math.round(found.size / 1e3)} KB`;
      }
      $$("[data-version]").forEach((v) => (v.textContent = version));
    } catch {}
  }

  latestRelease().finally(wireDownloads);
  onScroll();
})();
