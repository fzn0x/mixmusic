package main

// language is a language the landing page is in.
type language struct {
	// Dir is the folder of its page, under the site's root: "" for
	// English, the root's own page.
	Dir string
	// Tag is its BCP 47 tag, for the page's lang and hreflang.
	Tag string
	// Name is its name, in itself, for the language menu.
	Name string
	// Locale is its Open Graph locale.
	Locale string
	// RTL is set for a language read right to left.
	RTL bool
}

// languages are the landing page's languages, English first, then those
// of the most populous countries, in the README's order: site/i18n/<Dir>.json
// translates the page into each.
var languages = []language{
	{Dir: "", Tag: "en", Name: "English", Locale: "en_US"},
	{Dir: "hi", Tag: "hi", Name: "हिन्दी", Locale: "hi_IN"},
	{Dir: "zh-cn", Tag: "zh-Hans", Name: "简体中文", Locale: "zh_CN"},
	{Dir: "id", Tag: "id", Name: "Bahasa Indonesia", Locale: "id_ID"},
	{Dir: "ur", Tag: "ur", Name: "اردو", Locale: "ur_PK", RTL: true},
	{Dir: "pt-br", Tag: "pt-BR", Name: "Português (Brasil)", Locale: "pt_BR"},
	{Dir: "bn", Tag: "bn", Name: "বাংলা", Locale: "bn_BD"},
	{Dir: "ru", Tag: "ru", Name: "Русский", Locale: "ru_RU"},
	{Dir: "am", Tag: "am", Name: "አማርኛ", Locale: "am_ET"},
}

// path is the language's page under the site's root: "" or "hi/".
func (l language) path() string {
	if l.Dir == "" {
		return ""
	}
	return l.Dir + "/"
}
