package qbt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (qb *Client) ListTorrents(opts ListOptions) ([]Torrent, error) {
	if err := qb.ensureLogin(); err != nil {
		return nil, err
	}

	params := url.Values{}
	if opts.Category != "" {
		params.Add("category", opts.Category)
	}

	resp, err := qb.sendRequest("GET", "/api/v2/torrents/info?"+params.Encode(), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch torrents. Status: %d, Response: %s", resp.StatusCode, body)
	}

	var torrents []Torrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return torrents, nil
}

func (qb *Client) AddTorrentLink(opts TorrentConfig) error {
	if err := qb.ensureLogin(); err != nil {
		return err
	}

	data := url.Values{
		"urls":          {opts.MagnetURI},
		"savepath":      {opts.Directory},
		"category":      {opts.Category},
		"paused":        {fmt.Sprintf("%v", opts.Paused)},
		"skip_checking": {fmt.Sprintf("%v", opts.SkipChecking)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := qb.sendRequest("POST", "/api/v2/torrents/add", strings.NewReader(data.Encode()), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// Reusable pause/resume function
func (qb *Client) updateTorrentStatus(action, hash string, optional map[string]string) error {
	if err := qb.ensureLogin(); err != nil {
		return err
	}

	data := url.Values{"hashes": {hash}}
	for k, v := range optional {
		data[k] = []string{v}
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := qb.sendRequest("POST", fmt.Sprintf("/api/v2/torrents/%s", action), strings.NewReader(data.Encode()), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to %s torrent. Status: %d, Response: %s", action, resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) PauseTorrents(hash string) error {
	return qb.updateTorrentStatus("pause", hash, nil)
}

func (qb *Client) ResumeTorrents(hash string) error {
	return qb.updateTorrentStatus("resume", hash, nil)
}

func (qb *Client) DeleteTorrents(hash string, deleteFiles bool) error {
	opt := map[string]string{
		"deleteFiles": fmt.Sprintf("%v", deleteFiles),
	}

	return qb.updateTorrentStatus("delete", hash, opt)
}

func (qb *Client) IncreaseTorrentsPriority(hash string) error {
	return qb.updateTorrentStatus("increasePrio", hash, nil)
}

func (qb *Client) DecreaseTorrentsPriority(hash string) error {
	return qb.updateTorrentStatus("decreasePrio", hash, nil)
}

func (qb *Client) AddTorrentTags(hash string, tags []string) error {
	if err := qb.ensureLogin(); err != nil {
		return err
	}

	data := url.Values{
		"hashes": {hash},
		"tags":   tags,
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := qb.sendRequest("POST", "/api/v2/torrents/addTags", strings.NewReader(data.Encode()), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set tags to torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}
