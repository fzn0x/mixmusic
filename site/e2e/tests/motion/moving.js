// What the reduced-motion check looks at (#360): the parts of the page that
// carry data-motion, which are the ones of every demo (#357) and feature tile
// (#358) that move. A new demo or tile marks its moving parts and is
// covered, with nothing to add here.

// A snapshot of how the marked parts look right now: where each is, how
// opaque, how transformed, and the animations running on it.
const snapshot = () => {
  const out = {};
  const roots = [...document.querySelectorAll("[data-motion]")];
  let n = 0;
  for (const root of roots) {
    const id = `${root.tagName.toLowerCase()}${root.id ? "#" + root.id : ""}[${n++}]`;
    for (const [i, el] of [root, ...root.querySelectorAll("*")].entries()) {
      const r = el.getBoundingClientRect();
      const css = getComputedStyle(el);
      const running = el.getAnimations().filter((a) => a.playState === "running").map((a) => a.animationName || a.transitionProperty || "an animation");
      out[`${id} ${i}:${el.tagName.toLowerCase()}${typeof el.className === "string" && el.className ? "." + el.className.trim().split(/\s+/).join(".") : ""}`] = {
        rect: [r.left, r.top, r.width, r.height].map((v) => Math.round(v * 10) / 10).join(","),
        opacity: css.opacity,
        transform: css.transform,
        translate: css.translate,
        scale: css.scale,
        running: running.join(" "),
      };
    }
  }
  return out;
};

const differences = (a, b, when) => {
  const out = [];
  for (const key of Object.keys(a)) {
    const x = a[key];
    const y = b[key];
    if (!y) continue;
    for (const field of Object.keys(x)) if (x[field] !== y[field]) out.push(`${key}: ${field} went from "${x[field]}" to "${y[field]}" ${when}`);
  }
  return out;
};

// moving returns what moves among the marked parts: while the page rests,
// after each one is scrolled to, and while it is hovered and focused.
export const moving = async (page) => {
  const problems = [];
  const count = await page.evaluate(() => document.querySelectorAll("[data-motion]").length);
  for (let i = 0; i < count; i++) {
    const root = page.locator("[data-motion]").nth(i);
    await root.evaluate((el) => el.scrollIntoView({ block: "center" }));
    // Whatever eases in has finished.
    await page.waitForTimeout(700);
    const name = await root.evaluate((el) => `${el.tagName.toLowerCase()}${typeof el.className === "string" && el.className ? "." + el.className.trim().split(/\s+/).join(".") : ""}`);

    const first = await page.evaluate(snapshot);
    await page.waitForTimeout(600);
    const second = await page.evaluate(snapshot);
    problems.push(...differences(first, second, `at rest, in ${name}`));

    const box = await root.boundingBox();
    if (box) await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await root.evaluate((el) => el.focus?.({ preventScroll: true }));
    await page.waitForTimeout(600);
    const hovered = await page.evaluate(snapshot);
    problems.push(...differences(second, hovered, `when hovered or focused, in ${name}`));
    await page.mouse.move(0, 0);
  }
  return problems;
};
