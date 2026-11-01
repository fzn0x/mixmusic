// The picture of a music video, which MixMusic's now playing page shows in
// place of the cover, muted: public/video.html loads this in a web view
// of the main window. MixMusic plays the sound in its own engine, and the
// Go side (video.go, in the root package) tells this page the engine's
// time a few times a second, which the picture follows, nudging its rate,
// or seeking when it is far off.
//
// The picture comes through Media Source Extensions, a part of the stream
// at a time, from MixMusic's proxy at the stream scheme: a <video> can't
// play the stream scheme itself, as WebView2 gets a scheme's responses
// only once they are whole, and WebKitGTK plays no media of a custom
// scheme. The page holds about AHEAD seconds of the picture ahead of what
// shows, and fetches nothing while the sound is paused or the page isn't
// shown.
//
// Without a document, as in MixMusic's tests, it only defines mvLogic: the
// parts of it that decide, which take no browser.
(function (root) {
  "use strict";

  /** AHEAD is how many seconds of the picture the page holds ahead of what shows. */
  const AHEAD = 30;
  /** BEHIND is how many seconds it keeps behind what shows, for a seek back. */
  const BEHIND = 30;
  /** JUMP is how far, in seconds, the picture may be from the sound before it seeks. */
  const JUMP = 0.3;
  /** NEAR is how close, in seconds, the picture must be to the sound to play at its own rate. */
  const NEAR = 0.025;
  /** TRIES is how many times a part is fetched before the picture gives up. */
  const TRIES = 3;
  /** DRIFT_WINDOW is how many seconds of the drift drift() tells of. */
  const DRIFT_WINDOW = 5;

  /** covered tells whether ranges, [start, end] pairs in seconds, hold time t. */
  function covered(ranges, t) {
    for (const [start, end] of ranges) {
      if (t >= start && t < end) return true;
    }
    return false;
  }

  /**
   * nextPart returns the index of the next of parts, [time, duration, first
   * byte, last byte], to fetch: the first of those that play from time t
   * on, up to ahead seconds after it, that ranges don't hold, or -1. The
   * part playing at t counts as held when t is; the others, when their
   * middle is. skip, when given, leaves a part out.
   */
  function nextPart(parts, ranges, t, ahead, skip) {
    for (let i = 0; i < parts.length; i++) {
      const time = parts[i][0];
      const end = time + parts[i][1];
      if (end <= t) continue;
      if (time > t + ahead) break;
      if (skip && skip(i)) continue;
      const probe = Math.min(Math.max(time + (end - time) / 2, t), end - 0.001);
      if (!covered(ranges, probe)) return i;
    }
    return -1;
  }

  /**
   * target returns where the picture is to be at now, in milliseconds since
   * the epoch, of clock: the sound's time, at, in seconds, which the Go
   * side read at sentAt, in milliseconds since the epoch, and moving on
   * from there while it plays.
   */
  function target(clock, now) {
    if (!clock.playing) return clock.at;
    return clock.at + Math.max(0, now - clock.sentAt) / 1000;
  }

  /**
   * correction returns how the picture closes the gap d, the seconds it
   * runs ahead of the sound, behind when negative: it seeks when far off,
   * else plays a little slower or faster, up to 8%, to catch up in about
   * two seconds.
   */
  function correction(d) {
    const off = Math.abs(d);
    if (off > JUMP) return { seek: true, rate: 1 };
    if (off > NEAR) return { seek: false, rate: Math.min(1.08, Math.max(0.92, 1 - d / 2)) };
    return { seek: false, rate: 1 };
  }

  /**
   * stale returns the time before which the page drops what it holds,
   * ranges, at time t: what lies more than behind seconds back, once a
   * second of it does; 0 when not.
   */
  function stale(ranges, t, behind) {
    const before = t - behind;
    return ranges.length > 0 && ranges[0][0] < before - 1 ? before : 0;
  }

  const logic = { covered, nextPart, target, correction, stale, AHEAD, BEHIND, JUMP, NEAR };
  if (typeof document === "undefined") {
    root.mvLogic = logic;
    return;
  }

  const video = document.querySelector("video");
  video.muted = true;
  /**
   * cur is the picture the page plays: desc, as the Go side gives it
   * (src, mime, init, segments, duration), its MediaSource and
   * SourceBuffer, ready once that took the initialization segment, and
   * the fetch under way.
   */
  let cur = null;
  /** clock is the sound's time, as the last sync told it. */
  let clock = { at: 0, sentAt: 0, playing: false };
  /** active is set while the Go side shows the picture. */
  let active = false;
  let pumping = false;
  let pumpAgain = false;
  /** tries counts the fetches of each part of cur since the last seek. */
  let tries = new Map();
  /** drifts are the drift of the frames shown lately, [when, |drift|]. */
  let drifts = [];

  const now = () => performance.timeOrigin + performance.now();
  const wanted = () => cur !== null && cur.ready && !cur.failed && active && clock.playing && !document.hidden;
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  /** buffered returns what the SourceBuffer of c holds, as [start, end] pairs. */
  function buffered(c) {
    const out = [];
    if (!c.sb) return out;
    const b = c.sb.buffered;
    for (let i = 0; i < b.length; i++) out.push([b.start(i), b.end(i)]);
    return out;
  }

  /** settled resolves once the SourceBuffer of c is done with what it does. */
  function settled(c) {
    return new Promise((resolve, reject) => {
      if (!c.sb.updating) return resolve();
      const done = () => {
        c.sb.removeEventListener("updateend", done);
        c.sb.removeEventListener("error", failed);
        resolve();
      };
      const failed = () => {
        c.sb.removeEventListener("updateend", done);
        c.sb.removeEventListener("error", failed);
        reject(new Error("the picture's data doesn't decode"));
      };
      c.sb.addEventListener("updateend", done);
      c.sb.addEventListener("error", failed);
    });
  }

  /** get fetches bytes from to to of the stream of c. */
  async function get(c, from, to) {
    c.fetching = new AbortController();
    try {
      const res = await fetch(c.desc.src + "&range=" + from + "-" + to, { signal: c.fetching.signal, cache: "no-store" });
      if (!res.ok) throw new Error("MixMusic's proxy answered HTTP " + res.status);
      return await res.arrayBuffer();
    } finally {
      c.fetching = null;
    }
  }

  /** append appends data to the SourceBuffer of c, once it is free. */
  async function append(c, data) {
    await settled(c);
    if (cur !== c) return;
    c.sb.appendBuffer(data);
    await settled(c);
  }

  /** drop drops what the SourceBuffer of c holds before time t. */
  async function drop(c, t) {
    await settled(c);
    if (cur !== c) return;
    c.sb.remove(0, t);
    await settled(c);
  }

  /** stop gives up the fetch of c under way, if any. */
  function stop(c) {
    if (c && c.fetching) c.fetching.abort();
  }

  /** load starts playing the picture desc tells of, in place of cur's. */
  function load(desc) {
    unload();
    const ms = new MediaSource();
    const c = { desc, ms, sb: null, ready: false, fetching: null, failed: "", shown: false };
    cur = c;
    tries = new Map();
    drifts = [];
    video.playbackRate = 1;
    video.src = URL.createObjectURL(ms);
    (async () => {
      await new Promise((resolve) => ms.addEventListener("sourceopen", resolve, { once: true }));
      URL.revokeObjectURL(video.src);
      if (cur !== c) return;
      ms.duration = desc.duration;
      c.sb = ms.addSourceBuffer(desc.mime);
      const init = await get(c, 0, desc.init - 1);
      if (cur !== c) return;
      await append(c, init);
      c.ready = true;
      hold();
    })().catch((e) => fail(c, e));
  }

  /** unload stops playing cur, and drops it. */
  function unload() {
    const c = cur;
    if (!c) return;
    cur = null;
    stop(c);
    video.pause();
    video.removeAttribute("src");
    video.load();
  }

  /** fail gives up the picture of c, telling the Go side why when it asks. */
  function fail(c, e) {
    if (cur !== c || (e && e.name === "AbortError")) return;
    c.failed = (e && e.message) || "the picture couldn't play";
    stop(c);
    video.pause();
    console.error("mixmusic: the picture:", c.failed);
  }

  /**
   * pump fetches the parts of c that play next, while the picture plays:
   * one at a time, about AHEAD seconds ahead of what shows, dropping what
   * lies more than BEHIND seconds back.
   */
  async function pump(c) {
    if (pumping) {
      pumpAgain = true;
      return;
    }
    if (cur !== c || !c.ready) return;
    pumping = true;
    try {
      while (cur === c && wanted()) {
        const t = video.currentTime;
        const before = stale(buffered(c), t, BEHIND);
        if (before > 0) {
          await drop(c, before);
          continue;
        }
        const i = nextPart(c.desc.segments, buffered(c), t, AHEAD, (i) => (tries.get(i) || 0) >= TRIES);
        if (i < 0) break;
        const n = (tries.get(i) || 0) + 1;
        tries.set(i, n);
        const [, , from, to] = c.desc.segments[i];
        try {
          const data = await get(c, from, to);
          if (cur !== c) break;
          await append(c, data);
        } catch (e) {
          if (e.name === "AbortError" || cur !== c) break;
          if (e.name === "QuotaExceededError") {
            // Full: drop more of what lies behind, and try again.
            await drop(c, Math.max(0, video.currentTime - 2));
            tries.set(i, n - 1);
            continue;
          }
          if (n >= TRIES) throw e;
          await sleep(1000 * n);
        }
      }
    } catch (e) {
      fail(c, e);
    } finally {
      pumping = false;
      if (pumpAgain) {
        pumpAgain = false;
        if (cur) setTimeout(() => pump(cur), 0);
      }
    }
  }

  /** seek moves the picture to t, giving up a fetch for elsewhere. */
  function seek(t) {
    if (!cur) return;
    stop(cur);
    tries = new Map();
    video.currentTime = Math.max(0, t);
  }

  /** hold plays the picture, or pauses it at the sound's time, as clock says. */
  function hold() {
    if (!cur || !cur.ready) return;
    if (!wanted()) {
      stop(cur);
      if (!video.paused) video.pause();
      // A paused sound that moved, as by a seek, moves the picture there,
      // if it holds that time: nothing is fetched while it's paused.
      if (!clock.playing && Math.abs(video.currentTime - clock.at) > 2 * NEAR && covered(buffered(cur), clock.at)) {
        video.currentTime = clock.at;
      }
      return;
    }
    const t = target(clock, now());
    if (Math.abs(video.currentTime - t) > JUMP) seek(t);
    if (video.paused) video.play().catch(() => {});
    pump(cur);
  }

  /** adjust closes the gap d by which the picture runs ahead of the sound, at t. */
  function adjust(d, t) {
    if (video.seeking) return;
    const c = correction(d);
    if (c.seek) {
      seek(t);
    } else if (video.playbackRate !== c.rate) {
      video.playbackRate = c.rate;
    }
    const at = now();
    drifts.push([at, Math.abs(d)]);
    while (drifts.length && drifts[0][0] < at - DRIFT_WINDOW * 1000) drifts.shift();
  }

  /** onFrame follows the sound at each frame of the picture shown. */
  function onFrame(_, meta) {
    video.requestVideoFrameCallback(onFrame);
    if (!cur) return;
    cur.shown = true;
    if (!wanted()) return;
    const t = target(clock, performance.timeOrigin + meta.expectedDisplayTime);
    adjust(meta.mediaTime - t, t);
  }

  if ("requestVideoFrameCallback" in video) {
    video.requestVideoFrameCallback(onFrame);
  } else {
    // Older WebKit: a few times a second, less closely.
    video.addEventListener("timeupdate", () => {
      if (cur && wanted()) adjust(video.currentTime - target(clock, now()), target(clock, now()));
    });
  }
  video.addEventListener("loadeddata", () => {
    if (cur) cur.shown = true;
  });
  video.addEventListener("timeupdate", () => {
    if (cur && wanted()) pump(cur);
  });
  video.addEventListener("error", () => {
    if (cur && video.error) fail(cur, new Error(video.error.message || "the picture doesn't decode (" + video.error.code + ")"));
  });
  document.addEventListener("visibilitychange", hold);

  /** drift tells how far the picture was from the sound over the last seconds, in milliseconds. */
  function drift() {
    if (drifts.length === 0) return { max: 0, mean: 0, frames: 0 };
    let max = 0;
    let sum = 0;
    for (const [, d] of drifts) {
      max = Math.max(max, d);
      sum += d;
    }
    return { max: Math.round(max * 1000), mean: Math.round((sum / drifts.length) * 1000), frames: drifts.length };
  }

  /** status tells the Go side how the picture is: shown once its first frame showed. */
  function status() {
    return {
      src: cur ? cur.desc.src : "",
      shown: cur !== null && cur.shown && !cur.failed,
      failed: cur ? cur.failed : "",
      drift: drift(),
    };
  }

  /**
   * codecs returns the families of the codecs this web view plays the
   * picture in through Media Source Extensions, in the order MixMusic
   * prefers them: those it decodes in hardware first, then VP9, whose
   * streams are smaller for the same picture, then H.264. AV1 isn't among
   * them: without hardware to decode it, it takes several times the CPU.
   */
  async function codecs() {
    if (!window.MediaSource) return [];
    // The types of YouTube's streams, and the same in full, which Media
    // Capabilities wants: it takes no "vp9".
    const kinds = [
      ["vp9", 'video/webm; codecs="vp9"', 'video/webm; codecs="vp09.00.21.08"'],
      ["avc1", 'video/mp4; codecs="avc1.4D401E"', 'video/mp4; codecs="avc1.4D401E"'],
    ];
    const found = [];
    for (const [family, type, exact] of kinds) {
      if (!MediaSource.isTypeSupported(type)) continue;
      let efficient = 0;
      try {
        const info = await navigator.mediaCapabilities.decodingInfo({
          type: "media-source",
          video: { contentType: exact, width: 640, height: 360, bitrate: 600000, framerate: 30 },
        });
        efficient = info.supported && info.powerEfficient ? 1 : 0;
      } catch {
        // It plays; how well isn't known.
      }
      found.push({ family, efficient });
    }
    found.sort((a, b) => b.efficient - a.efficient);
    return found.map((c) => c.family);
  }

  /**
   * mv is what the Go side calls. load plays a picture, sync tells the
   * sound's time and whether the picture shows ({at, sentAt, playing,
   * active, filter}), clear stops; each returns status().
   */
  root.mv = {
    codecs,
    load(desc) {
      load(desc);
      return status();
    },
    sync(s) {
      clock = { at: s.at, sentAt: s.sentAt, playing: !!s.playing };
      active = !!s.active;
      video.style.filter = s.filter || "";
      hold();
      return status();
    },
    clear() {
      unload();
      return status();
    },
    status,
  };
})(this);
