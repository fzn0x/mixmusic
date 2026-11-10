import { expect, load, test } from "../page.js";
import { moving } from "./moving.js";

// Under reduced motion (#360) nothing moves that the visitor didn't ask to
// move. These runs ask the browser for it (see motion.spec.js).

test("no animation runs on the home page while it rests", async ({ page }) => {
  await load(page, "/");
  const running = await page.evaluate(() =>
    document
      .getAnimations()
      .filter((a) => a.playState === "running" && a.effect?.target)
      .map((a) => {
        const el = a.effect.target;
        return `${el.tagName.toLowerCase()}${typeof el.className === "string" && el.className ? "." + el.className.trim().split(/\s+/).join(".") : ""}: ${a.animationName || a.transitionProperty || "an animation"}`;
      }),
  );
  expect(running, "animations running at rest").toEqual([]);
});

test("the parts of demos and tiles that move, marked data-motion, stay still", async ({ page }) => {
  await load(page, "/");
  expect(await moving(page)).toEqual([]);
});

// The check has to see movement when there is some: a marked part that
// animates itself, in a way no style sheet can switch off, is caught.
test("the check catches a marked part that moves", async ({ page }) => {
  await load(page, "/");
  await page.evaluate(() => {
    const el = document.createElement("div");
    el.dataset.motion = "";
    el.style.cssText = "width: 40px; height: 40px; background: red; margin: 8px";
    el.animate([{ transform: "translateX(0)" }, { transform: "translateX(80px)" }], { duration: 2000, iterations: Infinity, direction: "alternate" });
    document.querySelector("main").prepend(el);
  });
  const found = await moving(page);
  expect(found.length, "what the check reported for a part that moves").toBeGreaterThan(0);
});

// How an element and what it holds sit: their transforms and their places.
const look = (el) =>
  [el, ...el.querySelectorAll("*")]
    .map((e) => {
      const r = e.getBoundingClientRect();
      return `${getComputedStyle(e).transform} ${Math.round((r.top + scrollY) * 10) / 10},${Math.round(r.left * 10) / 10},${Math.round(r.width * 10) / 10}`;
    })
    .join("|");

test("hovering what is on the page does not move it", async ({ page }) => {
  test.setTimeout(120_000);
  await load(page, "/");
  const bad = [];
  for (const selector of [".btn", ".link", ".tile", ".system", ".chip", ".tab", ".play", ".a-round", ".a-btn"]) {
    const count = await page.locator(selector).count();
    for (let i = 0; i < count; i++) {
      const el = page.locator(selector).nth(i);
      if (!(await el.isVisible())) continue;
      await el.evaluate((e) => e.scrollIntoView({ block: "center" }));
      await page.waitForTimeout(50);
      const before = await el.evaluate(look);
      const box = await el.boundingBox();
      await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
      await page.waitForTimeout(120);
      const after = await el.evaluate(look);
      if (before !== after) bad.push(`${selector} #${i}: ${before} became ${after} when hovered`);
      await page.mouse.move(0, 0);
    }
  }
  expect(bad).toEqual([]);
});

test("the hero's player still plays", async ({ page }) => {
  await load(page, "/");
  const play = page.locator(".demo-wrap [data-play]:visible").first();
  await expect(play).toHaveAttribute("aria-label", "Play");
  await play.click();
  await expect(play).toHaveAttribute("aria-label", "Pause");
  await play.click();
  await expect(play).toHaveAttribute("aria-label", "Play");
  // And it still moves on to the next song, where its page has the button.
  const next = page.locator(".demo-wrap [data-next]:visible").first();
  if (await next.count()) {
    const title = page.locator(".demo-wrap [data-title]:visible").first();
    const before = await title.textContent();
    await next.click();
    await expect(title).not.toHaveText(before);
  }
});
