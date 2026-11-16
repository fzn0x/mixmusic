import { expect, load, test } from "./page.js";

// The page that opens the https form of an invite link to a room (#562):
// what follows its # is the room's code, a dot, and what follows the # of
// the app's link.
const key = "0123456789abcdef".repeat(4);
const secret = "AbCdEfGhIjKlMnOpQrStUv";
const rest = `${key}.${secret}@203.0.113.7:41234,%5B2001:db8::5%5D:41234`;

test.describe("join a room", () => {
  test("opens the app's link of a whole invite", async ({ page, seen }) => {
    await load(page, `/join/#ABC123.${rest}`);
    const open = page.getByRole("link", { name: "Open in MixMusic" });
    await expect(open).toBeVisible();
    await expect(open).toHaveAttribute("href", `mixmusic://join/ABC123#${rest}`);
    await expect(page.getByText("This invite link isn't whole", { exact: false })).toBeHidden();
    expect(seen.console, "console errors").toEqual([]);
  });

  test("tells of an invite that isn't whole", async ({ page }) => {
    await load(page, "/join/#ABC123.nothex");
    await expect(page.getByText("This invite link isn't whole", { exact: false })).toBeVisible();
    await expect(page.getByRole("link", { name: "Open in MixMusic" })).toBeHidden();
    await expect(page.getByRole("link", { name: "Download MixMusic" })).toBeVisible();
  });
});
