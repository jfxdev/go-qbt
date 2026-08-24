package qbt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// GetRSSFeeds gets configured RSS feeds. qBittorrent's rss/items response is
// a recursive tree of feeds and folders; feeds nested inside folders are
// returned keyed by their full path (folder and feed names joined with
// "\", matching the path format used by AddFeed/RemoveFeed/MoveRSSItem),
// e.g. "Shows\Anime". Folders themselves are not included in the result.
func (qb *Client) GetRSSFeeds(withData bool) (map[string]RSSFeed, error) {
	params := url.Values{}
	params.Add("withData", fmt.Sprintf("%v", withData))

	endpoint := fmt.Sprintf("%s/api/v2/rss/items?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get RSS feeds: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get RSS feeds. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var tree map[string]json.RawMessage
	if err := json.Unmarshal(body, &tree); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	feeds := make(map[string]RSSFeed)
	if err := flattenRSSItems(tree, "", feeds); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return feeds, nil
}

// flattenRSSItems walks a raw rss/items JSON object, decoding leaf feeds
// (items with a "url" field) into feeds and recursing into folders
// (items without a "url" field), prefixing nested items with their
// parent path joined by "\".
func flattenRSSItems(items map[string]json.RawMessage, prefix string, feeds map[string]RSSFeed) error {
	for name, raw := range items {
		path := name
		if prefix != "" {
			path = prefix + `\` + name
		}

		var probe struct {
			URL *string `json:"url"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			return fmt.Errorf("item %q: %w", path, err)
		}

		if probe.URL != nil {
			var feed RSSFeed
			if err := json.Unmarshal(raw, &feed); err != nil {
				return fmt.Errorf("item %q: %w", path, err)
			}
			feeds[path] = feed
			continue
		}

		var children map[string]json.RawMessage
		if err := json.Unmarshal(raw, &children); err != nil {
			return fmt.Errorf("folder %q: %w", path, err)
		}
		if err := flattenRSSItems(children, path, feeds); err != nil {
			return err
		}
	}

	return nil
}

// AddRSSFeed adds a new RSS feed
func (qb *Client) AddRSSFeed(feedURL, path string) error {
	data := url.Values{}
	data.Set("url", feedURL)
	data.Set("path", path)

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/addFeed", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to add RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add RSS feed. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// RemoveRSSFeed removes an RSS feed
func (qb *Client) RemoveRSSFeed(path string) error {
	data := url.Values{
		"path": {path},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/removeItem", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to remove RSS feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove RSS feed. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// SetRSSFeedURL changes an existing RSS feed's URL in place. Requires
// qBittorrent 4.6.0+ (WebAPI v2.9.1+); on older servers, remove the feed
// with RemoveRSSFeed and re-add it with AddRSSFeed instead.
func (qb *Client) SetRSSFeedURL(path, feedURL string) error {
	data := url.Values{
		"path": {path},
		"url":  {feedURL},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/setFeedURL", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set RSS feed URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set RSS feed URL. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// AddRSSFolder adds a new RSS folder
func (qb *Client) AddRSSFolder(path string) error {
	data := url.Values{
		"path": {path},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/addFolder", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to add RSS folder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add RSS folder. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// MoveRSSItem moves or renames an RSS feed or folder
func (qb *Client) MoveRSSItem(itemPath, destPath string) error {
	data := url.Values{
		"itemPath": {itemPath},
		"destPath": {destPath},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/moveItem", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to move RSS item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to move RSS item. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// RefreshRSSItem forces an immediate refresh of an RSS feed or folder
func (qb *Client) RefreshRSSItem(itemPath string) error {
	data := url.Values{
		"itemPath": {itemPath},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/refreshItem", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to refresh RSS item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to refresh RSS item. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// MarkRSSItemAsRead marks an RSS article as read, or the whole feed as read
// when articleID is empty.
func (qb *Client) MarkRSSItemAsRead(itemPath string, articleID string) error {
	data := url.Values{
		"itemPath": {itemPath},
	}
	if articleID != "" {
		data.Set("articleId", articleID)
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/markAsRead", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to mark RSS item as read: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to mark RSS item as read. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetRSSRules gets all RSS auto-downloading rules, keyed by rule name
func (qb *Client) GetRSSRules() (map[string]RSSRule, error) {
	endpoint := fmt.Sprintf("%s/api/v2/rss/rules", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get RSS rules: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get RSS rules. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var rules map[string]RSSRule
	if err := json.Unmarshal(body, &rules); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return rules, nil
}

// SetRSSRule creates or updates an RSS auto-downloading rule
func (qb *Client) SetRSSRule(ruleName string, rule RSSRule) error {
	ruleDef, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to marshal RSS rule: %w", err)
	}

	data := url.Values{
		"ruleName": {ruleName},
		"ruleDef":  {string(ruleDef)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/setRule", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set RSS rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set RSS rule. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// RenameRSSRule renames an RSS auto-downloading rule
func (qb *Client) RenameRSSRule(ruleName, newRuleName string) error {
	data := url.Values{
		"ruleName":    {ruleName},
		"newRuleName": {newRuleName},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/renameRule", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to rename RSS rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to rename RSS rule. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// RemoveRSSRule removes an RSS auto-downloading rule
func (qb *Client) RemoveRSSRule(ruleName string) error {
	data := url.Values{
		"ruleName": {ruleName},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/rss/removeRule", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to remove RSS rule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove RSS rule. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetMatchingRSSArticles previews which articles an RSS rule would currently
// match, grouped by feed name.
func (qb *Client) GetMatchingRSSArticles(ruleName string) (map[string][]string, error) {
	params := url.Values{}
	params.Add("ruleName", ruleName)

	endpoint := fmt.Sprintf("%s/api/v2/rss/matchingArticles?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching RSS articles: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get matching RSS articles. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var matches map[string][]string
	if err := json.Unmarshal(body, &matches); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return matches, nil
}
