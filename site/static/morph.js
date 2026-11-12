// Text that changes in place morphs into its new words: the letters the
// old and the new share slide to their new places, the rest fade out and
// in, and digits roll by place value. It is torph's TextMorph
// (vendor/torph.js, MIT), which keeps the words readable to a screen
// reader, and stands still under reduced motion.
//
// app.js hands it what it would otherwise set at once:
//
//   MixMorph.say(element, words)
//
// The change waits until the element is in the window, and, in the hero,
// until its entrance (reveal.css) has ended, so that it is seen. This is a
// module, which runs before app.js; without it, app.js sets the words.
import { TextMorph } from "./vendor/torph.js";

const morphs = new WeakMap();
const morph = (el) => {
  let m = morphs.get(el);
  if (!m) {
    m = new TextMorph({ element: el, duration: 700, ease: "cubic-bezier(0.22, 1, 0.36, 1)", locale: document.documentElement.lang || "en" });
    // The first update draws the words as they are, without a morph.
    m.update(el.textContent.trim());
    morphs.set(el, m);
  }
  return m;
};

// Resolves once the element has come into the window.
const seen = (el) =>
  new Promise((resolve) => {
    if (!window.MixReveal) resolve();
    else window.MixReveal.watch(el, { animate: false, onEnter: () => resolve() });
  });

// Resolves once what holds the element in the hero has risen in.
const risen = async (el) => {
  const step = el.closest(".hero-copy > *");
  if (!step) return;
  await Promise.all(step.getAnimations().map((a) => a.finished.catch(() => {})));
};

window.MixMorph = {
  say(el, words) {
    const m = morph(el);
    el.dataset.morphTo = words;
    seen(el)
      .then(() => risen(el))
      .then(() => new Promise((r) => setTimeout(r, 200)))
      .then(() => {
        // Only the newest words, if they were changed again meanwhile.
        if (el.dataset.morphTo === words) m.update(words);
      });
  },
};
