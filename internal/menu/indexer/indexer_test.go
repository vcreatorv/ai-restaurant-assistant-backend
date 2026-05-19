package indexer

import (
	"strings"
	"testing"
)

// TestBuildEmbedText_NoPairing — без pairing-тегов выходной формат не меняется
// (то, что было до миграции 000012).
func TestBuildEmbedText_NoPairing(t *testing.T) {
	got := BuildEmbedText(DishView{
		Name:         "Дорадо",
		Description:  "Свежая дорадо целиком с лаймом.",
		Composition:  "дорадо, лайм, оливковое масло",
		Cuisine:      "european",
		CategoryName: "Морепродукты",
	})
	want := "Дорадо. Свежая дорадо целиком с лаймом. Состав: дорадо, лайм, оливковое масло. Кухня: европейская. Категория: Морепродукты."
	if got != want {
		t.Fatalf("mismatch:\n  got:  %q\n  want: %q", got, want)
	}
}

// TestBuildEmbedText_WithPairingAllAxes — все 4 оси, проверяем порядок блоков
// и группировку фраз внутри оси.
func TestBuildEmbedText_WithPairingAllAxes(t *testing.T) {
	got := BuildEmbedText(DishView{
		Name:         "Дорадо",
		Description:  "Свежая дорадо целиком.",
		Composition:  "дорадо, лайм",
		Cuisine:      "european",
		CategoryName: "Морепродукты",
		PairingTags: []PairingTagView{
			{Axis: "drink", EmbedPhrase: "белому вину"},
			{Axis: "drink", EmbedPhrase: "игристому, шампанскому"},
			{Axis: "role", EmbedPhrase: "основное горячее"},
			{Axis: "occasion", EmbedPhrase: "романтического ужина, свидания"},
			{Axis: "vibe", EmbedPhrase: "лёгкое, не тяжёлое"},
		},
	})
	// Проверяем, что все 4 префикса есть в правильном порядке (drink → role → occasion → vibe).
	mustContain(t, got, "Подходит к: белому вину, игристому, шампанскому.")
	mustContain(t, got, "Подаётся как: основное горячее.")
	mustContain(t, got, "Хорошо для: романтического ужина, свидания.")
	mustContain(t, got, "Тип: лёгкое, не тяжёлое.")
	// Порядок: drink перед role перед occasion перед vibe.
	mustOrdered(t, got,
		"Подходит к:", "Подаётся как:", "Хорошо для:", "Тип:")
}

// TestBuildEmbedText_OnlyOneAxis — тегов только одной оси: остальных префиксов нет.
func TestBuildEmbedText_OnlyOneAxis(t *testing.T) {
	got := BuildEmbedText(DishView{
		Name:         "Картофель фри",
		Cuisine:      "european",
		CategoryName: "Гарниры",
		PairingTags: []PairingTagView{
			{Axis: "drink", EmbedPhrase: "светлому пиву, лагеру"},
			{Axis: "role", EmbedPhrase: "гарнир к мясу или рыбе"},
		},
	})
	mustContain(t, got, "Подходит к: светлому пиву, лагеру.")
	mustContain(t, got, "Подаётся как: гарнир к мясу или рыбе.")
	mustNotContain(t, got, "Хорошо для:")
	mustNotContain(t, got, "Тип:")
}

// TestBuildEmbedText_EmptyPhraseIgnored — теги с пустой фразой не появляются в выводе.
func TestBuildEmbedText_EmptyPhraseIgnored(t *testing.T) {
	got := BuildEmbedText(DishView{
		Name:         "Х",
		Cuisine:      "russian",
		CategoryName: "Закуски",
		PairingTags: []PairingTagView{
			{Axis: "drink", EmbedPhrase: ""},
		},
	})
	mustNotContain(t, got, "Подходит к:")
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected %q to contain %q", s, sub)
	}
}

func mustNotContain(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Fatalf("expected %q NOT to contain %q", s, sub)
	}
}

func mustOrdered(t *testing.T, s string, parts ...string) {
	t.Helper()
	pos := -1
	for _, p := range parts {
		idx := strings.Index(s, p)
		if idx < 0 {
			t.Fatalf("part %q not found in %q", p, s)
		}
		if idx <= pos {
			t.Fatalf("part %q out of order in %q (prev pos=%d, this=%d)", p, s, pos, idx)
		}
		pos = idx
	}
}
