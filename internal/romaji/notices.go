package romaji

import _ "embed"

// License is the MIT license of kagome, the morphological analyser that
// reads kanji, and of kagome-dict, which loads its dictionary: copies of
// MixMusic must reproduce them.
var License = "kagome\n\n" + kagomeLicense + "\nkagome-dict\n\n" + kagomeDictLicense

var (
	//go:embed LICENSE.kagome
	kagomeLicense string
	//go:embed LICENSE.kagome-dict
	kagomeDictLicense string
)

// DictionaryNotice is the notice of the IPA dictionary, mecab-ipadic,
// which the dictionary of kagome is made from, and which must go with it:
// MixMusic downloads it the first time romaji is turned on.
//
//go:embed NOTICE.ipadic
var DictionaryNotice string
