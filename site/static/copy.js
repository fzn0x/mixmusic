// Copy buttons on blocks of code and commands, on the home page (a <pre>
// with data-copy) and in the docs (each <pre> of a doc): one click, or
// Enter or Space, copies the block as it reads, without its last line
// break, the button says "Copied", and a live region tells screen readers
// so. Where the Clipboard API is missing or refused, as on a page not
// served over HTTPS, the button copies the way browsers did before it, and
// failing that selects the block, for Ctrl+C (⌘C on a Mac). Blocks and
// tables that scroll sideways take the keyboard's focus, to scroll them.
(() => {
  const d = document;
  // The docs' tables scroll sideways in a narrow window: the keyboard can
  // reach them, to scroll them with the arrow keys.
  d.querySelectorAll(".prose table").forEach((table) => (table.tabIndex = 0));
  const blocks = d.querySelectorAll(".prose pre, pre[data-copy]");
  if (!blocks.length) return;
  const mac = /Mac|iPhone|iPad/.test(navigator.userAgentData?.platform || navigator.platform || "");

  // One live region for the page, there before anything is said in it,
  // as screen readers only follow regions they have seen.
  const live = d.createElement("div");
  live.setAttribute("role", "status");
  live.className = "copy-live";
  Object.assign(live.style, { position: "absolute", width: "1px", height: "1px", overflow: "hidden", clipPath: "inset(50%)", whiteSpace: "nowrap" });
  d.body.append(live);
  let quiet;
  const say = (text) => {
    // Emptied first, so that "Copied" twice is said twice.
    live.textContent = "";
    clearTimeout(quiet);
    setTimeout(() => (live.textContent = text), 60);
    quiet = setTimeout(() => (live.textContent = ""), 4000);
  };

  // The icons, as the site draws them: two sheets, and a tick.
  const SVG = "http://www.w3.org/2000/svg";
  const PATHS = { copy: "M9 9h10.5v10.5H9zM15 9V4.5H4.5V15H9", done: "M5 12.5l4.5 4.5L19 7.5" };
  const icon = () => {
    const svg = d.createElementNS(SVG, "svg");
    svg.setAttribute("class", "icon");
    svg.setAttribute("width", "20");
    svg.setAttribute("height", "20");
    svg.setAttribute("viewBox", "0 0 24 24");
    svg.setAttribute("aria-hidden", "true");
    svg.append(d.createElementNS(SVG, "path"));
    return svg;
  };

  const copyOld = (text, back) => {
    const area = d.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    Object.assign(area.style, { position: "fixed", top: "0", left: "0", width: "1px", height: "1px", opacity: "0" });
    d.body.append(area);
    area.select();
    let ok = false;
    try {
      ok = d.execCommand("copy");
    } catch {}
    area.remove();
    back.focus();
    return ok;
  };
  const copy = async (text, back) => {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      return copyOld(text, back);
    }
  };

  blocks.forEach((pre) => {
    const code = pre.querySelector("code") || pre;
    // A block that scrolls sideways can be scrolled with the arrow keys
    // once the keyboard can reach it.
    pre.tabIndex = 0;
    const label = pre.dataset.copy || "Copy the code";
    const b = d.createElement("button");
    b.type = "button";
    b.className = "icon-btn copy";
    b.setAttribute("aria-label", label);
    b.title = label;
    const svg = icon();
    const tip = d.createElement("span");
    tip.className = "copy-tip";
    tip.setAttribute("aria-hidden", "true");
    b.append(svg, tip);
    const look = (state, text) => {
      svg.firstChild.setAttribute("d", state === "done" ? PATHS.done : PATHS.copy);
      b.classList.toggle("is-done", state === "done");
      b.classList.toggle("is-told", !!text);
      tip.textContent = text || "";
    };
    look("copy");
    let back;
    b.addEventListener("click", async () => {
      clearTimeout(back);
      if (await copy(code.textContent.replace(/\n$/, ""), b)) {
        look("done", "Copied");
        say("Copied");
      } else {
        // Nothing could copy: the block is selected for the keyboard.
        const range = d.createRange();
        range.selectNodeContents(code);
        const sel = getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
        const keys = mac ? "⌘C" : "Ctrl+C";
        look("copy", `Press ${keys}`);
        say(`Selected. Press ${keys} to copy it.`);
      }
      back = setTimeout(() => look("copy"), 2000);
    });
    const wrap = d.createElement("div");
    wrap.className = "code";
    pre.replaceWith(wrap);
    wrap.append(pre, b);
  });
})();
