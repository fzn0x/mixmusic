import { expect, load, test } from "./page.js";

// The feature tiles that act out what they do (#358): hovered or focused,
// their picture plays; at rest it is as it was. Under reduced motion
// nothing plays, whatever is hovered or focused.
const tile = (page, heading) => page.locator(".bento .tile", { has: page.locator("h3", { hasText: heading }) });
const reduced = (page) => page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches);

test("the tiles that play have a hidden picture marked for the reduced-motion check", async ({ page }) => {
  await load(page, "/");
  for (const heading of ["Podcasts", "Downloads", "Dislikes", "Global shortcuts"]) {
    const art = tile(page, heading).locator(".tile-act");
    await expect(art, `${heading}'s picture`).toHaveAttribute("aria-hidden", "true");
    await expect(art).toHaveAttribute("data-motion", "");
  }
  for (const heading of ["Lyrics in your language", "Speed"]) {
    await expect(tile(page, heading).locator("[data-motion]"), `${heading}'s picture`).toHaveCount(1);
  }
});

test("hovering a tile plays it, and leaving it puts the picture back", async ({ page }) => {
  await load(page, "/");
  const downloads = tile(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  await downloads.evaluate((el) => el.scrollIntoView({ block: "center" }));
  // Where nothing hovers (a browser emulating touch), the tile played by itself.
  if (await page.evaluate(() => matchMedia("(hover: none)").matches) && !(await reduced(page))) {
    await expect(percent).toHaveText("100%");
    return;
  }
  await expect(percent).toHaveText("40%");
  // The page scrolls smoothly: hover waits until the tile is still.
  await downloads.hover();
  if (await reduced(page)) {
    await page.waitForTimeout(1200);
    await expect(percent, "under reduced motion hovering changes nothing").toHaveText("40%");
    await expect(downloads, "and the tile takes no focus").not.toHaveAttribute("tabindex", /.*/);
    return;
  }
  await expect(percent).toHaveText("100%");
  await page.mouse.move(0, 0);
  await expect(percent).toHaveText("40%");
});

test("a tile plays for the keyboard's focus too", async ({ page }) => {
  await load(page, "/");
  if (await reduced(page)) return;
  const dislikes = tile(page, "Dislikes");
  await dislikes.evaluate((el) => el.scrollIntoView({ block: "center" }));
  // A key first, so that the focus that follows is the keyboard's.
  await page.keyboard.press("Shift");
  await dislikes.focus();
  await expect(dislikes).toBeFocused();
  await expect(dislikes.locator(".ta-song").nth(2), "the selection stepped down").toHaveClass(/is-on/);
  await page.keyboard.press("Tab");
  await expect(dislikes.locator(".ta-song").nth(0), "and went back to rest").toHaveClass(/is-on/);
});

test("on a touch screen each tile plays once on its own as it scrolls in", async ({ browser, browserName, baseURL }) => {
  test.skip(browserName !== "chromium", "only Chromium takes mobile emulation");
  test.skip(test.info().project.use.reducedMotion === "reduce", "under reduced motion nothing plays");
  const context = await browser.newContext({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 }, baseURL });
  const page = await context.newPage();
  await page.route("https://api.github.com/**", (route) => route.fulfill({ json: { stargazers_count: 1, tag_name: "v0.0.0", assets: [] } }));
  await load(page, "/");
  expect(await page.evaluate(() => matchMedia("(hover: none)").matches), "the emulation has no hover").toBe(true);
  const downloads = tile(page, "Downloads");
  await downloads.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await expect(downloads.locator("[data-percent]")).toHaveText("100%", { timeout: 6000 });
  await context.close();
});
