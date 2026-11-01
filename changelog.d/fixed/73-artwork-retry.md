- A cover that fails to load on a flaky connection tries again within
  seconds, rather than staying grey for a minute, and the others of its
  service follow as soon as one loads. A connection that went silent is
  dropped in about half a minute. The log now says which service's
  covers failed and why (#73).
