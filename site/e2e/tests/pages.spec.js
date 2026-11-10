import AxeBuilder from "@axe-core/playwright";
import { expect, load, test } from "./page.js";
import { pages } from "./pages.js";

// Rules of axe that are turned off, each with its reason. None for now.
const off = [];

for (const { name, path } of pages) {
  test.describe(name, () => {
    test("loads without console errors, failed requests or outside calls", async ({ page, seen }) => {
      await load(page, path);
      expect(seen.console, "console errors").toEqual([]);
      expect(seen.failed, "failed requests").toEqual([]);
      expect(seen.outside, "requests to addresses outside the site").toEqual([]);
    });

    test("does not scroll sideways", async ({ page }) => {
      await load(page, path);
      const wide = await page.evaluate(() => {
        const root = document.documentElement;
        const out = [];
        // What reaches past the window's right edge, to name the culprit.
        for (const el of document.body.querySelectorAll("*")) {
          const r = el.getBoundingClientRect();
          if (r.width && r.right > root.clientWidth + 1 && getComputedStyle(el).position !== "fixed") {
            out.push(`${el.tagName.toLowerCase()}${el.className && typeof el.className === "string" ? "." + el.className.trim().split(/\s+/).join(".") : ""} reaches ${Math.round(r.right)}`);
          }
        }
        return { scroll: root.scrollWidth, client: root.clientWidth, out: out.slice(0, 8) };
      });
      expect(wide.scroll, `the page is wider than the window (${wide.out.join("; ")})`).toBeLessThanOrEqual(wide.client);
    });

    test("every image has an alt and loads", async ({ page }) => {
      await load(page, path);
      const bad = await page.evaluate(async () => {
        // Lazy images start to load as they near the window: each is brought
        // near, and given a while.
        const out = [];
        for (const img of document.images) {
          if (!img.hasAttribute("alt")) out.push(`no alt: ${img.currentSrc || img.src}`);
          img.scrollIntoView({ block: "center" });
          const done = () => img.complete && img.naturalWidth > 0;
          for (let i = 0; i < 100 && !done(); i++) await new Promise((r) => setTimeout(r, 50));
          if (!done()) out.push(`not loaded: ${img.currentSrc || img.src}`);
        }
        return out;
      });
      expect(bad).toEqual([]);
    });

    test("has no accessibility violations", async ({ page }) => {
      await load(page, path);
      const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"]).disableRules(off).analyze();
      const found = result.violations.map((v) => `${v.id} (${v.impact}): ${v.help}\n    ${v.nodes.slice(0, 4).map((n) => n.target.join(" ")).join("\n    ")}`);
      expect(found, "axe violations").toEqual([]);
    });
  });
}
