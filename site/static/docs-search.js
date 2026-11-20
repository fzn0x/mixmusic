// The docs' search (#932, #938): a field in the docs' top app bar, whose
// results show under it as one types. A result is a section of a docs
// page that holds every word typed: its page, its heading and the words
// around the first one found. The index, docs/search.json, which
// tools/sitefinish makes from the built pages, comes once one starts to
// search. `/` puts the cursor in the field from anywhere on a docs page.
// A narrow window has no room for the field: the bar's search button, and
// `/`, open it over the bar, and its back button closes it. The arrows go
// through the results, Enter opens one at its heading, and Escape closes
// them, then empties the field, then, over a narrow bar, closes it.
(() => {
  const d = document;
  const box = d.querySelector("[data-search]");
  if (!box) return;
  const input = box.querySelector("input");
  const panel = box.querySelector("[data-search-panel]");
  const list = box.querySelector("[role=listbox]");
  const note = box.querySelector("[data-search-note]");
  const status = box.querySelector("[data-search-status]");
  const root = box.dataset.searchRoot; // the docs' address, ending in "/"
  const narrow = matchMedia("(max-width: 899.98px)");
  const LIMIT = 10; // results shown
  const PER_PAGE = 3; // of them, from one page at most

  // Words as the search compares them: in lower case and without accents,
  // so that "rós" finds "Ros" and "ROS".
  const fold = (s) => s.normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();
  const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  // A word typed matches where a word starts: "play" finds "playlists"
  // and "Play", not "display".
  const starts = (w, flags = "u") => new RegExp(`(?<![\\p{L}\\p{N}])${escape(w)}`, flags);

  let sections = null; // the index's sections, each with its page and folded words
  let loading = null;
  const load = () => {
    loading ??= fetch(root + "search.json")
      .then((r) => {
        if (!r.ok) throw new Error(`docs/search.json: ${r.status}`);
        return r.json();
      })
      .then(({ pages, sections: all }) => {
        sections = all.map((s, i) => {
          const page = pages[s.p];
          return { ...s, i, page, ft: fold(page.t), fh: fold(s.h || ""), fx: fold(s.x) };
        });
      })
      .catch((err) => {
        loading = null; // the next search tries again
        throw err;
      });
    return loading;
  };

  // score is how well a section answers the words: 0 when one of them is
  // nowhere in it, more for words in its heading or its page's title,
  // and for the words together, in the order typed.
  const score = (s, words, phrase) => {
    let total = 0;
    for (const w of words) {
      let got = 0;
      if (w.re.test(s.fh)) got += 12;
      if (w.re.test(s.ft)) got += 8;
      got += Math.min(s.fx.match(w.all)?.length || 0, 5) * 1.5;
      // Inside a word, as "list" in "playlists", it counts a little.
      if (!got && w.text.length >= 3 && (s.fh.includes(w.text) || s.ft.includes(w.text) || s.fx.includes(w.text))) got = 1;
      if (!got) return 0;
      total += got;
    }
    if (words.length > 1) {
      if (s.fh.includes(phrase)) total += 20;
      else if (s.fx.includes(phrase)) total += 6;
    }
    if (s.fh === phrase || (!s.h && s.ft === phrase)) total += 10;
    return total;
  };

  const find = (query) => {
    const texts = [...new Set(fold(query).split(/[^\p{L}\p{N}]+/u).filter(Boolean))].slice(0, 8);
    if (!texts.length) return { words: [], found: [] };
    const words = texts.map((text) => ({ text, re: starts(text), all: starts(text, "gu") }));
    const phrase = texts.join(" ");
    const ranked = [];
    for (const s of sections) {
      const n = score(s, words, phrase);
      if (n > 0) ranked.push([n, s]);
    }
    ranked.sort((a, b) => b[0] - a[0] || a[1].i - b[1].i);
    const found = [];
    const perPage = new Map();
    for (const [, s] of ranked) {
      const n = perPage.get(s.p) || 0;
      if (n >= PER_PAGE) continue;
      perPage.set(s.p, n + 1);
      found.push(s);
      if (found.length === LIMIT) break;
    }
    return { words, found };
  };

  // snippet is the words of a section around the first word found, cut
  // at spaces.
  const snippet = (text, words) => {
    const low = text.toLowerCase();
    let at = -1;
    for (const w of words) {
      const m = starts(w.text).exec(low);
      if (m && (at < 0 || m.index < at)) at = m.index;
    }
    let start = Math.max(0, at - 48);
    let end = Math.min(text.length, Math.max(at, 0) + 120);
    if (start > 0) {
      const space = text.indexOf(" ", start);
      if (space >= 0 && space < at) start = space + 1;
    }
    if (end < text.length) {
      const space = text.lastIndexOf(" ", end);
      if (space > start) end = space;
    }
    return (start > 0 ? "… " : "") + text.slice(start, end) + (end < text.length ? " …" : "");
  };

  // marked puts text in el with each word found in a <mark>.
  const marked = (el, text, words) => {
    const re = new RegExp(`(?<![\\p{L}\\p{N}])(${words.map((w) => escape(w.text)).join("|")})`, "giu");
    let last = 0;
    for (const m of text.matchAll(re)) {
      el.append(text.slice(last, m.index));
      const mark = d.createElement("mark");
      mark.textContent = m[0];
      el.append(mark);
      last = m.index + m[0].length;
    }
    el.append(text.slice(last));
  };

  let results = [];
  let active = -1;

  // In a wide window, the panel sits under the field, wider than it, and
  // moves left as far as it must to stay in the window; in a narrow one,
  // it takes the window's width under the bar (docs.css).
  const field = box.querySelector(".search-field");
  const place = () => {
    if (panel.hidden) return;
    if (narrow.matches) {
      panel.removeAttribute("style");
      return;
    }
    const r = field.getBoundingClientRect();
    const width = Math.min(Math.max(r.width, 560), innerWidth - 32);
    panel.style.left = `${Math.max(16, Math.min(r.left, innerWidth - 16 - width))}px`;
    panel.style.top = `${r.bottom + 8}px`;
    panel.style.width = `${width}px`;
    panel.style.maxHeight = `${Math.max(160, innerHeight - r.bottom - 24)}px`;
  };
  addEventListener("resize", place);

  // A narrow window shows the field over the bar while one searches.
  const bar = box.closest("[data-topbar]");
  const opener = d.querySelector("[data-search-open]");
  const searching = (on) => {
    bar?.classList.toggle("is-searching", on);
    opener?.setAttribute("aria-expanded", String(on));
  };
  const isSearching = () => bar?.classList.contains("is-searching");
  const leave = () => {
    close();
    searching(false);
    opener?.focus();
  };

  const open = () => {
    panel.hidden = false;
    input.setAttribute("aria-expanded", "true");
    place();
  };
  const close = () => {
    panel.hidden = true;
    input.setAttribute("aria-expanded", "false");
    setActive(-1);
  };
  const say = (words, shown) => {
    status.textContent = words;
    note.textContent = shown ? words : "";
    note.hidden = !shown;
  };
  const setActive = (i) => {
    list.querySelector("[aria-selected=true]")?.setAttribute("aria-selected", "false");
    active = i;
    const el = i >= 0 ? list.children[i] : null;
    if (!el) {
      input.removeAttribute("aria-activedescendant");
      return;
    }
    el.setAttribute("aria-selected", "true");
    input.setAttribute("aria-activedescendant", el.id);
    el.scrollIntoView({ block: "nearest" });
  };

  const show = () => {
    const query = input.value.trim();
    list.replaceChildren();
    results = [];
    setActive(-1);
    if (!query) {
      say("", false);
      close();
      return;
    }
    if (!sections) {
      open();
      say("Loading the search…", true);
      load().then(
        () => input.value.trim() === query && show(),
        () => input.value.trim() === query && say("The search didn't load. Check the connection, and try again.", true),
      );
      return;
    }
    const { words, found } = find(query);
    results = found;
    found.forEach((s, i) => {
      const li = d.createElement("li");
      li.id = `docs-search-result-${i}`;
      li.className = "search-result";
      li.setAttribute("role", "option");
      li.setAttribute("aria-selected", "false");
      const where = d.createElement("span");
      where.className = "search-where";
      const page = d.createElement("span");
      page.className = "search-page";
      page.textContent = s.page.t;
      where.append(page);
      if (s.h) {
        const heading = d.createElement("span");
        heading.className = "search-heading";
        marked(heading, s.h, words);
        where.append(heading);
      }
      const words_ = d.createElement("span");
      words_.className = "search-snippet";
      marked(words_, snippet(s.x, words), words);
      li.append(where, words_);
      li.addEventListener("click", () => go(s));
      list.append(li);
    });
    open();
    if (found.length) say(found.length === 1 ? "1 result" : `${found.length} results`, false);
    else say(`Nothing in the docs holds “${query}”.`, true);
  };

  // go opens a result at its heading. On the page shown, a narrow bar
  // closes its search, and the heading comes into view.
  const go = (s) => {
    const url = new URL(root + s.page.u + (s.a ? `#${s.a}` : ""), location.href);
    close();
    if (url.pathname !== location.pathname) {
      location.href = url.href;
      return;
    }
    searching(false);
    if (s.a) location.hash = s.a;
    else scrollTo(0, 0);
  };

  input.addEventListener("input", show);
  input.addEventListener("focus", () => {
    load().catch(() => {}); // early, while the visitor types
    if (input.value.trim()) show();
  });
  input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      if (!results.length) return;
      e.preventDefault();
      open();
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((active + step + results.length + (active < 0 && step < 0 ? 1 : 0)) % results.length);
    } else if (e.key === "Enter") {
      const s = results[active] || results[0];
      if (s) {
        e.preventDefault();
        go(s);
      }
    } else if (e.key === "Escape") {
      if (!panel.hidden) {
        e.preventDefault();
        e.stopPropagation();
        close();
      } else if (input.value) {
        e.preventDefault();
        e.stopPropagation();
        input.value = "";
        show();
      } else if (isSearching()) {
        e.preventDefault();
        e.stopPropagation();
        leave();
      }
    }
  });
  // A click on a result keeps the cursor in the field until it goes.
  list.addEventListener("mousedown", (e) => e.preventDefault());
  // Leaving the field closes the results, and a narrow bar's search once
  // the focus goes to something else on the page, or a tap lands off it.
  box.addEventListener("focusout", (e) => {
    if (box.contains(e.relatedTarget)) return;
    close();
    if (e.relatedTarget) searching(false);
  });
  d.addEventListener("pointerdown", (e) => {
    if (isSearching() && !box.contains(e.target)) searching(false);
  });
  box.querySelector("[data-search-close]")?.addEventListener("click", leave);
  narrow.addEventListener("change", () => {
    searching(false);
    place();
  });

  // `/`, and the top bar's search button, put the cursor in the field,
  // over the bar in a narrow window. The drawer open over one makes the
  // bar inert, so it closes first.
  const focusSearch = () => {
    if (d.documentElement.classList.contains("drawer-open")) d.querySelector(".scrim[data-drawer-close]")?.click();
    if (narrow.matches) searching(true);
    input.focus();
    input.select();
  };
  opener?.addEventListener("click", focusSearch);
  d.addEventListener("keydown", (e) => {
    if (e.key !== "/" || e.ctrlKey || e.metaKey || e.altKey || e.defaultPrevented) return;
    const t = e.target;
    if (t.isContentEditable || t.closest?.("input, textarea, select")) return;
    e.preventDefault();
    focusSearch();
  });
})();
