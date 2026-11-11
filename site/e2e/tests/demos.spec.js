import { expect, load, test } from "./page.js";

// The small demos beside the features (#357): each plays once as it scrolls
// into the window and ends in its finished state, at once under reduced
// motion; the pointer over one holds it. A demo marks itself data-done when
// it has played to its end.
const demos = [
  { name: "search", tab: "tab-search", done: (el) => el.querySelector(".d-status").textContent.includes("Playing") },
  { name: "queue", tab: "tab-queue", done: (el) => el.querySelectorAll(".d-row").length === 5 },
  { name: "download", tab: "tab-library", done: (el) => el.querySelector("[data-percent]").textContent === "100%" },
  { name: "lyrics", tab: "tab-lyrics", done: (el) => [...el.querySelectorAll("i")].every((w) => w.classList.contains("is-sung")) },
  { name: "theme", tab: null, done: (el) => el.querySelector("[data-name]").textContent === "Walnut theme" },
  { name: "mini", tab: "tab-keyboard", done: (el) => el.querySelector("[data-now]").textContent === "1:28" },
];

const show = async (page, { tab, name }) => {
  if (tab) await page.click(`#${tab}`);
  const demo = page.locator(`[data-demo="${name}"]`);
  await demo.evaluate((el) => el.scrollIntoView({ block: "center" }));
  // A pointer left over the demo, after the tab's click, would hold it.
  await page.mouse.move(0, 0);
  return demo;
};

for (const d of demos) {
  test(`the ${d.name} demo plays to its finished state, with made-up words and songs`, async ({ page }) => {
    await load(page, "/");
    const demo = await show(page, d);
    await expect(demo).toHaveAttribute("data-done", "", { timeout: 25_000 });
    expect(await demo.evaluate(d.done), "the state it ends in").toBe(true);
    // Decoration: hidden from screen readers, and marked for the reduced-motion check.
    await expect(demo).toHaveAttribute("aria-hidden", "true");
    await expect(demo).toHaveAttribute("data-motion", "");
    expect(await demo.boundingBox().then((b) => b.width), "fits its column").toBeLessThanOrEqual(461);
  });
}

test("a demo that is hovered holds, and goes on when the pointer leaves", async ({ page, isMobile }) => {
  test.skip(await page.evaluate(() => matchMedia("(prefers-reduced-motion: reduce)").matches), "under reduced motion a demo is finished at once");
  await load(page, "/");
  const demo = await show(page, demos[0]);
  const typed = demo.locator(".d-typed");
  await expect(typed).not.toHaveText("", { timeout: 10_000 });
  const box = await demo.boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  const held = await typed.textContent();
  await page.waitForTimeout(1500);
  expect(await typed.textContent(), "what it showed while held").toBe(held);
  await page.mouse.move(0, 0);
  await expect(demo).toHaveAttribute("data-done", "", { timeout: 15_000 });
});
