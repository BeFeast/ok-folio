package telegram

import "testing"

func TestParseArtworkCaptionRealFixtures(t *testing.T) {
	cases := []struct {
		name    string
		caption string
		title   string
		artist  string
		medium  string
		year    int
	}{
		{"range+title", "Тереза Кондеминас Солер (Teresa Condeminas Soler, 1905 — 2003)\nУтро\n\nСекретный контент 🔞", "Утро", "Тереза Кондеминас Солер (Teresa Condeminas Soler, 1905 — 2003)", "", 0},
		{"title-year+range", "Обнаженная на софе 1933 г.\n\nГовард Чандлер Кристи (США, 1873 - 1952)\n\nСекретный контент 🔞", "Обнаженная на софе", "Говард Чандлер Кристи (США, 1873 - 1952)", "", 1933},
		{"title+medium+range", "\"Scinscape\" 1965 г.\n\nхолст, масло\n\nРальф Гоингс (США, 1928 - 2016)\n\nСекретный контент 🔞", "Scinscape", "Ральф Гоингс (США, 1928 - 2016)", "холст, масло", 1965},
		{"quoted-title+range", "\"Весна\"\n\nДжозеф Боулер (США, 1928 - 2017)\n\nСекретный контент 🔞", "Весна", "Джозеф Боулер (США, 1928 - 2017)", "", 0},
		{"no-parens-artist", "Грация\nВиктор Анатольевич Долгополов\n\nСекретный контент 🔞", "Грация", "Виктор Анатольевич Долгополов", "", 0},
		{"dash-title+born-only", "-\n\nДэниел Мейдман (США, р. 1975)\n\nСекретный контент 🔞", "", "Дэниел Мейдман (США, р. 1975)", "", 0},
		{"sentence-title+single-year", "Да здравствует Франция!\n\nОмар Ортис (Omar Ortiz, 1977)\n\nСекретный контент 🔞", "Да здравствует Франция!", "Омар Ортис (Omar Ortiz, 1977)", "", 0},
		{"dash-title+linen-medium", "-\n\nлён, масло\n\nСтивен Карнелиус Робертс (США, р. 1952)\n\nСекретный контент 🔞", "", "Стивен Карнелиус Робертс (США, р. 1952)", "лён, масло", 0},
		{"medium+born-only", "\"Обнажённая\"\n\nхолст, масло\n\nДжонни Идальго (Перу, р. 1970)\n\nСекретный контент 🔞", "Обнажённая", "Джонни Идальго (Перу, р. 1970)", "холст, масло", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := parseArtworkCaption(c.caption)
			if p.Title != c.title {
				t.Errorf("Title = %q, want %q", p.Title, c.title)
			}
			if p.Artist != c.artist {
				t.Errorf("Artist = %q, want %q", p.Artist, c.artist)
			}
			if p.Medium != c.medium {
				t.Errorf("Medium = %q, want %q", p.Medium, c.medium)
			}
			gotYear := 0
			if !p.Date.IsZero() {
				gotYear = p.Date.Year()
			}
			if gotYear != c.year {
				t.Errorf("year = %d, want %d", gotYear, c.year)
			}
		})
	}
}
