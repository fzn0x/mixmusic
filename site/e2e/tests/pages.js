// The pages the checks load: the home page, in English and in two of its
// other languages (Urdu, read right to left, and Hindi), and some docs pages
// of the kinds there are (an index, a long page of tables, one of code, one
// of images).
export const pages = [
  { name: "home", path: "/" },
  { name: "home in Urdu", path: "/ur/" },
  { name: "home in Hindi", path: "/hi/" },
  { name: "docs index", path: "/docs/" },
  { name: "docs getting started", path: "/docs/getting-started/" },
  { name: "docs settings", path: "/docs/settings/" },
  { name: "docs plugins", path: "/docs/plugins/" },
  { name: "docs troubleshooting", path: "/docs/troubleshooting/" },
];

// The home page's languages besides English, as tools/sitefinish makes them
// (languages.go): each at /<dir>/, with its lang.
export const languages = [
  { dir: "hi", tag: "hi" },
  { dir: "zh-cn", tag: "zh-Hans" },
  { dir: "id", tag: "id" },
  { dir: "ur", tag: "ur", rtl: true },
  { dir: "pt-br", tag: "pt-BR" },
  { dir: "bn", tag: "bn" },
  { dir: "ru", tag: "ru" },
  { dir: "am", tag: "am" },
];
