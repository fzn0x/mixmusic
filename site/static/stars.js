// The star count of the GitHub button in the top bar, on the home page and
// the docs: how many have starred MixMusic on GitHub, asked of GitHub's API
// and kept in the browser for an hour, so that the pages ask at most once
// an hour. When GitHub doesn't answer, or limits the requests of the
// visitor's address, the count stays hidden. Its room is kept while it
// loads (stars.css), so that nothing moves when it comes.
(() => {
  const slots = document.querySelectorAll("[data-stars]");
  if (!slots.length) return;
  const REPO = "fzn0x/mixmusic";
  const KEY = "mixmusic:stars";
  const HOUR = 60 * 60 * 1000;

  // 317, 1.2k, 12k: never more than GitHub has.
  const short = (n) => (n < 1000 ? String(n) : n < 10000 ? `${Math.floor(n / 100) / 10}k` : `${Math.floor(n / 1000)}k`);

  const show = (n) => {
    const full = `${n.toLocaleString("en")} ${n === 1 ? "star" : "stars"}`;
    slots.forEach((slot) => {
      const shown = document.createElement("span");
      shown.setAttribute("aria-hidden", "true");
      shown.textContent = short(n);
      // Screen readers hear the whole number after the button's name.
      const spoken = document.createElement("span");
      spoken.className = "stars-sr";
      spoken.textContent = `, ${full}`;
      slot.replaceChildren(shown, spoken);
      slot.title = `${full} on GitHub`;
      slot.classList.add("is-shown");
    });
  };
  const hide = () => slots.forEach((slot) => (slot.hidden = true));

  // Storage can be missing or refused, as in a private window: then the
  // count is asked for on each page.
  let kept = null;
  try {
    kept = JSON.parse(localStorage.getItem(KEY));
  } catch {}
  if (kept && Number.isInteger(kept.count) && Date.now() - kept.at >= 0 && Date.now() - kept.at < HOUR) {
    show(kept.count);
    return;
  }

  fetch(`https://api.github.com/repos/${REPO}`, { headers: { Accept: "application/vnd.github+json" } })
    .then((res) => (res.ok ? res.json() : Promise.reject(new Error(`GitHub answered ${res.status}`))))
    .then((repo) => {
      const count = repo?.stargazers_count;
      if (!Number.isInteger(count) || count < 0) throw new Error("no star count");
      try {
        localStorage.setItem(KEY, JSON.stringify({ count, at: Date.now() }));
      } catch {}
      show(count);
    })
    .catch(hide);
})();
