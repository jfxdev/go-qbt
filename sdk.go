package qbt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jfxdev/go-qbt/request"
)

func (qb *Client) ListTorrents(opts ListOptions) ([]*TorrentResponse, error) {
	if err := qb.ensureLogin(); err != nil {
		return nil, err
	}

	params := url.Values{}
	if opts.Category != "" {
		params.Add("category", opts.Category)
	}

	resp, err := request.Do(http.MethodGet,
		fmt.Sprintf("%s/api/v2/torrents/info?%s", qb.config.BaseURL, params.Encode()),
		request.WithCookieJar(qb.config.jar),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch torrents. Status: %d, Response: %s", resp.StatusCode, body)
	}

	var response []*TorrentResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return response, nil
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

	resp, err := request.Do(http.MethodGet,
		fmt.Sprintf("%s/api/v2/torrents/add", qb.config.BaseURL),
		request.WithBody(strings.NewReader(data.Encode())),
		request.WithCookieJar(qb.config.jar),
		request.WithHeaders(headers),
	)
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

	resp, err := request.Do(http.MethodPost,
		fmt.Sprintf("%s/api/v2/torrents/%s", qb.config.BaseURL, action),
		request.WithBody(strings.NewReader(data.Encode())),
		request.WithCookieJar(qb.config.jar),
		request.WithHeaders(headers),
	)
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

	resp, err := request.Do(http.MethodPost,
		fmt.Sprintf("%s/api/v2/torrents/addTags", qb.config.BaseURL),
		request.WithCookieJar(qb.config.jar),
		request.WithBody(strings.NewReader(data.Encode())),
		request.WithHeaders(headers),
	)
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

func (qb *Client) GetMainData() (*MainDataResponse, error) {
	if err := qb.ensureLogin(); err != nil {
		return nil, err
	}

	resp, err := request.Do(http.MethodGet,
		fmt.Sprintf("%s/api/v2/sync/maindata", qb.config.BaseURL),
		request.WithCookieJar(qb.config.jar),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get main data. Status: %d, Response: %s", resp.StatusCode, body)
	}

	var result *MainDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return result, nil
}
