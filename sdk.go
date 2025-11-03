package qbt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jfxdev/go-qbt/request"
)

// Helper to perform requests with automatic retry
func (qb *Client) doWithRetry(method, endpoint string, body io.Reader, headers map[string]string) (*http.Response, error) {
	var resp *http.Response
	var err error

	// Use a simple retry without context to avoid cancellation issues
	err = qb.retryWithBackoff(func() error {
		// Ensure we are logged in using context.Background() to avoid cancellation
		if err := qb.ensureLoginWithContext(context.Background()); err != nil {
			return fmt.Errorf("failed to ensure login: %w", err)
		}

		// Perform the request without context to avoid cancellation issues
		resp, err = request.Do(method, endpoint,
			request.WithBody(body),
			request.WithHeaders(headers),
			request.WithCookieJar(qb.config.jar),
		)

		if err != nil {
			return err
		}

		// Check for authentication errors and invalidate cookies
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			qb.invalidateCookies()
			return fmt.Errorf("authentication error: status code %d", resp.StatusCode)
		}

		// Retry on retryable status codes
		if qb.isRetryableStatusCode(resp.StatusCode) {
			return fmt.Errorf("retryable status code: %d", resp.StatusCode)
		}

		return nil
	}, fmt.Sprintf("%s %s", method, endpoint))

	return resp, err
}

func (qb *Client) isRetryableStatusCode(statusCode int) bool {
	for _, code := range qb.retryConfig.RetryableCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

func (qb *Client) ListTorrents(opts ListOptions) ([]*TorrentResponse, error) {
	params := url.Values{}
	if opts.Category != "" {
		params.Add("category", opts.Category)
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/info?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list torrents: %w", err)
	}
	defer resp.Body.Close()

	var response []*TorrentResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	for _, torrent := range response {
		torrent.MagnetLink, err = ParseMagnetLink(torrent.MagnetURI)
		if err != nil {
			return nil, fmt.Errorf("failed to parse magnet link: %w", err)
		}
	}

	return response, nil
}

func (qb *Client) AddTorrentLink(opts TorrentConfig) error {
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

	endpoint := fmt.Sprintf("%s/api/v2/torrents/add", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to add torrent: %w", err)
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
	data := url.Values{"hashes": {hash}}
	for k, v := range optional {
		data[k] = []string{v}
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/%s", qb.config.BaseURL, action)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to %s torrent: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to %s torrent. Status: %d, Response: %s", action, resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) StartTorrents(hash string) error {
	return qb.updateTorrentStatus("start", hash, nil)
}

func (qb *Client) StopTorrents(hash string) error {
	return qb.updateTorrentStatus("stop", hash, nil)
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
	data := url.Values{
		"hashes": {hash},
		"tags":   tags,
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/addTags", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to add tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set tags to torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) DeleteTorrentTags(hash string, tags []string) error {
	data := url.Values{
		"hashes": {hash},
		"tags":   tags,
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/removeTags", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to remove tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove tags from torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) SetCategory(hash string, category string) error {
	data := url.Values{
		"hashes":   {hash},
		"category": {category},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setCategory", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set category: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set category for torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) RemoveCategory(hash string) error {
	data := url.Values{
		"hashes":   {hash},
		"category": {""}, // Empty category removes the category
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setCategory", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to remove category: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove category from torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) ListTorrentFiles(hash string) ([]*TorrentFile, error) {
	params := url.Values{}
	params.Add("hash", hash)

	endpoint := fmt.Sprintf("%s/api/v2/torrents/files?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list torrent files: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body first to avoid context cancellation during JSON decoding
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list torrent files. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var files []*TorrentFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return files, nil
}

func (qb *Client) ForceRecheck(hash string) error {
	data := url.Values{
		"hashes": {hash},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/recheck", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to force recheck: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to force recheck torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) ForceReannounce(hash string) error {
	data := url.Values{
		"hashes": {hash},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/reannounce", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to force reannounce: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to force reannounce torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) GetTorrent(hash string) (*TorrentResponse, error) {
	params := url.Values{}
	params.Add("hashes", hash)

	endpoint := fmt.Sprintf("%s/api/v2/torrents/info?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body first to avoid context cancellation during JSON decoding
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var torrents []*TorrentResponse
	if err := json.Unmarshal(body, &torrents); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if len(torrents) == 0 {
		return nil, fmt.Errorf("torrent not found with hash: %s", hash)
	}

	// Parse magnet link for the torrent
	torrent := torrents[0]
	torrent.MagnetLink, err = ParseMagnetLink(torrent.MagnetURI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse magnet link: %w", err)
	}

	return torrent, nil
}

func (qb *Client) GetTorrentProperties(hash string) (*TorrentProperties, error) {
	params := url.Values{}
	params.Add("hash", hash)

	endpoint := fmt.Sprintf("%s/api/v2/torrents/properties?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent properties: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body first to avoid context cancellation during JSON decoding
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent properties. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var properties TorrentProperties
	if err := json.Unmarshal(body, &properties); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	// Normalize fields: some API versions use different field names
	// Use max_ratio if ratio_limit is 0
	if properties.RatioLimit == 0 && properties.MaxRatio != 0 {
		properties.RatioLimit = properties.MaxRatio
	}
	// Use max_seeding_time if seeding_time_limit is 0
	if properties.SeedingTimeLimit == 0 && properties.MaxSeedingTime != 0 {
		properties.SeedingTimeLimit = properties.MaxSeedingTime
	}

	return &properties, nil
}

func (qb *Client) StopTorrent(hash string) error {
	return qb.updateTorrentStatus("pause", hash, nil)
}

func (qb *Client) StartTorrent(hash string) error {
	return qb.updateTorrentStatus("resume", hash, nil)
}

func (qb *Client) ForceStart(hash string) error {
	data := url.Values{
		"hashes": {hash},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setForceStart", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to force start torrent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to force start torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) GetMainData() (*MainDataResponse, error) {
	// Use a more robust approach without context for the main data call
	endpoint := fmt.Sprintf("%s/api/v2/sync/maindata", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get main data: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body first to avoid context cancellation during JSON decoding
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var result *MainDataResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return result, nil
}

func (qb *Client) GetTransferInfo() (*TransferInfoResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v2/transfer/info", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get transfer info: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body first to avoid context cancellation during JSON decoding
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body (status: %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get transfer info. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var result *TransferInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error decoding response (status: %d, body: %s): %w", resp.StatusCode, string(body), err)
	}

	return result, nil
}

func (qb *Client) GetAppVersion() (string, error) {
	resp, err := qb.doWithRetry(http.MethodGet, fmt.Sprintf("%s/api/v2/app/version", qb.config.BaseURL), nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get app version: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body (status: %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get app version. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

func (qb *Client) GetAPIVersion() (string, error) {
	resp, err := qb.doWithRetry(http.MethodGet, fmt.Sprintf("%s/api/v2/app/webapiVersion", qb.config.BaseURL), nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get api version: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response body (status: %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get api version. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

func (qb *Client) GetBuildInfo() (*TransferInfoResponse, error) {
	resp, err := qb.doWithRetry(http.MethodGet, fmt.Sprintf("%s/api/v2/app/buildInfo", qb.config.BaseURL), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get build info: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body first to avoid context cancellation during JSON decoding
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body (status: %d): %w", resp.StatusCode, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get build info. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var result TransferInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("error decoding response (status: %d, body: %s): %w", resp.StatusCode, string(body), err)
	}

	return &result, nil
}

// ===== ESSENTIAL FEATURES FOR SEEDBOX =====

// GetTorrentTrackers gets tracker information for a torrent
func (qb *Client) GetTorrentTrackers(hash string) ([]*TorrentTracker, error) {
	params := url.Values{}
	params.Add("hash", hash)

	endpoint := fmt.Sprintf("%s/api/v2/torrents/trackers?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent trackers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent trackers. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var trackers []*TorrentTracker
	if err := json.Unmarshal(body, &trackers); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return trackers, nil
}

// GetTorrentPeers gets peer information for a torrent
func (qb *Client) GetTorrentPeers(hash string) ([]*TorrentPeer, error) {
	params := url.Values{}
	params.Add("hash", hash)

	endpoint := fmt.Sprintf("%s/api/v2/torrents/peers?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get torrent peers: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get torrent peers. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var peers []*TorrentPeer
	if err := json.Unmarshal(body, &peers); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return peers, nil
}

// GetGlobalSettings gets qBittorrent global settings
func (qb *Client) GetGlobalSettings() (*GlobalSettings, error) {
	endpoint := fmt.Sprintf("%s/api/v2/app/preferences", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get global settings: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get global settings. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var settings GlobalSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &settings, nil
}

// SetGlobalSettings sets qBittorrent global settings
func (qb *Client) SetGlobalSettings(settings GlobalSettings) error {
	jsonData, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	endpoint := fmt.Sprintf("%s/api/v2/app/setPreferences", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(string(jsonData)), headers)
	if err != nil {
		return fmt.Errorf("failed to set global settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set global settings. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetCategories gets all categories
func (qb *Client) GetCategories() (map[string]Category, error) {
	endpoint := fmt.Sprintf("%s/api/v2/torrents/categories", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get categories. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var categories map[string]Category
	if err := json.Unmarshal(body, &categories); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return categories, nil
}

// CreateCategory creates a new category
func (qb *Client) CreateCategory(name, savePath string) error {
	data := url.Values{
		"category": {name},
		"savePath": {savePath},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/createCategory", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create category. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// DeleteCategory removes a category
func (qb *Client) DeleteCategory(name string) error {
	data := url.Values{
		"category": {name},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/deleteCategory", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete category. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetLogs gets system logs
func (qb *Client) GetLogs(normal bool, info bool, warning bool, critical bool, lastKnownID int) ([]*LogEntry, error) {
	params := url.Values{}
	params.Add("normal", fmt.Sprintf("%v", normal))
	params.Add("info", fmt.Sprintf("%v", info))
	params.Add("warning", fmt.Sprintf("%v", warning))
	params.Add("critical", fmt.Sprintf("%v", critical))
	params.Add("last_known_id", fmt.Sprintf("%d", lastKnownID))

	endpoint := fmt.Sprintf("%s/api/v2/log/main?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get logs. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var logs []*LogEntry
	if err := json.Unmarshal(body, &logs); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return logs, nil
}

// GetNetworkInfo gets network information
func (qb *Client) GetNetworkInfo() (*NetworkInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v2/transfer/info", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get network info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get network info. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var info NetworkInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &info, nil
}

// SetDownloadSpeedLimit sets the download speed limit
func (qb *Client) SetDownloadSpeedLimit(limit int) error {
	data := url.Values{
		"dl_limit": {fmt.Sprintf("%d", limit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/transfer/setDownloadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set download speed limit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set download speed limit. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetGlobalDownloadLimit gets the global download speed limit
func (qb *Client) GetGlobalDownloadLimit() (int, error) {
	endpoint := fmt.Sprintf("%s/api/v2/transfer/downloadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get global download limit: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get global download limit. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var limit int
	if err := json.Unmarshal(body, &limit); err != nil {
		return 0, fmt.Errorf("error decoding response: %w", err)
	}

	return limit, nil
}

// SetUploadSpeedLimit sets the upload speed limit
func (qb *Client) SetUploadSpeedLimit(limit int) error {
	data := url.Values{
		"up_limit": {fmt.Sprintf("%d", limit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/transfer/setUploadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set upload speed limit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set upload speed limit. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetGlobalUploadLimit gets the global upload speed limit
func (qb *Client) GetGlobalUploadLimit() (int, error) {
	endpoint := fmt.Sprintf("%s/api/v2/transfer/uploadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get global upload limit: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get global upload limit. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var limit int
	if err := json.Unmarshal(body, &limit); err != nil {
		return 0, fmt.Errorf("error decoding response: %w", err)
	}

	return limit, nil
}

// ToggleSpeedLimits toggles speed limits
func (qb *Client) ToggleSpeedLimits() error {
	endpoint := fmt.Sprintf("%s/api/v2/transfer/toggleSpeedLimitsMode", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to toggle speed limits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to toggle speed limits. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// SetGlobalRateLimits sets global download and upload speed limits
func (qb *Client) SetGlobalRateLimits(downloadLimit, uploadLimit int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update the rate limits
	settings.GlobalDLSpeedLimit = downloadLimit
	settings.GlobalUPSpeedLimit = uploadLimit
	settings.GlobalDLSpeedLimitEnabled = downloadLimit > 0
	settings.GlobalUPSpeedLimitEnabled = uploadLimit > 0

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// SetAlternativeRateLimits sets alternative global download and upload speed limits
func (qb *Client) SetAlternativeRateLimits(downloadLimit, uploadLimit int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update the alternative rate limits
	settings.AltGlobalSpeedLimit = downloadLimit
	settings.AlternativeGlobalSpeedLimit = uploadLimit
	settings.AltGlobalSpeedLimitEnabled = downloadLimit > 0
	settings.AlternativeGlobalSpeedLimitEnabled = uploadLimit > 0

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// SetTorrentDownloadLimit sets download speed limit for a specific torrent
func (qb *Client) SetTorrentDownloadLimit(hash string, limit int) error {
	data := url.Values{
		"hashes": {hash},
		"limit":  {fmt.Sprintf("%d", limit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setDownloadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set torrent download limit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set torrent download limit. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// SetTorrentUploadLimit sets upload speed limit for a specific torrent
func (qb *Client) SetTorrentUploadLimit(hash string, limit int) error {
	data := url.Values{
		"hashes": {hash},
		"limit":  {fmt.Sprintf("%d", limit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setUploadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set torrent upload limit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set torrent upload limit. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetTorrentDownloadLimit gets download speed limit for a specific torrent
func (qb *Client) GetTorrentDownloadLimit(hash string) (int, error) {
	data := url.Values{
		"hashes": {hash},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/downloadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return 0, fmt.Errorf("failed to get torrent download limit: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get torrent download limit. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var result map[string]int
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("error decoding response: %w", err)
	}

	limit, exists := result[hash]
	if !exists {
		return 0, fmt.Errorf("download limit not found for hash: %s", hash)
	}

	return limit, nil
}

// GetTorrentUploadLimit gets upload speed limit for a specific torrent
func (qb *Client) GetTorrentUploadLimit(hash string) (int, error) {
	data := url.Values{
		"hashes": {hash},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/uploadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return 0, fmt.Errorf("failed to get torrent upload limit: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get torrent upload limit. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var result map[string]int
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("error decoding response: %w", err)
	}

	limit, exists := result[hash]
	if !exists {
		return 0, fmt.Errorf("upload limit not found for hash: %s", hash)
	}

	return limit, nil
}

// SetTorrentShareLimit sets share limits for a specific torrent
// ratioLimit: -2 means use global limit, -1 means no limit
// seedingTimeLimit: -2 means use global limit, -1 means no limit (in minutes)
// inactiveSeedingTimeLimit: -2 means use global limit, -1 means no limit (in minutes)
func (qb *Client) SetTorrentShareLimit(hash string, ratioLimit float64, seedingTimeLimit int, inactiveSeedingTimeLimit int) error {
	data := url.Values{
		"hashes":                   {hash},
		"ratioLimit":               {fmt.Sprintf("%.2f", ratioLimit)},
		"seedingTimeLimit":         {fmt.Sprintf("%d", seedingTimeLimit)},
		"inactiveSeedingTimeLimit": {fmt.Sprintf("%d", inactiveSeedingTimeLimit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setShareLimits", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set torrent share limit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set torrent share limit. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// GetRSSFeeds gets configured RSS feeds
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

	var feeds map[string]RSSFeed
	if err := json.Unmarshal(body, &feeds); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return feeds, nil
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
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

// SetTorrentLocation sets the location for torrent files
func (qb *Client) SetTorrentLocation(hash string, location string) error {
	data := url.Values{
		"hashes":   {hash},
		"location": {location},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setLocation", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set torrent location: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set torrent location. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// RenameTorrent renames a torrent
func (qb *Client) RenameTorrent(hash string, newName string) error {
	data := url.Values{
		"hash": {hash},
		"name": {newName},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/rename", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to rename torrent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to rename torrent. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// SuperSeedingMode enables or disables super seeding for a torrent
func (qb *Client) SuperSeedingMode(hash string, enabled bool) error {
	data := url.Values{
		"hashes": {hash},
		"value":  {fmt.Sprintf("%v", enabled)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setSuperSeeding", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, strings.NewReader(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set super seeding mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set super seeding mode. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// ===== MAXIMUM ACTIVE TORRENT MANAGEMENT =====

// SetMaxActiveDownloads sets the maximum number of active downloads
func (qb *Client) SetMaxActiveDownloads(maxDownloads int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update the max active downloads
	settings.MaxActiveDownloads = maxDownloads

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// SetMaxActiveUploads sets the maximum number of active uploads
func (qb *Client) SetMaxActiveUploads(maxUploads int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update the max active uploads
	settings.MaxActiveUploads = maxUploads

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// SetMaxActiveTorrents sets the maximum number of active torrents
func (qb *Client) SetMaxActiveTorrents(maxTorrents int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update the max active torrents
	settings.MaxActiveTorrents = maxTorrents

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// SetMaxActiveCheckingTorrents sets the maximum number of active checking torrents
func (qb *Client) SetMaxActiveCheckingTorrents(maxChecking int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update the max active checking torrents
	settings.MaxActiveCheckingTorrents = maxChecking

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// SetMaxActiveTorrentLimits sets all maximum active torrent limits at once
func (qb *Client) SetMaxActiveTorrentLimits(maxDownloads, maxUploads, maxTorrents, maxChecking int) error {
	// Get current global settings
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return fmt.Errorf("failed to get global settings: %w", err)
	}

	// Update all the max active limits
	settings.MaxActiveDownloads = maxDownloads
	settings.MaxActiveUploads = maxUploads
	settings.MaxActiveTorrents = maxTorrents
	settings.MaxActiveCheckingTorrents = maxChecking

	// Apply the updated settings
	return qb.SetGlobalSettings(*settings)
}

// GetMaxActiveDownloads gets the current maximum number of active downloads
func (qb *Client) GetMaxActiveDownloads() (int, error) {
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return 0, fmt.Errorf("failed to get global settings: %w", err)
	}
	return settings.MaxActiveDownloads, nil
}

// GetMaxActiveUploads gets the current maximum number of active uploads
func (qb *Client) GetMaxActiveUploads() (int, error) {
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return 0, fmt.Errorf("failed to get global settings: %w", err)
	}
	return settings.MaxActiveUploads, nil
}

// GetMaxActiveTorrents gets the current maximum number of active torrents
func (qb *Client) GetMaxActiveTorrents() (int, error) {
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return 0, fmt.Errorf("failed to get global settings: %w", err)
	}
	return settings.MaxActiveTorrents, nil
}

// GetMaxActiveCheckingTorrents gets the current maximum number of active checking torrents
func (qb *Client) GetMaxActiveCheckingTorrents() (int, error) {
	settings, err := qb.GetGlobalSettings()
	if err != nil {
		return 0, fmt.Errorf("failed to get global settings: %w", err)
	}
	return settings.MaxActiveCheckingTorrents, nil
}
