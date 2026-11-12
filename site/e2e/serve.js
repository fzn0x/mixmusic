// A small static server for the built site, for the Playwright checks: it
// serves site/public on 127.0.0.1 only, so that nothing listens on the
// network. Build the site for it with
//   zola --root site build --base-url http://127.0.0.1:4173
import { createReadStream, statSync } from "node:fs";
import { createServer } from "node:http";
import { extname, join, normalize, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL("../public", import.meta.url)));
const port = Number(process.env.PORT || 4173);

const types = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json",
  ".txt": "text/plain; charset=utf-8",
  ".xml": "application/xml",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".webp": "image/webp",
  ".woff2": "font/woff2",
  ".mp4": "video/mp4",
};

const file = (path) => {
  try {
    const s = statSync(path);
    if (s.isDirectory()) return file(join(path, "index.html"));
    return s.isFile() ? { path, size: s.size } : null;
  } catch {
    return null;
  }
};

createServer((req, res) => {
  const url = new URL(req.url, "http://127.0.0.1");
  const path = normalize(join(root, decodeURIComponent(url.pathname)));
  if (path !== root && !path.startsWith(root + sep)) {
    res.writeHead(403).end();
    return;
  }
  let found = file(path);
  let status = 200;
  if (!found) {
    found = file(join(root, "404.html"));
    status = 404;
  }
  if (!found) {
    res.writeHead(404).end();
    return;
  }
  const head = { "content-type": types[extname(found.path)] || "application/octet-stream", "accept-ranges": "bytes", "cache-control": "no-store" };
  // Browsers ask for videos in ranges.
  const range = /^bytes=(\d*)-(\d*)$/.exec(req.headers.range || "");
  if (range && status === 200) {
    const start = range[1] === "" ? Math.max(0, found.size - Number(range[2])) : Number(range[1]);
    const end = range[1] === "" || range[2] === "" ? found.size - 1 : Math.min(Number(range[2]), found.size - 1);
    if (start > end || start >= found.size) {
      res.writeHead(416, { "content-range": `bytes */${found.size}` }).end();
      return;
    }
    res.writeHead(206, { ...head, "content-range": `bytes ${start}-${end}/${found.size}`, "content-length": end - start + 1 });
    if (req.method === "HEAD") return void res.end();
    createReadStream(found.path, { start, end }).pipe(res);
    return;
  }
  res.writeHead(status, { ...head, "content-length": found.size });
  if (req.method === "HEAD") return void res.end();
  createReadStream(found.path).pipe(res);
}).listen(port, "127.0.0.1", () => console.log(`serving ${root} at http://127.0.0.1:${port}`));
