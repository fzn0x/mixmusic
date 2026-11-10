package overlay

// overlayPage is the page of /overlay, which a browser source shows: the
// cover, the title, the artists and a progress bar that moves between the
// events. It asks nothing of the server but the events and the cover, and
// puts no text in the page but through textContent.
const overlayPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MixMusic now playing</title>
<style>
  html, body { margin: 0; background: transparent; overflow: hidden; }
  body { font-family: "Segoe UI", system-ui, -apple-system, "Helvetica Neue", sans-serif; color: #fff; }
  #card { display: flex; align-items: center; gap: 16px; box-sizing: border-box; width: 480px; padding: 12px;
    background: rgba(18, 18, 22, 0.78); border-radius: 14px; transition: opacity 0.4s; }
  #card.idle { opacity: 0; }
  #cover { width: 88px; height: 88px; flex: none; border-radius: 8px; object-fit: cover; background: rgba(255, 255, 255, 0.08); }
  #cover.none { visibility: hidden; }
  #text { flex: 1; min-width: 0; }
  #title { font-size: 22px; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  #artist { font-size: 16px; opacity: 0.75; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-top: 2px; }
  #bar { height: 5px; margin-top: 12px; border-radius: 3px; background: rgba(255, 255, 255, 0.2); overflow: hidden; }
  #fill { height: 100%; width: 0; background: #fff; }
  #bar.live { display: none; }
</style>
</head>
<body>
<div id="card" class="idle">
  <img id="cover" class="none" alt="">
  <div id="text">
    <div id="title"></div>
    <div id="artist"></div>
    <div id="bar"><div id="fill"></div></div>
  </div>
</div>
<script>
(function () {
  var card = document.getElementById("card"), cover = document.getElementById("cover");
  var title = document.getElementById("title"), artist = document.getElementById("artist");
  var bar = document.getElementById("bar"), fill = document.getElementById("fill");
  var state = null, told = 0, lastCover = "";

  function show(s) {
    state = s;
    told = performance.now();
    card.className = s.active ? "" : "idle";
    title.textContent = s.title;
    artist.textContent = s.artist;
    bar.className = s.live || !s.durationMs ? "live" : "";
    if (s.cover !== lastCover) {
      lastCover = s.cover;
      if (s.cover) { cover.src = s.cover; cover.className = ""; } else { cover.removeAttribute("src"); cover.className = "none"; }
    }
  }

  // The bar moves between the events, from the position and the speed of
  // the last one; each event puts it right.
  function draw() {
    if (state && state.durationMs > 0) {
      var pos = state.positionMs;
      if (state.playing && !state.live) pos += (performance.now() - told) * state.speed;
      fill.style.width = Math.max(0, Math.min(1, pos / state.durationMs)) * 100 + "%";
    }
    requestAnimationFrame(draw);
  }

  function connect() {
    var source = new EventSource("events");
    source.addEventListener("now-playing", function (e) { show(JSON.parse(e.data)); });
    source.onerror = function () { source.close(); setTimeout(connect, 3000); };
  }
  connect();
  draw();
})();
</script>
</body>
</html>
`
