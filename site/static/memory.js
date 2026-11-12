// The memory section: each figure counts up from nought as its card comes
// into the window, and its bar grows with it, on a spring that eases to a
// stop, as Magic UI's Number Ticker does (magicui.design, MIT). Its spring
// is stiffer here, so that a figure is at its true value within about a
// second and a half, not creeping up to it for seconds. A light under the
// pointer follows it around each card's edge, as Magic UI's Magic Card
// does.
//
// The page has the figures and full bars as written, so without script,
// with reduced motion, and to search engines they are there at once.
(() => {
  const cards = [...document.querySelectorAll("[data-mem] .mem-card")];
  if (!cards.length) return;
  const reduced = () => window.MixReveal?.reduced ?? matchMedia("(prefers-reduced-motion: reduce)").matches;
  // The figures are written in Western digits on every page (the HTML has
  // them so, untranslated), and count in the same.
  const format = new Intl.NumberFormat("en", { maximumFractionDigits: 0 });

  // A spring from 0 to 1, damped just past critical so it never
  // overshoots: stiffness 20, damping 9, mass 1, 99% there at 1.5 s.
  const STIFFNESS = 20;
  const DAMPING = 9;
  const w = Math.sqrt(STIFFNESS);
  const zeta = DAMPING / (2 * w);
  const r1 = -w * (zeta - Math.sqrt(zeta * zeta - 1));
  const r2 = -w * (zeta + Math.sqrt(zeta * zeta - 1));
  const spring = (t) => 1 - (r2 * Math.exp(r1 * t) - r1 * Math.exp(r2 * t)) / (r2 - r1);

  const count = (card, delay) => {
    const figure = card.querySelector("[data-count]");
    const target = Number(figure.dataset.count);
    figure.textContent = format.format(0);
    card.style.setProperty("--grow", "0");
    let start = 0;
    const frame = (now) => {
      if (!start) start = now + delay;
      const t = Math.max(0, now - start) / 1000;
      const p = spring(t);
      const done = target - p * target < 0.5;
      figure.textContent = format.format(done ? target : Math.round(p * target));
      card.style.setProperty("--grow", done ? "1" : String(p));
      if (!done) requestAnimationFrame(frame);
    };
    requestAnimationFrame(frame);
  };

  if (!reduced() && window.MixReveal) {
    cards.forEach((card, i) => {
      card.style.setProperty("--grow", "0");
      window.MixReveal.watch(card, { animate: false, onEnter: () => count(card, i * 150) });
    });
  }

  // The light under the pointer, where there is a pointer that hovers.
  if (matchMedia("(hover: hover)").matches) {
    cards.forEach((card) => {
      card.addEventListener("pointermove", (e) => {
        const box = card.getBoundingClientRect();
        card.style.setProperty("--mx", `${e.clientX - box.left}px`);
        card.style.setProperty("--my", `${e.clientY - box.top}px`);
      });
    });
  }
})();
