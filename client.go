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

	"github.com/jfxdev/go-qbt/request"
)

func New(config Config) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating cookie jar: %w", err)
	}

	config.jar = jar

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

func (qb *Client) login() error {
	data := url.Values{
		"username": {qb.config.Username},
		"password": {qb.config.Password},
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, err := request.Do(http.MethodPost,
		fmt.Sprintf("%s/api/v2/auth/login", qb.config.BaseURL),
		request.WithBody(strings.NewReader(data.Encode())),
		request.WithHeaders(headers),
		request.WithCookieJar(qb.config.jar),
		request.WithUpdateCookies(),
	)
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
	resp, err := request.Do(http.MethodGet,
		fmt.Sprintf("%s/api/v2/app/version", qb.config.BaseURL),
		request.WithCookieJar(qb.config.jar),
	)

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

	resp, err := request.Do(http.MethodPost,
		fmt.Sprintf("%s/api/v2/auth/logout", qb.config.BaseURL),
		request.WithCookieJar(qb.config.jar),
		request.WithHeaders(headers),
	)
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
