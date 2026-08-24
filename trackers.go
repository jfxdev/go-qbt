package qbt

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AddTrackers adds one or more trackers to a torrent. Adding a tracker
// that's already present on the torrent is a no-op on qBittorrent's side.
func (qb *Client) AddTrackers(hash string, urls []string) error {
	data := url.Values{
		"hash": {hash},
		"urls": {strings.Join(urls, "\n")},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/addTrackers", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to add trackers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add trackers. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// RemoveTrackers removes one or more trackers from a torrent. Removing a
// tracker that isn't present on the torrent is a no-op on qBittorrent's
// side.
func (qb *Client) RemoveTrackers(hash string, urls []string) error {
	data := url.Values{
		"hash": {hash},
		"urls": {strings.Join(urls, "\n")},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/removeTrackers", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to remove trackers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove trackers. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// EditTracker swaps a torrent tracker's URL in place, preserving
// per-tracker state (like tier) that a RemoveTrackers+AddTrackers pair
// would lose.
func (qb *Client) EditTracker(hash, origURL, newURL string) error {
	data := url.Values{
		"hash":    {hash},
		"origUrl": {origURL},
		"newUrl":  {newURL},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/editTracker", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to edit tracker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to edit tracker. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}
