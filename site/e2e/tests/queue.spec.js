import { expect, load, test } from "./page.js";

// Two services, one queue: the rows that slide into one queue fit the
// window under the bar, each cover and its words inside its row and no
// row over another, in a laptop's low window too. A window too low for
// them, as a phone turned sideways, shows the queue as it is without
// script. The checks set their own windows, so one project runs them.
const windows = [
  { width: 390, height: 844, moves: true },
  { width: 844, height: 390, moves: false },
  { width: 1356, height: 315, moves: false },
  { width: 1366, height: 680, moves: true },
  { width: 1280, height: 800, moves: true },
  { width: 1920, height: 950, moves: true },
];

const frame = (page) => page.evaluate(() => new Promise((done) => requestAnimationFrame(() => requestAnimationFrame(done))));

test.describe("the queue", () => {
  test.beforeEach(({}, testInfo) => {
    test.skip(testInfo.project.name !== "chromium-desktop", "the checks set their own windows: chromium-desktop runs them");
  });

  for (const { width, height, moves } of windows) {
    test(`fits a ${width} by ${height} window, ${moves ? "moving" : "as it is"}`, async ({ page }) => {
      await page.setViewportSize({ width, height });
      await load(page, "/");
      // The end of its scroll, where the rows stand in one queue.
      await page.evaluate(() => {
        const merge = document.querySelector("[data-merge]");
        const top = merge.getBoundingClientRect().top + scrollY;
        scrollTo(0, top + Math.max(merge.offsetHeight - (innerHeight - 64), 0));
      });
      await frame(page);
      const got = await page.evaluate(() => {
        const merge = document.querySelector("[data-merge]");
        const rows = [...merge.querySelectorAll(".merge-row")];
        const boxes = rows.map((r) => r.getBoundingClientRect());
        const bad = [];
        boxes.forEach((a, i) => {
          const inside = (r) => r.top >= a.top - 0.5 && r.bottom <= a.bottom + 0.5;
          if (!inside(rows[i].querySelector("img").getBoundingClientRect())) bad.push(`row ${i}'s cover is outside it`);
          const title = rows[i].querySelector(".merge-text strong").getBoundingClientRect();
          if (!inside(title)) bad.push(`row ${i}'s title is outside it`);
          for (let j = i + 1; j < boxes.length; j++) {
            const b = boxes[j];
            if (a.left < b.right && b.left < a.right && a.top < b.bottom - 1 && b.top < a.bottom - 1) bad.push(`rows ${i} and ${j} overlap`);
          }
        });
        return { moves: merge.classList.contains("is-live"), bad };
      });
      expect(got.moves, "whether the queue moves").toBe(moves);
      expect(got.bad).toEqual([]);
    });
  }

  test("turned sideways and back, a phone's queue stops moving, then moves again", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await load(page, "/");
    const moving = async (width, height) => {
      await page.setViewportSize({ width, height });
      await frame(page);
      return page.evaluate(() => document.querySelector("[data-merge]").classList.contains("is-live"));
    };
    expect(await moving(390, 844)).toBe(true);
    expect(await moving(844, 390)).toBe(false);
    expect(await moving(390, 844)).toBe(true);
  });
});
