package duwdoctor

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Post is one parsed t.me/s channel message.
type Post struct {
	ID   int
	At   time.Time
	Text string
}

// ParseChannel parses t.me/s/<channel> HTML into posts sorted ascending by id.
// Ported from duw-mcp-server src/services/telegram-web.ts (parseChannelHtml).
func ParseChannel(html string) ([]Post, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var posts []Post
	doc.Find(".tgme_widget_message[data-post]").Each(func(_ int, s *goquery.Selection) {
		dp, ok := s.Attr("data-post") // "<channel>/<id>"
		if !ok {
			return
		}
		parts := strings.Split(dp, "/")
		id, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return
		}
		textSel := s.Find(".tgme_widget_message_text").First()
		textSel.Find("br").ReplaceWithHtml("\n")
		// NOTE: the first arg below is a literal non-breaking space (U+00A0), normalized to
		// a regular space — same as the cheerio source's .replace(/<nbsp>/g, " ").
		text := strings.TrimSpace(strings.ReplaceAll(textSel.Text(), " ", " "))
		if text == "" {
			return // media-only / service post
		}
		dt, _ := s.Find("a.tgme_widget_message_date time").Attr("datetime")
		at, _ := time.Parse(time.RFC3339, dt)
		posts = append(posts, Post{ID: id, At: at.UTC(), Text: text})
	})
	sort.Slice(posts, func(i, j int) bool { return posts[i].ID < posts[j].ID })
	return posts, nil
}

// ObservedState returns the state implied by the most recent STATUS post
// (scanning newest->oldest, skipping the daily summary / non-status posts).
func ObservedState(posts []Post) (State, time.Time, bool) {
	for i := len(posts) - 1; i >= 0; i-- {
		if st, ok := classifyStatus(posts[i].Text); ok {
			return st, posts[i].At, true
		}
	}
	return "", time.Time{}, false
}

// classifyStatus order matters: "niedostępna" contains "dostępna".
func classifyStatus(text string) (State, bool) {
	switch {
	case strings.Contains(text, "nieaktywna"):
		return StateInactive, true
	case strings.Contains(text, "niedostępna"):
		return StateActiveDisabled, true
	case strings.Contains(text, "dostępna"):
		return StateActiveEnabled, true
	default:
		return "", false
	}
}

// MaxIdenticalRun returns the longest run of consecutive byte-identical Text values
// among posts whose At is within `window` of `now`. The window is relative to NOW
// (not the newest post) so an old flood does not re-trigger forever.
func MaxIdenticalRun(posts []Post, now time.Time, window time.Duration) int {
	cutoff := now.Add(-window)
	var recent []Post
	for _, p := range posts {
		if !p.At.Before(cutoff) {
			recent = append(recent, p)
		}
	}
	if len(recent) == 0 {
		return 0
	}
	best, cur := 1, 1
	for i := 1; i < len(recent); i++ {
		if recent[i].Text == recent[i-1].Text {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
	}
	return best
}
