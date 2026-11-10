import { expect, load, test } from "./page.js";

// The bar at the top of a page stays on one line, inside itself, at every
// width, and not only at the phone's and the desktop's (#426): the home
// page's nav wrapped out of its bar at about 820 px.
const bars = [
  { name: "home", path: "/", bar: ".appbar-inner" },
  { name: "docs", path: "/docs/", bar: ".topbar" },
];
const widths = [480, 600, 700, 761, 800, 820, 860, 900, 960, 1000, 1080, 1100, 1280, 1440];

for (const { name, path, bar } of bars) {
  test(`the ${name} page's top bar keeps its labels on one line inside it, at every width`, async ({ page }) => {
    await load(page, path);
    const bad = [];
    for (const width of widths) {
      await page.setViewportSize({ width, height: 800 });
      const found = await page.evaluate((selector) => {
        const box = document.querySelector(selector).getBoundingClientRect();
        const out = [];
        for (const el of document.querySelector(selector).querySelectorAll("a, button")) {
          const r = el.getBoundingClientRect();
          // What is hidden, or clipped to nothing for a screen reader, takes no room.
          if (!r.width || !r.height || r.width <= 1) continue;
          const name = el.textContent.trim().replace(/\s+/g, " ").slice(0, 20) || el.getAttribute("aria-label");
          if (r.top < box.top - 0.5 || r.bottom > box.bottom + 0.5) out.push(`${name} is outside the bar vertically (${Math.round(r.top)}-${Math.round(r.bottom)} in ${Math.round(box.top)}-${Math.round(box.bottom)})`);
          if (r.left < -0.5 || r.right > innerWidth + 0.5) out.push(`${name} is outside the window (${Math.round(r.left)}-${Math.round(r.right)} in ${innerWidth})`);
          // A label that wrapped is taller than one line of its text.
          const line = parseFloat(getComputedStyle(el).lineHeight) || parseFloat(getComputedStyle(el).fontSize) * 1.3;
          if (el.matches("nav a") && r.height > line + 24) out.push(`${name} wraps onto ${Math.round(r.height / line)} lines`);
        }
        return out;
      }, bar);
      bad.push(...found.map((f) => `at ${width}px: ${f}`));
    }
    expect(bad).toEqual([]);
  });
}
