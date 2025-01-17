package qbt

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

func New(config Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating cookie jar: %w", err)
	}

	return &Client{
		config:          config,
		client:          &http.Client{Jar: jar},
		MaxLoginRetries: 3,
		RetryDelay:      2 * time.Second,
	}, nil
}

func (qb *Client) Update(config Config) {
	qb.mu.Lock()
	qb.config = config
	qb.mu.Unlock()
}

func (qb *Client) sendRequest(method, endpoint string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", qb.config.BaseURL, endpoint)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := qb.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}

	return resp, nil
}

func (qb *Client) login() error {
	data := url.Values{
		"username": {qb.config.Username},
		"password": {qb.config.Password},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := qb.sendRequest("POST", "/api/v2/auth/login", strings.NewReader(data.Encode()), headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}

func (qb *Client) ensureLogin() error {
	for retries := 0; retries < qb.MaxLoginRetries; retries++ {
		if qb.isCookieValid() {
			return nil
		}

		log.Println("Cookie is invalid. Attempting login...")
		if err := qb.login(); err != nil {
			if retries < qb.MaxLoginRetries-1 {
				time.Sleep(qb.RetryDelay << retries) // Exponential backoff
			} else {
				return fmt.Errorf("login failed after %d attempts: %w", qb.MaxLoginRetries, err)
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("failed to ensure login")
}

func (qb *Client) isCookieValid() bool {
	resp, err := qb.sendRequest("GET", "/api/v2/app/version", nil, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		return false
	}
	defer resp.Body.Close()
	return true
}

func (qb *Client) Close() error {
	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := qb.sendRequest("POST", "/api/v2/auth/logout", nil, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("logout failed. Status: %d, Response: %s", resp.StatusCode, body)
	}

	return nil
}
