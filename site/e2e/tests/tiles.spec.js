import AxeBuilder from "@axe-core/playwright";
import { expect, load, test } from "./page.js";

// The feature cards that act out what they do (#358): the tiles of "New in"
// and the plugin card of "Make it yours". Their pictures play on their own,
// again and again, while they are well in the window, go back to their
// start out of it, and hold where they are while hovered or focused. Under
// reduced motion each shows how it ends, and nothing plays.
const card = (page, heading) => page.locator(".bento .tile, .columns .column", { has: page.locator("h3", { hasText: heading }) });
const reduced = (page) => page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches);
const hoverless = (page) => page.evaluate(() => matchMedia("(hover: none)").matches);
// center puts a card in the middle of the window at once: the page scrolls
// smoothly, which takes WebKit a while from far away.
const center = (loc) => loc.evaluate((el) => el.scrollIntoView({ block: "center", behavior: "instant" }));
// Longer than any wait between two changes of a picture, as tiles.js
// plays them: a picture that didn't change for that long is held.
const held = 3500;
// How long a round of Downloads may take: WebKit on a busy runner keeps
// the steps' timers late, and a card starts after those that came in with
// it.
const round = { timeout: 10_000 };
const headings = [
  "Lyrics in your language",
  "Podcasts",
  "Downloads",
  "Layouts",
  "5.1 surround",
  "Casting",
  "A player around the notch",
  "Speed",
  "Global shortcuts",
  "Dislikes",
  "Live streams and music videos",
  "No ads",
  "Add a plugin",
];

test("each card has one picture, hidden from assistive technology and marked for the reduced-motion check", async ({ page }) => {
  await load(page, "/");
  for (const heading of headings) {
    const art = card(page, heading).locator(".tile-act");
    await expect(art, `${heading}'s picture`).toHaveCount(1);
    await expect(art).toHaveAttribute("aria-hidden", "true");
    await expect(art).toHaveAttribute("data-motion", "");
  }
});

test("a card plays on its own, again and again, once in the window, and is back at its start out of it", async ({ page }) => {
  await load(page, "/");
  const downloads = card(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  if (await reduced(page)) {
    await center(downloads);
    await page.waitForTimeout(held);
    await expect(percent, "under reduced motion it shows how it ends").toHaveText("100%");
    await expect(downloads, "and the card takes no focus").not.toHaveAttribute("tabindex", /.*/);
    return;
  }
  await expect(percent, "out of the window, it waits at its start").toHaveText("40%");
  await center(downloads);
  await expect(percent).toHaveText("100%", round);
  await expect(percent, "back at its start").toHaveText("40%", round);
  await expect(percent, "and again").toHaveText("100%", round);
  await page.evaluate(() => scrollTo({ top: 0, behavior: "instant" }));
  await expect(percent, "out of the window, back at its start").toHaveText("40%", { timeout: 3000 });
});

test("every card's picture moves on once in the window", async ({ page }) => {
  test.setTimeout(120_000);
  await load(page, "/");
  test.skip(await reduced(page), "under reduced motion nothing plays");
  for (const heading of headings) {
    const art = card(page, heading).locator(".tile-act");
    await center(card(page, heading));
    const first = await art.innerHTML();
    await expect.poll(() => art.innerHTML(), { message: `${heading}'s picture moves on`, timeout: 8000 }).not.toBe(first);
  }
});

test("hovering a card holds it where it is, and leaving it lets it go on", async ({ page }) => {
  await load(page, "/");
  test.skip(await reduced(page), "under reduced motion nothing plays");
  test.skip(await hoverless(page), "nothing hovers here");
  const downloads = card(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  await center(downloads);
  await expect(percent).toHaveText("100%", round);
  await downloads.hover();
  const at = await percent.textContent();
  await page.waitForTimeout(held);
  await expect(percent, "hovered, it held").toHaveText(at);
  await page.mouse.move(0, 0);
  await expect(percent, "and goes on once left").not.toHaveText(at, round);
});

test("the keyboard's focus holds a card too", async ({ page }) => {
  await load(page, "/");
  test.skip(await reduced(page), "under reduced motion nothing plays");
  const dislikes = card(page, "Dislikes");
  const queue = dislikes.locator(".ta-qwrap");
  const at = () => queue.getAttribute("data-at");
  await center(dislikes);
  await expect(queue, "it plays: the queue passes over the song disliked").toHaveAttribute("data-at", "2", round);
  // A key first, so that the focus that follows is the keyboard's.
  await page.keyboard.press("Shift");
  await dislikes.focus();
  await expect(dislikes).toBeFocused();
  const was = await at();
  await page.waitForTimeout(held);
  expect(await at(), "focused, it held").toBe(was);
  // The focus leaves without scrolling the card away, as Tab may: out of the
  // window, it would go back to its start.
  await dislikes.evaluate((el) => el.blur());
  await expect.poll(at, { message: "and goes on once the focus left", ...round }).not.toBe(was);
});

test("the shortcuts card's keys can be read in either scheme", async ({ page }) => {
  // The card stays dark in the light scheme: its keys must too, not take
  // the page's light key under the card's white letters. It is read at
  // rest, as reduced motion shows it: playing, it fades from one picture to
  // the next, whose colours halfway axe would read.
  await page.emulateMedia({ reducedMotion: "reduce" });
  await load(page, "/");
  const shortcuts = card(page, "Global shortcuts");
  await center(shortcuts);
  await expect(shortcuts.locator("kbd").first()).toBeVisible(round);
  const result = await new AxeBuilder({ page }).include(".ta-keysact").withRules(["color-contrast"]).analyze();
  expect(result.violations.flatMap((v) => v.nodes.map((n) => `${n.target.join(" ")}: ${n.any[0]?.message}`))).toEqual([]);
});

test("on a touch screen the cards play on their own too", async ({ browser, browserName, baseURL }) => {
  test.skip(browserName !== "chromium", "only Chromium takes mobile emulation");
  test.skip(test.info().project.use.reducedMotion === "reduce", "under reduced motion nothing plays");
  const context = await browser.newContext({ hasTouch: true, isMobile: true, viewport: { width: 390, height: 844 }, baseURL });
  const page = await context.newPage();
  await page.route("https://api.github.com/**", (route) => route.fulfill({ json: { stargazers_count: 1, tag_name: "v0.0.0", assets: [] } }));
  await load(page, "/");
  expect(await hoverless(page), "the emulation has no hover").toBe(true);
  const downloads = card(page, "Downloads");
  const percent = downloads.locator("[data-percent]");
  await center(downloads);
  await expect(percent).toHaveText("100%", round);
  await expect(percent, "and again").toHaveText("40%", round);
  await context.close();
});
