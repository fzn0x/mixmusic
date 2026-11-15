- Quitting MixMusic stops its plugins: the function a plugin returns to
  run when it stops runs at every quit too, as it does when the plugin is
  turned off, and what it keeps with `api.storage` then is saved. MixMusic
  waits 3 seconds at most for all of them, so a plugin can't hold up the
  quit (#623).
