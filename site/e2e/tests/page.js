import { expect, test as base } from "@playwright/test";

// The one outside address the site asks for, the star count and the latest
// release on GitHub, is answered here, so that no check calls a live service.
const github = (route) => {
  const url = route.request().url();
  if (url.endsWith("/releases/latest")) {
    return route.fulfill({ json: { tag_name: "v0.0.0", html_url: "https://github.com/fzn0x/mixmusic/releases/tag/v0.0.0", assets: [] } });
  }
  return route.fulfill({ json: { stargazers_count: 1234 } });
};

// test is Playwright's, with `seen` added: what the page did wrong while it
// loaded, which a test reads after loading the page with `load`.
export const test = base.extend({
  seen: async ({ page, baseURL }, use) => {
    const seen = { console: [], failed: [], outside: [] };
    await page.route("https://api.github.com/**", github);
    page.on("console", (m) => {
      if (m.type() === "error") seen.console.push(`${m.text()} (${m.location().url})`);
    });
    page.on("pageerror", (e) => seen.console.push(`uncaught: ${e.message}`));
    page.on("requestfailed", (r) => {
      // A browser drops the video it began to fetch once it has what it
      // needs (Chromium says aborted, Firefox says parsed data cached),
      // which is not a failure. A video the server refuses still fails, in
      // the status check below.
      if (r.resourceType() === "media") return;
      seen.failed.push(`${r.method()} ${r.url()}: ${r.failure()?.errorText}`);
    });
    page.on("response", (r) => {
      if (r.status() >= 400) seen.failed.push(`${r.request().method()} ${r.url()}: ${r.status()}`);
    });
    page.on("request", (r) => {
      const url = new URL(r.url());
      if (!r.url().startsWith(baseURL) && !/^(data|blob|about):/.test(r.url()) && url.hostname !== "api.github.com") seen.outside.push(r.url());
    });
    await use(seen);
  },
});

// load opens a page and waits until it is quiet: loaded, its fonts ready,
// and every image fetched, which is scrolled through for the lazy ones. It
// ends back at the top.
export const load = async (page, path) => {
  const response = await page.goto(path, { waitUntil: "load" });
  expect(response.status(), `${path} answered`).toBe(200);
  await page.evaluate(() => document.fonts.ready);
  await page.evaluate(async () => {
    const step = Math.max(200, innerHeight / 2);
    for (let y = 0; y < document.documentElement.scrollHeight; y += step) {
      scrollTo(0, y);
      await new Promise((r) => setTimeout(r, 40));
    }
    scrollTo(0, 0);
  });
  await page.waitForLoadState("networkidle");
  // Whatever eases in has finished.
  await page.waitForTimeout(600);
};

export { expect };
