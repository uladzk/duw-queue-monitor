package duwdoctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestParseChannel_Fixture(t *testing.T) {
	// Arrange
	html := readFixture(t, "channel.html")
	// Act
	posts, err := ParseChannel(html)
	// Assert
	if err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}
	if len(posts) != 20 {
		t.Fatalf("got %d posts, want 20", len(posts))
	}
	if posts[0].ID != 17963 || posts[len(posts)-1].ID != 17982 {
		t.Fatalf("ids: first=%d last=%d, want 17963/17982", posts[0].ID, posts[len(posts)-1].ID)
	}
}

func TestObservedState_SkipsDailySummary(t *testing.T) {
	// Arrange — newest post (17982) is the daily summary; most recent STATUS post is the moon (17981)
	posts, err := ParseChannel(readFixture(t, "channel.html"))
	if err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}
	// Act
	st, at, ok := ObservedState(posts)
	// Assert
	if !ok || st != StateInactive {
		t.Fatalf("ObservedState = (%v, ok=%v), want (Inactive, true)", st, ok)
	}
	want := time.Date(2026, 6, 15, 13, 38, 54, 0, time.UTC)
	if !at.Equal(want) {
		t.Fatalf("status post time = %s, want %s", at, want)
	}
}

func TestClassifyStatus_SubstringTrap(t *testing.T) {
	// Arrange — "niedostępna" contains "dostępna"; must NOT classify as ActiveEnabled
	cases := map[string]State{
		"💤 Kolejka odbiór karty jest obecnie niedostępna (na razie nie ma wolnych biletów).": StateActiveDisabled,
		"🔔 Kolejka odbiór karty jest dostępna! Pozostało biletów: 8":                         StateActiveEnabled,
		"🌙 Kolejka odbiór karty jest nieaktywna — prawdopodobnie koniec godzin pracy DUW.":    StateInactive,
		"📊 Kolejka Odbiór karty pobytu — podsumowanie dnia:":                                  "", // not a status
	}
	for text, want := range cases {
		st, ok := classifyStatus(text)
		if want == "" {
			if ok {
				t.Fatalf("classifyStatus(%q) ok=true, want not-a-status", text)
			}
			continue
		}
		// Act / Assert
		if !ok || st != want {
			t.Fatalf("classifyStatus(%q) = (%v, %v), want %v", text, st, ok, want)
		}
	}
}

func TestMaxIdenticalRun(t *testing.T) {
	base := time.Date(2026, 6, 16, 9, 35, 0, 0, time.UTC)
	now := base.Add(70 * time.Second)
	window := 10 * time.Minute
	flood := func(n int) []Post {
		out := make([]Post, n)
		for i := 0; i < n; i++ {
			out[i] = Post{ID: 18000 + i, At: base.Add(time.Duration(i) * 3 * time.Second),
				Text: "🔔 Kolejka odbiór karty jest dostępna! Pozostało biletów: 8"}
		}
		return out
	}
	cases := []struct {
		name  string
		posts []Post
		now   time.Time
		want  int
	}{
		{"22-identical-recent", flood(22), now, 22},
		{"flood-too-old", flood(22), base.Add(2 * time.Hour), 0},
		{"mixed-recent", []Post{
			{ID: 1, At: now.Add(-2 * time.Minute), Text: "a"},
			{ID: 2, At: now.Add(-1 * time.Minute), Text: "b"},
			{ID: 3, At: now, Text: "c"},
		}, now, 1},
		{"empty", nil, now, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaxIdenticalRun(tc.posts, tc.now, window); got != tc.want {
				t.Fatalf("MaxIdenticalRun(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}
