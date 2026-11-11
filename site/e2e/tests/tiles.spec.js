import { expect, load, test } from "./page.js";

// The feature tiles that act out what they do (#358): their pictures play
// on their own, again and again, while they are in the window, and hold
// where they are while hovered or focused. Under reduced motion nothing
// plays, whatever is hovered or focused.
const tile = (page, heading) => page.locator(".bento .tile", { has: page.locator("h3", { hasText: heading }) });
const reduced = (page) => page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches);
const hoverless = (page) => page.evaluate(() => matchMedia("(hover: none)").matches);
// Longer than any wait between two changes of a picture, as tiles.js
// plays them: a picture that didn't change for that long is held.
const held = 3500;

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

test("a tile plays on its own, again and again, once in the window", async ({ page }) => {
  await load(page, "/");
  const downloads = tile(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  await expect(percent, "out of the window, it waits at rest").toHaveText("40%");
  await downloads.evaluate((el) => el.scrollIntoView({ block: "center" }));
  if (await reduced(page)) {
    await page.waitForTimeout(held);
    await expect(percent, "under reduced motion nothing plays").toHaveText("40%");
    await expect(downloads, "and the tile takes no focus").not.toHaveAttribute("tabindex", /.*/);
    return;
  }
  await expect(percent).toHaveText("100%");
  await expect(percent, "back at rest").toHaveText("40%");
  await expect(percent, "and again").toHaveText("100%");
});

test("hovering a tile holds it where it is, and leaving it lets it go on", async ({ page }) => {
  await load(page, "/");
  test.skip(await reduced(page), "under reduced motion nothing plays");
  test.skip(await hoverless(page), "nothing hovers here");
  const downloads = tile(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  await downloads.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await expect(percent).toHaveText("100%");
  // The page scrolls smoothly: hover waits until the tile is still.
  await downloads.hover();
  const at = await percent.textContent();
  await page.waitForTimeout(held);
  await expect(percent, "hovered, it held").toHaveText(at);
  await page.mouse.move(0, 0);
  await expect(percent, "and goes on once left").not.toHaveText(at);
});

test("the keyboard's focus holds a tile too", async ({ page }) => {
  await load(page, "/");
  test.skip(await reduced(page), "under reduced motion nothing plays");
  const dislikes = tile(page, "Dislikes");
  const songs = dislikes.locator(".ta-song");
  const on = () => songs.evaluateAll((all) => all.findIndex((s) => s.classList.contains("is-on")));
  await dislikes.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await expect(songs.nth(2), "it plays").toHaveClass(/is-on/);
  // A key first, so that the focus that follows is the keyboard's.
  await page.keyboard.press("Shift");
  await dislikes.focus();
  await expect(dislikes).toBeFocused();
  const at = await on();
  await page.waitForTimeout(held);
  expect(await on(), "focused, it held").toBe(at);
  // The focus leaves without scrolling the tile away, as Tab may: out of the
  // window, it would wait.
  await dislikes.evaluate((el) => el.blur());
  await expect.poll(on, { message: "and goes on once the focus left" }).not.toBe(at);
});

test("on a touch screen the tiles play on their own too", async ({ browser, browserName, baseURL }) => {
  test.skip(browserName !== "chromium", "only Chromium takes mobile emulation");
  test.skip(test.info().project.use.reducedMotion === "reduce", "under reduced motion nothing plays");
  const context = await browser.newContext({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 }, baseURL });
  const page = await context.newPage();
  await page.route("https://api.github.com/**", (route) => route.fulfill({ json: { stargazers_count: 1, tag_name: "v0.0.0", assets: [] } }));
  await load(page, "/");
  expect(await hoverless(page), "the emulation has no hover").toBe(true);
  const downloads = tile(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  await downloads.evaluate((el) => el.scrollIntoView({ block: "center" }));
  await expect(percent).toHaveText("100%", { timeout: 6000 });
  await expect(percent, "and again").toHaveText("40%", { timeout: 6000 });
  await context.close();
});
