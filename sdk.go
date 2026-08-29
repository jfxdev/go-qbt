package qbt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/jfxdev/go-qbt/request"
)

// Helper to perform requests with automatic retry. All requests derive from
// the client's lifecycle context (qb.ctx), so Cancel() aborts them
// immediately - e.g. when the worker backing this client is deleted or its
// credentials change mid-request.
func (qb *Client) doWithRetry(method, endpoint string, body []byte, headers map[string]string) (*http.Response, error) {
	var resp *http.Response
	var err error

	err = qb.retryWithBackoffWithContext(qb.ctx, func() error {
		if err := qb.ensureLoginWithContext(qb.ctx); err != nil {
			return fmt.Errorf("failed to ensure login: %w", err)
		}

		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		resp, err = request.Do(method, endpoint,
			request.WithBody(bodyReader),
			request.WithHeaders(headers),
			request.WithCookieJar(qb.config.jar),
			request.WithContext(qb.ctx),
			request.WithTimeoutDuration(qb.config.RequestTimeout),
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

// AddTorrentFile adds a torrent from a .torrent file's raw bytes via
// multipart/form-data, mirroring AddTorrentLink's options.
func (qb *Client) AddTorrentFile(opts TorrentFileConfig) error {
	if len(opts.FileData) == 0 {
		return fmt.Errorf("torrent file data is empty")
	}

	fileName := opts.FileName
	if fileName == "" {
		fileName = "upload.torrent"
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("torrents", fileName)
	if err != nil {
		return fmt.Errorf("failed to create multipart file field: %w", err)
	}
	if _, err := part.Write(opts.FileData); err != nil {
		return fmt.Errorf("failed to write torrent file data: %w", err)
	}

	fields := map[string]string{
		"savepath":      opts.Directory,
		"category":      opts.Category,
		"paused":        fmt.Sprintf("%v", opts.Paused),
		"skip_checking": fmt.Sprintf("%v", opts.SkipChecking),
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return fmt.Errorf("failed to write multipart field %q: %w", key, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to finalize multipart body: %w", err)
	}

	headers := map[string]string{
		"Content-Type": writer.FormDataContentType(),
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/add", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, buf.Bytes(), headers)
	if err != nil {
		return fmt.Errorf("failed to add torrent file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add torrent file. Status: %d, Response: %s", resp.StatusCode, body)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

func (qb *Client) TopTorrentsPriority(hash string) error {
	return qb.updateTorrentStatus("topPrio", hash, nil)
}

func (qb *Client) BottomTorrentsPriority(hash string) error {
	return qb.updateTorrentStatus("bottomPrio", hash, nil)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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
	method, endpoint, body, headers := buildCategoryRequest(qb.config.BaseURL, hash, category)

	resp, err := qb.doWithRetry(method, endpoint, body, headers)
	if err != nil {
		return fmt.Errorf("failed to set category: %w", err)
	}
	return handleCategoryResponse(resp, "set category for torrent")
}

func (qb *Client) RemoveCategory(hash string) error {
	method, endpoint, body, headers := buildCategoryRequest(qb.config.BaseURL, hash, "")

	resp, err := qb.doWithRetry(method, endpoint, body, headers)
	if err != nil {
		return fmt.Errorf("failed to remove category: %w", err)
	}
	return handleCategoryResponse(resp, "remove category from torrent")
}

func buildCategoryRequest(baseURL, hash, category string) (string, string, []byte, map[string]string) {
	data := url.Values{
		"hashes":   {hash},
		"category": {category},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/setCategory", baseURL)
	return http.MethodPost, endpoint, []byte(data.Encode()), headers
}

func handleCategoryResponse(resp *http.Response, action string) error {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to %s. Status: %d, Response: %s", action, resp.StatusCode, body)
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

// SetFilePriority sets the download priority of one or more files within a
// torrent. indexes are the file indexes as returned by ListTorrentFiles.
// priority follows qBittorrent's values: 0 = do not download, 1 = normal,
// 6 = high, 7 = maximum.
func (qb *Client) SetFilePriority(hash string, indexes []int, priority int) error {
	ids := make([]string, len(indexes))
	for i, idx := range indexes {
		ids[i] = strconv.Itoa(idx)
	}

	data := url.Values{
		"hash":     {hash},
		"id":       {strings.Join(ids, "|")},
		"priority": {strconv.Itoa(priority)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/filePrio", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set file priority: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set file priority. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) ForceRecheck(hash string) error {
	data := url.Values{
		"hashes": {hash},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/recheck", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	var raw struct {
		Peers map[string]*TorrentPeer `json:"peers"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	peers := make([]*TorrentPeer, 0, len(raw.Peers))
	for _, peer := range raw.Peers {
		peers = append(peers, peer)
	}

	sort.Slice(peers, func(i, j int) bool {
		if peers[i].IP != peers[j].IP {
			return peers[i].IP < peers[j].IP
		}
		return peers[i].Port < peers[j].Port
	})

	return peers, nil
}

// GetGlobalSettings gets qBittorrent global settings
// BanPeers bans peers instance-wide, given as "ip:port"
func (qb *Client) BanPeers(peers []string) error {
	data := url.Values{
		"peers": {strings.Join(peers, "|")},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/transfer/banPeers", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to ban peers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to ban peers. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

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

	// The API requires application/x-www-form-urlencoded with a 'json' field
	data := url.Values{
		"json": {string(jsonData)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/app/setPreferences", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

// GetTags returns all tags registered on the server, including tags that
// are not currently attached to any torrent.
func (qb *Client) GetTags() ([]string, error) {
	endpoint := fmt.Sprintf("%s/api/v2/torrents/tags", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tags. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var tags []string
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return tags, nil
}

// validateTagNames rejects tag names that would corrupt the comma-separated
// "tags" field CreateTags and DeleteTags send to qBittorrent: a comma inside
// a name is indistinguishable from a separator between two names, so e.g.
// "movies,4k" sent as a single tag would silently become two.
func validateTagNames(tags []string) error {
	for _, tag := range tags {
		if strings.Contains(tag, ",") {
			return fmt.Errorf("invalid tag name %q: tag names must not contain a comma", tag)
		}
	}
	return nil
}

// CreateTags creates one or more tags on the server without attaching them
// to any torrent. A tag name may not contain a comma, since tags are sent
// to qBittorrent as a single comma-separated field.
func (qb *Client) CreateTags(tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	if err := validateTagNames(tags); err != nil {
		return err
	}

	data := url.Values{
		"tags": {strings.Join(tags, ",")},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/createTags", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to create tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create tags. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

// DeleteTags removes one or more tags from the server, detaching them from
// every torrent that carries them. Deleting a tag that does not exist is a
// no-op on qBittorrent's side.
func (qb *Client) DeleteTags(tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	if err := validateTagNames(tags); err != nil {
		return err
	}

	data := url.Values{
		"tags": {strings.Join(tags, ",")},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/torrents/deleteTags", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete tags. Status: %d, Response: %s", resp.StatusCode, body)
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

// GetPeerLogs gets peer logs
func (qb *Client) GetPeerLogs(lastKnownID int) ([]*PeerLogEntry, error) {
	params := url.Values{}
	params.Add("last_known_id", fmt.Sprintf("%d", lastKnownID))

	endpoint := fmt.Sprintf("%s/api/v2/log/peers?%s", qb.config.BaseURL, params.Encode())

	resp, err := qb.doWithRetry(http.MethodGet, endpoint, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer logs: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get peer logs. Status: %d, Response: %s", resp.StatusCode, string(body))
	}

	var logs []*PeerLogEntry
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

// SetGlobalDownloadSpeedLimit sets the download speed limit
func (qb *Client) SetGlobalDownloadSpeedLimit(limit int) error {
	data := url.Values{
		"limit": {fmt.Sprintf("%d", limit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/transfer/setDownloadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

// SetGlobalUploadSpeedLimit sets the upload speed limit
func (qb *Client) SetGlobalUploadSpeedLimit(limit int) error {
	data := url.Values{
		"limit": {fmt.Sprintf("%d", limit)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/transfer/setUploadLimit", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

// SetAlternativeRateLimits sets alternative global download and upload speed limits
func (qb *Client) SetAlternativeRateLimits(downloadLimit, uploadLimit int) error {
	// Create a map with only the fields to update
	updates := map[string]interface{}{
		"alt_dl_limit": downloadLimit,
		"alt_up_limit": uploadLimit,
	}

	jsonData, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// The API requires application/x-www-form-urlencoded with a 'json' field
	data := url.Values{
		"json": {string(jsonData)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/app/setPreferences", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set alternative rate limits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set alternative rate limits. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
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

// SetMaxActiveTorrentLimits sets all maximum active torrent limits at once
func (qb *Client) SetMaxActiveTorrentLimits(maxDownloads, maxUploads, maxTorrents, maxChecking int) error {
	// Create a map with only the fields to update
	updates := map[string]interface{}{
		"max_active_downloads":         maxDownloads,
		"max_active_uploads":           maxUploads,
		"max_active_torrents":          maxTorrents,
		"max_active_checking_torrents": maxChecking,
	}

	jsonData, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// The API requires application/x-www-form-urlencoded with a 'json' field
	data := url.Values{
		"json": {string(jsonData)},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	endpoint := fmt.Sprintf("%s/api/v2/app/setPreferences", qb.config.BaseURL)

	resp, err := qb.doWithRetry(http.MethodPost, endpoint, []byte(data.Encode()), headers)
	if err != nil {
		return fmt.Errorf("failed to set max active torrent limits: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set max active torrent limits. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}
