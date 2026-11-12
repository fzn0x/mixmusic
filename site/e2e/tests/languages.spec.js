import { expect, load, test } from "./page.js";
import { languages } from "./pages.js";

// The landing page's languages besides English, as tools/sitefinish makes
// them: each at /<dir>/, its words translated, its lang and dir, its
// addresses a folder up, the alternates search engines read, and the
// language menus. These run in Chromium alone: pages.js gives Urdu and
// Hindi every check in every browser.
const all = languages.length + 1;

test.describe("languages", () => {
  test.skip(({ browserName }) => browserName !== "chromium", "the languages' pages are checked in Chromium; pages.js checks two in every browser");

  for (const l of languages) {
    test(`the page in ${l.tag} loads, in its language, with the others`, async ({ page, seen }) => {
      await load(page, `/${l.dir}/`);
      expect(seen.console, "console errors").toEqual([]);
      expect(seen.failed, "failed requests").toEqual([]);
      expect(seen.outside, "requests outside the site").toEqual([]);
      const got = await page.evaluate(() => ({
        lang: document.documentElement.lang,
        dir: document.documentElement.dir,
        title: document.title,
        canonical: document.querySelector('link[rel="canonical"]')?.href,
        alternates: [...document.querySelectorAll('link[rel="alternate"][hreflang]')].map((a) => a.hreflang),
        menu: [...document.querySelectorAll(".lang-list a")].map((a) => [a.lang, a.getAttribute("aria-current")]),
        footer: document.querySelectorAll(".footer-langs a").length,
        wide: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      }));
      expect(got.lang).toBe(l.tag);
      expect(got.dir).toBe(l.rtl ? "rtl" : "");
      expect(got.title, "the title is translated").not.toBe("MixMusic: Spotify and YouTube Music in one desktop player");
      expect(got.canonical).toMatch(new RegExp(`/${l.dir}/$`));
      expect(got.alternates).toHaveLength(all + 1);
      expect(got.alternates).toContain("x-default");
      expect(got.menu).toHaveLength(all);
      expect(got.menu.filter(([, current]) => current === "page")).toEqual([[l.tag, "page"]]);
      expect(got.footer).toBe(all);
      expect(got.wide, "the page is wider than the window").toBe(false);
    });
  }

  test("the English page names the others, and its menu opens them", async ({ page }) => {
    await load(page, "/");
    expect(await page.locator('link[rel="alternate"][hreflang]').count()).toBe(all + 1);
    await page.locator(".lang-menu > summary").click();
    await page.locator('.lang-list a[hreflang="pt-BR"]').click();
    await expect(page).toHaveURL(/\/pt-br\/$/);
    expect(await page.evaluate(() => document.documentElement.lang)).toBe("pt-BR");
  });

  // The menu drops from the globe toward the page's middle: leftward on a
  // page read left to right, rightward on one read right to left.
  for (const path of ["/", "/ur/"]) {
    test(`the language menu opens inside the window, on ${path}`, async ({ page }) => {
      await load(page, path);
      for (const width of [390, 820, 1280]) {
        await page.setViewportSize({ width, height: 800 });
        await page.locator(".lang-menu > summary").click();
        const r = await page.locator(".lang-list").boundingBox();
        expect(r, `the menu at ${width}px`).not.toBeNull();
        expect(r.x, `the menu's left edge at ${width}px`).toBeGreaterThanOrEqual(0);
        expect(r.x + r.width, `the menu's right edge at ${width}px`).toBeLessThanOrEqual(width);
        await page.locator(".lang-menu > summary").click();
        await expect(page.locator(".lang-list")).toBeHidden();
      }
    });
  }

  test("the sitemap lists every language's page, and llms.txt the docs", async ({ request }) => {
    const sitemap = await (await request.get("/sitemap.xml")).text();
    for (const l of languages) expect(sitemap).toContain(`/${l.dir}/</loc>`);
    expect(sitemap).toContain('hreflang="x-default"');
    const llms = await request.get("/llms.txt");
    expect(llms.status()).toBe(200);
    const text = await llms.text();
    expect(text.startsWith("# MixMusic\n\n> ")).toBe(true);
    expect(text).toContain("/docs/getting-started/");
    expect((await request.get("/llms-full.txt")).status()).toBe(200);
  });
});
