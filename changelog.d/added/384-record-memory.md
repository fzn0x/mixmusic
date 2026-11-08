- Record memory use, in Settings › About, for a report of MixMusic taking
  too much memory: while it's on, MixMusic writes how it uses its memory
  to `memory.log` in the logs folder each minute, its Go heap's and the
  whole app's as the system tells it, and saves a heap profile there once
  it passes 1 GB, or when you press save a heap profile. They hold only
  how MixMusic uses its memory, nothing of your music or accounts, and
  Report a problem tells of them. It's off again at the next start
  (#384).
