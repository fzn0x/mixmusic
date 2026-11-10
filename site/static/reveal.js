// Scroll-in animations of the home page, in every browser, with an
// IntersectionObserver: the children of each section fade up as they come
// into the window (reveal.css has the look). The page's HTML never hides
// anything: this script marks the elements it reveals, with the class rv,
// so without it, with reduced motion, or without IntersectionObserver,
// nothing is marked and everything shows.
//
// The observer is reusable. Other parts of the page watch an element
// through window.MixReveal, which this script sets when it runs, before
// the scripts that follow it:
//
//   MixReveal.watch(elements, { animate, onEnter, rise })
//     elements  an element or a list of them.
//     animate   true (the default) marks them to fade up; false only
//               watches them. Marking is skipped under reduced motion.
//     onEnter   called once with the element, the first time it is in the
//               window, with reduced motion too. Where there is no
//               IntersectionObserver it is called at once, so that what
//               waits for it starts anyway.
//     rise      how many pixels it rises from (24 by default).
//     Returns a function that stops watching.
//
//   MixReveal.reduced   true when the visitor asked for reduced motion.
//
// Elements that come in together keep 80 ms apart, up to three steps, so
// that a heading and what is under it rise one after another.
(() => {
  const reduced = matchMedia("(prefers-reduced-motion: reduce)");
  const STEP = 80;
  const MOST = 3;
  const watching = new Map();

  const show = (el, now, delay) => {
    const options = watching.get(el);
    if (!options) return;
    watching.delete(el);
    io?.unobserve(el);
    if (options.marked) {
      if (now) el.classList.add("rv-now");
      else if (delay) el.style.setProperty("--rv-delay", `${delay}ms`);
      el.classList.add("is-in");
    }
    options.onEnter?.(el);
  };

  // The bottom edge of the window is left out by a tenth, so that an
  // element is seen to rise rather than to be there already.
  const io =
    "IntersectionObserver" in window
      ? new IntersectionObserver(
          (entries) => {
            let n = 0;
            for (const entry of entries) {
              if (entry.isIntersecting) show(entry.target, false, Math.min(n++, MOST) * STEP);
              // Above the window already, as when a link led past it:
              // there is nothing to watch come in.
              else if (entry.boundingClientRect.bottom <= 0) show(entry.target, true, 0);
            }
          },
          { rootMargin: "0px 0px -10% 0px" },
        )
      : null;

  // A link or a reload can take the page past elements without one of them
  // ever being in the window, which the observer reports no change of: while
  // some are watched, each scroll looks for those left above it.
  let asking = false;
  const past = () => {
    asking = false;
    for (const el of [...watching.keys()]) if (el.getBoundingClientRect().bottom <= 0) show(el, true, 0);
    if (!watching.size) removeEventListener("scroll", ask);
  };
  const ask = () => {
    if (asking) return;
    asking = true;
    requestAnimationFrame(past);
  };

  const watch = (elements, { animate = true, onEnter, rise } = {}) => {
    const list = elements instanceof Element ? [elements] : [...elements];
    for (const el of list) {
      if (watching.has(el)) continue;
      if (!io) {
        onEnter?.(el);
        continue;
      }
      const marked = animate && !reduced.matches;
      if (marked) {
        el.classList.add("rv");
        if (rise) el.style.setProperty("--rv-rise", `${rise}px`);
      }
      watching.set(el, { marked, onEnter });
      io.observe(el);
    }
    if (watching.size) addEventListener("scroll", ask, { passive: true });
    return () =>
      list.forEach((el) => {
        if (!watching.delete(el)) return;
        io?.unobserve(el);
        el.classList.remove("rv", "is-in", "rv-now");
      });
  };

  window.MixReveal = {
    watch,
    get reduced() {
      return reduced.matches;
    },
  };

  // Each section's children come in as the visitor reaches them: the
  // heading, then what it heads. The scroll of #one-queue, which is not a
  // .section, has its own motion.
  watch(document.querySelectorAll("main > .section > *"));
})();
