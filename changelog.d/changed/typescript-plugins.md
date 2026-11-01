- Plugins are written in TypeScript: the community's sleep timer and
  spectrum, and the plugin API itself, which the type checker now holds to
  the types of `sdk/index.d.ts`. MixMusic still runs the JavaScript they
  compile to, so every plugin, and every installed version, works as
  before. The plugin docs show how to write and compile one.
