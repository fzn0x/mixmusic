import AxeBuilder from "@axe-core/playwright";
import { expect, load, test } from "./page.js";

// The docs' search (#932): a field at the top of the docs' drawer, which a
// narrow window opens from the top bar's search button. Its results are
// the docs' sections that hold the words typed, and one opens at its
// heading.

// field puts the cursor in the search, from the top bar's button where the
// drawer is closed, and returns it.
const field = async (page) => {
  const opener = page.getByRole("button", { name: "Search the docs" });
  if (await opener.isVisible()) await opener.click();
  const input = page.getByRole("combobox", { name: "Search the docs" });
  await expect(input).toBeVisible();
  await input.click();
  return input;
};

test("the docs' search finds a section and opens the page at its heading", async ({ page }) => {
  await load(page, "/docs/");
  const input = await field(page);
  await input.fill("crossfade gapless");
  const first = page.getByRole("option").first();
  await expect(first).toContainText("Crossfade and gapless playback");
  await expect(first.locator("mark").first()).toBeVisible();
  await input.press("ArrowDown");
  await expect(first).toHaveAttribute("aria-selected", "true");
  await expect(input).toHaveAttribute("aria-activedescendant", await first.getAttribute("id"));
  await input.press("Enter");
  await expect(page).toHaveURL(/\/docs\/playing-music\/#crossfade-and-gapless-playback$/);
  await expect(page.locator("#crossfade-and-gapless-playback")).toBeInViewport();
});

test("the docs' search says when nothing holds the words", async ({ page }) => {
  await load(page, "/docs/getting-started/");
  const input = await field(page);
  await input.fill("qqzzxx");
  // Shown under the field, and read out by the status, which is hidden.
  await expect(page.locator(".search-note")).toHaveText("Nothing in the docs holds “qqzzxx”.");
  await expect(page.locator(".search-note")).toBeVisible();
  await expect(page.locator("[data-search-status]")).toHaveText("Nothing in the docs holds “qqzzxx”.");
  await expect(page.getByRole("option")).toHaveCount(0);
  // Escape closes the results, then empties the field.
  await input.press("Escape");
  await expect(input).toHaveAttribute("aria-expanded", "false");
  await input.press("Escape");
  await expect(input).toHaveValue("");
});

test("/ puts the cursor in the docs' search", async ({ page }) => {
  await load(page, "/docs/settings/");
  await page.keyboard.press("/");
  await expect(page.getByRole("combobox", { name: "Search the docs" })).toBeFocused();
});

test("the docs' search has no accessibility violations with its results open", async ({ page }) => {
  await load(page, "/docs/");
  const input = await field(page);
  await input.fill("lyrics");
  await expect(page.getByRole("option").first()).toBeVisible();
  await input.press("ArrowDown");
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa", "best-practice"]).analyze();
  const found = result.violations.map((v) => `${v.id} (${v.impact}): ${v.help}\n    ${v.nodes.slice(0, 4).map((n) => n.target.join(" ")).join("\n    ")}`);
  expect(found, "axe violations").toEqual([]);
});
