package qbt

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBuildCategoryRequest(t *testing.T) {
	t.Parallel()

	method, endpoint, body, headers := buildCategoryRequest("http://qb.test", "hash-1", "movies")

	if method != http.MethodPost {
		t.Fatalf("unexpected method: %s", method)
	}
	if endpoint != "http://qb.test/api/v2/torrents/setCategory" {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
	if headers["Content-Type"] != "application/x-www-form-urlencoded" {
		t.Fatalf("unexpected content type: %s", headers["Content-Type"])
	}
	payload := string(body)
	if !strings.Contains(payload, "hashes=hash-1") || !strings.Contains(payload, "category=movies") {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestBuildCategoryRequestForRemoval(t *testing.T) {
	t.Parallel()

	_, _, body, _ := buildCategoryRequest("http://qb.test", "hash-1", "")
	payload := string(body)
	if !strings.Contains(payload, "hashes=hash-1") || !strings.Contains(payload, "category=") {
		t.Fatalf("unexpected payload: %s", payload)
	}
}

func TestHandleCategoryResponse_PropagatesHTTPError(t *testing.T) {
	t.Parallel()

	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader("category rejected")),
		Header:     make(http.Header),
	}

	err := handleCategoryResponse(resp, "set category for torrent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "category rejected") {
		t.Fatalf("expected error body to be propagated, got %v", err)
	}
}

func TestGetRSSFeeds(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/items"] = endpointResponse{
		body: `{
			"RootFeed": {"url": "https://root.example/rss", "title": "Root", "lastBuild": "", "isLoading": false, "hasError": false, "articles": []},
			"Shows": {
				"Anime": {"url": "https://anime.example/rss", "title": "Anime", "lastBuild": "", "isLoading": false, "hasError": false, "articles": []},
				"Nested": {
					"Deep Show": {"url": "https://deep.example/rss", "title": "Deep Show", "lastBuild": "", "isLoading": false, "hasError": false, "articles": []}
				}
			}
		}`,
	}
	client := newTestClient(t, recorder.handler())

	feeds, err := client.GetRSSFeeds(false)
	if err != nil {
		t.Fatalf("GetRSSFeeds returned error: %v", err)
	}

	if len(feeds) != 3 {
		t.Fatalf("expected 3 feeds, got %d: %+v", len(feeds), feeds)
	}

	root, ok := feeds["RootFeed"]
	if !ok || root.URL != "https://root.example/rss" {
		t.Fatalf("expected root-level feed RootFeed, got %+v", feeds)
	}

	anime, ok := feeds[`Shows\Anime`]
	if !ok || anime.URL != "https://anime.example/rss" {
		t.Fatalf(`expected nested feed Shows\Anime, got %+v`, feeds)
	}

	deep, ok := feeds[`Shows\Nested\Deep Show`]
	if !ok || deep.URL != "https://deep.example/rss" {
		t.Fatalf(`expected doubly-nested feed Shows\Nested\Deep Show, got %+v`, feeds)
	}

	if _, ok := feeds["Shows"]; ok {
		t.Fatalf("folder %q should not appear in feeds, got %+v", "Shows", feeds)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{Path: "/api/v2/rss/items", Method: http.MethodGet},
	})
}

func TestGetRSSFeeds_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/items"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "forbidden",
	}
	client := newTestClient(t, recorder.handler())

	_, err := client.GetRSSFeeds(false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get RSS feeds") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestSetRSSFeedURL(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.SetRSSFeedURL("/Shows/feed", "http://example.com/new-feed"); err != nil {
		t.Fatalf("SetRSSFeedURL returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/setFeedURL",
			Method: http.MethodPost,
			Form: map[string]string{
				"path": "/Shows/feed",
				"url":  "http://example.com/new-feed",
			},
		},
	})
}

func TestSetRSSFeedURL_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/setFeedURL"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "url rejected",
	}
	client := newTestClient(t, recorder.handler())

	err := client.SetRSSFeedURL("/Shows/feed", "http://example.com/new-feed")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to set RSS feed URL") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "url rejected") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestAddRSSFolder(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.AddRSSFolder("/Shows"); err != nil {
		t.Fatalf("AddRSSFolder returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/addFolder",
			Method: http.MethodPost,
			Form:   map[string]string{"path": "/Shows"},
		},
	})
}

func TestAddRSSFolder_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/addFolder"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "folder rejected",
	}
	client := newTestClient(t, recorder.handler())

	err := client.AddRSSFolder("/Shows")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to add RSS folder") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "folder rejected") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestMoveRSSItem(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.MoveRSSItem("/Shows/old-name", "/Shows/new-name"); err != nil {
		t.Fatalf("MoveRSSItem returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/moveItem",
			Method: http.MethodPost,
			Form: map[string]string{
				"itemPath": "/Shows/old-name",
				"destPath": "/Shows/new-name",
			},
		},
	})
}

func TestRefreshRSSItem(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.RefreshRSSItem("/Shows/feed"); err != nil {
		t.Fatalf("RefreshRSSItem returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/refreshItem",
			Method: http.MethodPost,
			Form:   map[string]string{"itemPath": "/Shows/feed"},
		},
	})
}

func TestRefreshRSSItem_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/refreshItem"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "refresh failed",
	}
	client := newTestClient(t, recorder.handler())

	err := client.RefreshRSSItem("/Shows/feed")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to refresh RSS item") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "refresh failed") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestMarkRSSItemAsRead(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.MarkRSSItemAsRead("/Shows/feed", "article-1"); err != nil {
		t.Fatalf("MarkRSSItemAsRead returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/markAsRead",
			Method: http.MethodPost,
			Form: map[string]string{
				"itemPath":  "/Shows/feed",
				"articleId": "article-1",
			},
		},
	})
}

func TestMarkRSSItemAsRead_EmptyArticleIDMarksWholeFeed(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.MarkRSSItemAsRead("/Shows/feed", ""); err != nil {
		t.Fatalf("MarkRSSItemAsRead returned error: %v", err)
	}

	if len(recorder.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(recorder.calls))
	}
	call := recorder.calls[2]
	if call.Form.Get("itemPath") != "/Shows/feed" {
		t.Fatalf("unexpected itemPath: %s", call.Form.Get("itemPath"))
	}
	if _, ok := call.Form["articleId"]; ok {
		t.Fatalf("expected articleId to be omitted, got %v", call.Form["articleId"])
	}
}

func TestGetRSSRules(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/rules"] = endpointResponse{
		body: `{"linux-distros":{"enabled":true,"mustContain":"ubuntu","mustNotContain":"","useRegex":false,"episodeFilter":"","smartFilter":false,"previouslyMatchedEpisodes":[],"affectedFeeds":["http://example.com/feed"],"ignoreDays":0,"lastMatch":"","addPaused":false,"assignedCategory":"linux","savePath":"/downloads/linux"}}`,
	}
	client := newTestClient(t, recorder.handler())

	rules, err := client.GetRSSRules()
	if err != nil {
		t.Fatalf("GetRSSRules returned error: %v", err)
	}

	rule, ok := rules["linux-distros"]
	if !ok {
		t.Fatalf("expected rule linux-distros, got %v", rules)
	}
	if !rule.Enabled || rule.MustContain != "ubuntu" || rule.AssignedCategory != "linux" {
		t.Fatalf("unexpected rule contents: %+v", rule)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{Path: "/api/v2/rss/rules", Method: http.MethodGet},
	})
}

func TestGetRSSRules_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/rules"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "forbidden",
	}
	client := newTestClient(t, recorder.handler())

	_, err := client.GetRSSRules()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get RSS rules") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestSetRSSRule(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	rule := RSSRule{
		Enabled:          true,
		MustContain:      "1080p",
		AffectedFeeds:    []string{"http://example.com/feed"},
		AssignedCategory: "movies",
		SavePath:         "/downloads/movies",
	}

	if err := client.SetRSSRule("hd-movies", rule); err != nil {
		t.Fatalf("SetRSSRule returned error: %v", err)
	}

	if len(recorder.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(recorder.calls))
	}
	call := recorder.calls[2]
	if call.Path != "/api/v2/rss/setRule" || call.Method != http.MethodPost {
		t.Fatalf("unexpected call: %+v", call)
	}
	if call.Form.Get("ruleName") != "hd-movies" {
		t.Fatalf("unexpected ruleName: %s", call.Form.Get("ruleName"))
	}

	var decoded RSSRule
	if err := json.Unmarshal([]byte(call.Form.Get("ruleDef")), &decoded); err != nil {
		t.Fatalf("ruleDef is not valid JSON: %v", err)
	}
	if decoded.MustContain != rule.MustContain || decoded.AssignedCategory != rule.AssignedCategory {
		t.Fatalf("ruleDef did not round-trip: %+v", decoded)
	}
}

func TestSetRSSRule_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/setRule"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "invalid rule",
	}
	client := newTestClient(t, recorder.handler())

	err := client.SetRSSRule("hd-movies", RSSRule{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to set RSS rule") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid rule") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestRenameRSSRule(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.RenameRSSRule("hd-movies", "hd-movies-v2"); err != nil {
		t.Fatalf("RenameRSSRule returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/renameRule",
			Method: http.MethodPost,
			Form: map[string]string{
				"ruleName":    "hd-movies",
				"newRuleName": "hd-movies-v2",
			},
		},
	})
}

func TestRemoveRSSRule(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	if err := client.RemoveRSSRule("hd-movies"); err != nil {
		t.Fatalf("RemoveRSSRule returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/rss/removeRule",
			Method: http.MethodPost,
			Form:   map[string]string{"ruleName": "hd-movies"},
		},
	})
}

func TestRemoveRSSRule_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/removeRule"] = endpointResponse{
		statusCode: http.StatusNotFound,
		body:       "rule not found",
	}
	client := newTestClient(t, recorder.handler())

	err := client.RemoveRSSRule("hd-movies")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to remove RSS rule") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "rule not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestGetMatchingRSSArticles(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/matchingArticles"] = endpointResponse{
		body: `{"Linux Distros":["Article One","Article Two"]}`,
	}
	client := newTestClient(t, recorder.handler())

	matches, err := client.GetMatchingRSSArticles("hd-movies")
	if err != nil {
		t.Fatalf("GetMatchingRSSArticles returned error: %v", err)
	}

	articles, ok := matches["Linux Distros"]
	if !ok || len(articles) != 2 {
		t.Fatalf("unexpected matches: %v", matches)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{Path: "/api/v2/rss/matchingArticles", Method: http.MethodGet},
	})
}

func TestGetMatchingRSSArticles_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/rss/matchingArticles"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "unknown rule",
	}
	client := newTestClient(t, recorder.handler())

	_, err := client.GetMatchingRSSArticles("hd-movies")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get matching RSS articles") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "unknown rule") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestAddTrackers(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.AddTrackers("hash-1", []string{"udp://tracker1.example:80", "udp://tracker2.example:80"})
	if err != nil {
		t.Fatalf("AddTrackers returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/addTrackers",
			Method: http.MethodPost,
			Form: map[string]string{
				"hash": "hash-1",
				"urls": "udp://tracker1.example:80\nudp://tracker2.example:80",
			},
		},
	})
}

func TestAddTrackers_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/addTrackers"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "torrent not found",
	}
	client := newTestClient(t, recorder.handler())

	err := client.AddTrackers("hash-1", []string{"udp://tracker1.example:80"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to add trackers") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "torrent not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestRemoveTrackers(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.RemoveTrackers("hash-1", []string{"udp://tracker1.example:80", "udp://tracker2.example:80"})
	if err != nil {
		t.Fatalf("RemoveTrackers returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/removeTrackers",
			Method: http.MethodPost,
			Form: map[string]string{
				"hash": "hash-1",
				"urls": "udp://tracker1.example:80\nudp://tracker2.example:80",
			},
		},
	})
}

func TestRemoveTrackers_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/removeTrackers"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "torrent not found",
	}
	client := newTestClient(t, recorder.handler())

	err := client.RemoveTrackers("hash-1", []string{"udp://tracker1.example:80"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to remove trackers") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "torrent not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestEditTracker(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.EditTracker("hash-1", "udp://old.example:80", "udp://new.example:80")
	if err != nil {
		t.Fatalf("EditTracker returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/editTracker",
			Method: http.MethodPost,
			Form: map[string]string{
				"hash":    "hash-1",
				"origUrl": "udp://old.example:80",
				"newUrl":  "udp://new.example:80",
			},
		},
	})
}

func TestEditTracker_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/editTracker"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "tracker not found",
	}
	client := newTestClient(t, recorder.handler())

	err := client.EditTracker("hash-1", "udp://old.example:80", "udp://new.example:80")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to edit tracker") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "tracker not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestTopTorrentsPriority(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.TopTorrentsPriority("hash-1")
	if err != nil {
		t.Fatalf("TopTorrentsPriority returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/topPrio",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes": "hash-1",
			},
		},
	})
}

func TestTopTorrentsPriority_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/topPrio"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "torrent not found",
	}
	client := newTestClient(t, recorder.handler())

	err := client.TopTorrentsPriority("hash-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to topPrio torrent") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "torrent not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestBottomTorrentsPriority(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.BottomTorrentsPriority("hash-1")
	if err != nil {
		t.Fatalf("BottomTorrentsPriority returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/torrents/bottomPrio",
			Method: http.MethodPost,
			Form: map[string]string{
				"hashes": "hash-1",
			},
		},
	})
}

func TestBottomTorrentsPriority_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/torrents/bottomPrio"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "torrent not found",
	}
	client := newTestClient(t, recorder.handler())

	err := client.BottomTorrentsPriority("hash-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to bottomPrio torrent") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "torrent not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestBanPeers(t *testing.T) {
	recorder := newAPITestRecorder(t)
	client := newTestClient(t, recorder.handler())

	err := client.BanPeers([]string{"1.2.3.4:6881", "5.6.7.8:51413"})
	if err != nil {
		t.Fatalf("BanPeers returned error: %v", err)
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/transfer/banPeers",
			Method: http.MethodPost,
			Form: map[string]string{
				"peers": "1.2.3.4:6881|5.6.7.8:51413",
			},
		},
	})
}

func TestBanPeers_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/transfer/banPeers"] = endpointResponse{
		statusCode: http.StatusBadRequest,
		body:       "invalid peer",
	}
	client := newTestClient(t, recorder.handler())

	err := client.BanPeers([]string{"1.2.3.4:6881"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to ban peers") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid peer") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}

func TestGetTorrentPeers(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/sync/torrentPeers"] = endpointResponse{
		statusCode: http.StatusOK,
		body: `{
			"full_update": true,
			"rid": 1,
			"peers": {
				"5.6.7.8:51413": {"ip": "5.6.7.8", "port": 51413, "client": "qBittorrent", "progress": 0.5},
				"1.2.3.4:6881": {"ip": "1.2.3.4", "port": 6881, "client": "Transmission", "progress": 1}
			}
		}`,
	}
	client := newTestClient(t, recorder.handler())

	peers, err := client.GetTorrentPeers("hash-1")
	if err != nil {
		t.Fatalf("GetTorrentPeers returned error: %v", err)
	}

	if len(peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(peers))
	}

	if peers[0].IP != "1.2.3.4" || peers[0].Port != 6881 || peers[0].Client != "Transmission" {
		t.Fatalf("unexpected first peer: %+v", peers[0])
	}
	if peers[1].IP != "5.6.7.8" || peers[1].Port != 51413 || peers[1].Progress != 0.5 {
		t.Fatalf("unexpected second peer: %+v", peers[1])
	}

	recorder.assertCalls(t, []expectedCall{
		{Path: "/api/v2/app/version", Method: http.MethodGet},
		{Path: "/api/v2/auth/login", Method: http.MethodPost},
		{
			Path:   "/api/v2/sync/torrentPeers",
			Method: http.MethodGet,
		},
	})
}

func TestGetTorrentPeers_PropagatesHTTPError(t *testing.T) {
	recorder := newAPITestRecorder(t)
	recorder.responses["/api/v2/sync/torrentPeers"] = endpointResponse{
		statusCode: http.StatusNotFound,
		body:       "torrent not found",
	}
	client := newTestClient(t, recorder.handler())

	_, err := client.GetTorrentPeers("hash-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get torrent peers") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "torrent not found") {
		t.Fatalf("expected response body in error, got %v", err)
	}
}
