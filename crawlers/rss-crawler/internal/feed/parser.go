package feed

import (
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

// ParsedItem holds the fields extracted from a single RSS/Atom feed item.
type ParsedItem struct {
	Title      string
	RawText    string // description or content
	Link       string
	Published  time.Time
	GUID       string
	Categories []string
	Author     string
}

// Parse fetches and parses an RSS/Atom feed from the given URL.
// Returns the feed title and the parsed items.
func Parse(feedURL string) (string, []ParsedItem, error) {
	parser := gofeed.NewParser()
	parser.UserAgent = "aer-rss-crawler/1.0 (github.com/frogfromlake/aer)"

	f, err := parser.ParseURL(feedURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse feed %s: %w", feedURL, err)
	}

	var items []ParsedItem
	for _, item := range f.Items {
		if item == nil {
			continue
		}

		pi := ParsedItem{
			Title: item.Title,
			Link:  item.Link,
			GUID:  itemGUID(item),
		}

		// Prefer full content over truncated description
		if item.Content != "" {
			pi.RawText = item.Content
		} else {
			pi.RawText = item.Description
		}

		if item.PublishedParsed != nil {
			pi.Published = item.PublishedParsed.UTC()
		} else if item.UpdatedParsed != nil {
			pi.Published = item.UpdatedParsed.UTC()
		} else {
			pi.Published = time.Now().UTC()
		}

		if item.Author != nil {
			pi.Author = item.Author.Name
		}

		pi.Categories = item.Categories

		items = append(items, pi)
	}

	return f.Title, items, nil
}

// ParseString parses RSS/Atom XML from a string (used for testing).
func ParseString(data string) (string, []ParsedItem, error) {
	parser := gofeed.NewParser()

	f, err := parser.ParseString(data)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse feed string: %w", err)
	}

	var items []ParsedItem
	for _, item := range f.Items {
		if item == nil {
			continue
		}

		pi := ParsedItem{
			Title: item.Title,
			Link:  item.Link,
			GUID:  itemGUID(item),
		}

		if item.Content != "" {
			pi.RawText = item.Content
		} else {
			pi.RawText = item.Description
		}

		if item.PublishedParsed != nil {
			pi.Published = item.PublishedParsed.UTC()
		} else if item.UpdatedParsed != nil {
			pi.Published = item.UpdatedParsed.UTC()
		} else {
			pi.Published = time.Now().UTC()
		}

		if item.Author != nil {
			pi.Author = item.Author.Name
		}

		pi.Categories = item.Categories

		items = append(items, pi)
	}

	return f.Title, items, nil
}

// itemGUID returns a stable unique identifier for a feed item.
func itemGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	if item.Link != "" {
		return item.Link
	}
	return item.Title
}
