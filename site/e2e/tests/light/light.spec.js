import { expect, load, test } from "../page.js";

// The light run follows the system's light theme: nothing chose a theme, so
// the page is the light one, which these pages are drawn in.
test("the pages are light when the system is", async ({ page }) => {
  for (const path of ["/", "/docs/"]) {
    await load(page, path);
    expect(await page.evaluate(() => document.documentElement.dataset.theme), "no chosen theme").toBeUndefined();
    const luminance = await page.evaluate(() => {
      const [r, g, b] = getComputedStyle(document.body).backgroundColor.match(/\d+(\.\d+)?/g).map(Number);
      return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
    });
    expect(luminance, `${path}: the page's background`).toBeGreaterThan(0.8);
  }
});
