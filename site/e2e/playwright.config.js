// The checks of the built website (#366), run by .github/workflows/site-pages.yml
// and, on a computer with the site built, with `npm test` in this folder:
//   zola --root site build --base-url http://127.0.0.1:4173
//
// Every page is loaded in Chromium, Firefox and WebKit, at a phone's width
// and a desktop's, in the dark theme. Two more runs, in Chromium, use the
// light theme and reduced motion at both widths.
//
// Tests in tests/ run in every project. Tests in tests/motion/ run only in
// the reduced-motion projects: a check that nothing moves under reduced
// motion (#360) goes there, in a file of its own. Tests in tests/light/ run
// only in the light projects.
import { defineConfig, devices } from "@playwright/test";

const PORT = 4173;

const sizes = {
  phone: { viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: false },
  desktop: { viewport: { width: 1280, height: 800 } },
};

// WebKit and Firefox take their own engines' defaults for the rest.
const browsers = { chromium: devices["Desktop Chrome"], firefox: devices["Desktop Firefox"], webkit: devices["Desktop Safari"] };

const project = (name, browser, size, use = {}, files = {}) => ({
  name,
  use: { ...browsers[browser], ...sizes[size], ...use },
  ...files,
});

const own = { testIgnore: ["**/motion/**", "**/light/**"] };

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  workers: process.env.CI ? 2 : undefined,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: { baseURL: `http://127.0.0.1:${PORT}`, colorScheme: "dark", reducedMotion: "no-preference", locale: "en-US" },
  projects: [
    ...["chromium", "firefox", "webkit"].flatMap((b) => ["phone", "desktop"].map((s) => project(`${b}-${s}`, b, s, {}, own))),
    ...["phone", "desktop"].map((s) => project(`light-${s}`, "chromium", s, { colorScheme: "light" }, { testIgnore: ["**/motion/**"] })),
    ...["phone", "desktop"].map((s) => project(`reduced-motion-${s}`, "chromium", s, { reducedMotion: "reduce" }, { testIgnore: ["**/light/**"] })),
  ],
  webServer: {
    command: "node serve.js",
    url: `http://127.0.0.1:${PORT}/`,
    reuseExistingServer: !process.env.CI,
    env: { PORT: String(PORT) },
  },
});
