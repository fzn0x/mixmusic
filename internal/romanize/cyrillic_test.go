package romanize

import "testing"

// TestCyrillic writes Cyrillic by the system of each language, as their
// standards write their own examples.
func TestCyrillic(t *testing.T) {
	for _, c := range []struct{ lang, in, want string }{
		// The examples of Ukraine's resolution 55 of 2010.
		{"uk", "Алушта Борщагівка Вишгород Гадяч Згорани Ґалаґан Рівне Єнакієве Наєнко Житомир Закарпаття Іршава Їжакевич Кадиївка Йосипівка Стрий Київ Миколаїв Ужгород Харків Біла Церква Чернівці Гоща Юрій Крюківка Яготин Ічня Знам'янка",
			"Alushta Borshchahivka Vyshhorod Hadiach Zghorany Galagan Rivne Yenakiieve Naienko Zhytomyr Zakarpattia Irshava Yizhakevych Kadyivka Yosypivka Stryi Kyiv Mykolaiv Uzhhorod Kharkiv Bila Tserkva Chernivtsi Hoshcha Yurii Kriukivka Yahotyn Ichnia Znamianka"},
		// BGN/PCGN, its signs left out.
		{"ru", "Москва Екатеринбург Нижний Новгород Хабаровск Ульяновск Тверь Подъём Йошкар-Ола", "Moskva Yekaterinburg Nizhniy Novgorod Khabarovsk Ulyanovsk Tver Podyom Yoshkar-Ola"},
		{"ru", "ещё всё ёлка", "yeshcho vsyo yolka"},
		// Belarus's national system, which the UN recommends.
		{"be", "Мінск Гомель Магілёў Брэст Віцебск Гродна Вязынка Бялынічы Ельск Лёзна", "Minsk Homieĺ Mahilioŭ Brest Viciebsk Hrodna Viazynka Bialyničy Jeĺsk Liozna"},
		{"be", "п'ю", "pju"},
		// The Streamlined System, and ия at the end of a word.
		{"bg", "София Пловдив Търговище Шумен България Ямбол Русе", "Sofia Plovdiv Targovishte Shumen Balgaria Yambol Ruse"},
		// Gaj's Latin alphabet, its digraphs in capitals.
		{"sr", "Београд Ђорђе Љубљана Нови Сад Ћуприја Џеп Чачак ЉУБАВ", "Beograd Đorđe Ljubljana Novi Sad Ćuprija Džep Čačak LJUBAV"},
		// Macedonia's system of 2008.
		{"mk", "Скопје Ѓорче Петров Охрид Кичево Ќафа Ѕвезда Љубојно Њивиште", "Skopje Gjorche Petrov Ohrid Kichevo Kjafa Dzvezda Ljubojno Njivishte"},
		{"kk", "Қазақстан Алматы Өскемен Шымкент", "Qazaqstan Almaty Öskemen Shymkent"},
		{"ky", "Бишкек Ысык-Көл Нарын", "Bishkek Ysyk-Köl Naryn"},
		{"mn", "Улаанбаатар Хөвсгөл Дархан Эрдэнэт Зуунмод", "Ulaanbaatar Khövsgöl Darkhan Erdenet Zuunmod"},
		// Made-up lines, with English.
		{"ru", "Привет, world", "Privet, world"},
		{"uk", "Паперовий змій летить", "Paperovyi zmii letyt"},
	} {
		if got := Line(c.in, c.lang); got != c.want {
			t.Errorf("%s: %q,\nwant %q", c.lang, got, c.want)
		}
	}
}

// TestCyrillicLanguage tells the language of Cyrillic by its letters when
// the lyrics' isn't known, Russian when they tell none, and writes it so.
func TestCyrillicLanguage(t *testing.T) {
	for in, want := range map[string]string{
		"Їжак і ґанок":     "uk",
		"Він їде":          "uk",
		"Ця ноч і зоркі":   "be",
		"Сонца ўзышло":     "be",
		"Ђак и ћерка":      "sr",
		"Љубав и њива":     "sr",
		"Ѓурѓа и ќерка":    "mk",
		"Път към щастието": "bg",
		"Подъезд и съёмка": "ru",
		"Бумажный змей":    "ru",
		"Қала мен дала":    "kk",
		"Көңүл":            "ky",
		"Өвөл ирлээ":       "mn",
	} {
		if got := cyrillicLanguage(in); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
	// Lyrics of no known language read by their letters.
	if got := Line("Їжак", ""); got != "Yizhak" {
		t.Errorf("Ukrainian of no known language: %q", got)
	}
	if got := Line("Їжак", "ru"); got != "Yizhak" {
		t.Errorf("Ukrainian told as Russian: %q", got)
	}
}
