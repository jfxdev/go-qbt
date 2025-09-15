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
