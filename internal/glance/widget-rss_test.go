package glance

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRSSImageScraping(t *testing.T) {
	for _, tc := range []struct {
		name       string
		config     string
		feedImage  bool
		article    string
		status     int
		wantScrape bool
		wantImage  string
	}{
		{name: "disabled by default"},
		{name: "explicitly disabled", config: "scrape-images: false\n"},
		{name: "article image", config: "scrape-images: true\n", article: `<article><img src=""><img src="../images/photo.jpg"></article>`, status: 200, wantScrape: true, wantImage: "/images/photo.jpg"},
		{name: "main image", config: "scrape-images: true\n", article: `<main><img src="/images/photo.jpg"></main>`, status: 200, wantScrape: true, wantImage: "/images/photo.jpg"},
		{name: "post content image", config: "scrape-images: true\n", article: `<div class="post-content"><img src="/images/photo.jpg"></div>`, status: 200, wantScrape: true, wantImage: "/images/photo.jpg"},
		{name: "existing feed image", config: "scrape-images: true\n", feedImage: true, wantImage: "/feed.jpg"},
		{name: "no article image", config: "scrape-images: true\n", article: `<article>Text only</article>`, status: 200, wantScrape: true},
		{name: "article unavailable", config: "scrape-images: true\n", status: 503, wantScrape: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var articleRequests atomic.Int32
			var cacheHits atomic.Int32
			var serverURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Feed-Key") != "test-key" {
					t.Error("configured request header was not forwarded")
				}
				if r.URL.Path == "/feed" {
					if r.Header.Get("If-None-Match") == `"feed-v1"` {
						cacheHits.Add(1)
						w.WriteHeader(http.StatusNotModified)
						return
					}
					w.Header().Set("Content-Type", "application/rss+xml")
					w.Header().Set("ETag", `"feed-v1"`)
					feedImage := ""
					if tc.feedImage {
						feedImage = fmt.Sprintf(`<image><url>%s/feed.jpg</url><title>Feed</title><link>%s</link></image>`, serverURL, serverURL)
					}
					fmt.Fprintf(w, `<rss version="2.0"><channel><title>Feed</title><link>%s</link><description>Test</description>%s<item><title>First</title><link>%s/posts/first</link></item><item><title>Second</title><link>%s/posts/second</link></item></channel></rss>`, serverURL, feedImage, serverURL, serverURL)
					return
				}
				articleRequests.Add(1)
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				fmt.Fprint(w, tc.article)
			}))
			defer server.Close()
			serverURL = server.URL

			var request rssFeedRequest
			if err := yaml.Unmarshal([]byte(tc.config), &request); err != nil {
				t.Fatal(err)
			}
			request.URL = server.URL + "/feed"
			request.Headers = map[string]string{"X-Feed-Key": "test-key"}
			widget := &rssWidget{}
			if err := widget.initialize(); err != nil {
				t.Fatal(err)
			}
			// Fetch twice to verify scraped images survive conditional feed caching.
			for range 2 {
				items, err := widget.fetchItemsFromFeedTask(request)
				if err != nil {
					t.Fatal(err)
				}
				if len(items) != 2 {
					t.Fatalf("got %d items, want 2", len(items))
				}
				wantImage := tc.wantImage
				if wantImage != "" {
					wantImage = server.URL + wantImage
				}
				for i, item := range items {
					if item.ImageURL != wantImage {
						t.Errorf("item %d image = %q, want %q", i, item.ImageURL, wantImage)
					}
				}
			}
			wantRequests := int32(0)
			if tc.wantScrape {
				wantRequests = 2
			}
			if got := articleRequests.Load(); got != wantRequests {
				t.Errorf("article requests = %d, want %d", got, wantRequests)
			}
			if got := cacheHits.Load(); got != 1 {
				t.Errorf("conditional cache hits = %d, want 1", got)
			}
		})
	}
}
