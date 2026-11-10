import { expect, load, test } from "../page.js";

// Reduced motion: these runs ask the browser for it. A check that nothing
// moves under it (#360) goes in a file of its own beside this one.
test("the browser asks for reduced motion", async ({ page }) => {
  await load(page, "/");
  expect(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches)).toBe(true);
});
