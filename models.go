package qbt

import (
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

// Client is a high-level qBittorrent API client with cookie cache and retries.
type Client struct {
	mu              sync.RWMutex
	config          Config
	client          *http.Client
	MaxLoginRetries int
	RetryDelay      time.Duration

	// Internal enhancements for cookies and retries
	cookieCache   *CookieCache
	retryConfig   *RetryConfig
	lastLoginTime time.Time
	cookieValid   bool
	cookieValidMu sync.RWMutex
}

// Config contains runtime client settings and credentials.
type Config struct {
	BaseURL        string
	Username       string
	Password       string
	jar            *cookiejar.Jar
	RequestTimeout time.Duration
	MaxRetries     int
	RetryBackoff   time.Duration
}

// CookieCache stores session cookies to reduce validation requests.
type CookieCache struct {
	mu         sync.RWMutex
	cookies    map[string]*http.Cookie
	expiryTime time.Time
	lastUsed   time.Time
}

// RetryConfig configures retry behavior and backoff parameters.
type RetryConfig struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	BackoffFactor  float64
	RetryableCodes []int
}

// ListOptions filters listing endpoints.
type ListOptions struct {
	Category string
}

// ListFilter is deprecated; use ListOptions instead.
type ListFilter struct {
	Category string
}

// TorrentConfig configures new torrent creation.
type TorrentConfig struct {
	MagnetURI    string
	Directory    string
	Category     string
	Paused       bool
	SkipChecking bool
}

// TorrentResponse is a subset of torrent info returned by qBittorrent.
type TorrentResponse struct {
	AddedOn       int     `json:"added_on"`
	Category      string  `json:"category"`
	CompletionOn  int64   `json:"completion_on"`
	Dlspeed       int     `json:"dlspeed"`
	Downloaded    int     `json:"downloaded"`
	Eta           int     `json:"eta"`
	ForceStart    bool    `json:"force_start"`
	Hash          string  `json:"hash"`
	InfoHashV1    string  `json:"infohash_v1"`
	InfoHashV2    string  `json:"infohash_v2"`
	MagnetURI     string  `json:"magnet_uri"`
	Name          string  `json:"name"`
	NumComplete   int     `json:"num_complete"`
	NumIncomplete int     `json:"num_incomplete"`
	NumLeechs     int     `json:"num_leechs"`
	NumSeeds      int     `json:"num_seeds"`
	Priority      int     `json:"priority"`
	Progress      float64 `json:"progress"`
	Ratio         float64 `json:"ratio"`
	SavePath      string  `json:"save_path"`
	SeqDl         bool    `json:"seq_dl"`
	Size          int     `json:"size"`
	State         string  `json:"state"`
	SuperSeeding  bool    `json:"super_seeding"`
	Upspeed       int     `json:"upspeed"`
	Uploaded      int     `json:"uploaded"`
	Tags          string  `json:"tags"`
}

// MainDataResponse represents a subset of sync/maindata response.
type MainDataResponse struct {
	ServerState MainDataServerStateResponse `json:"server_state"`
}

// MainDataServerStateResponse contains server metrics.
type MainDataServerStateResponse struct {
	FreeSpaceOnDisk int `json:"free_space_on_disk"`
}

// TransferInfoResponse represents global transfer information.
type TransferInfoResponse struct {
	DlInfoSpeed      int    `json:"dl_info_speed"`
	DlInfoData       int    `json:"dl_info_data"`
	UpInfoSpeed      int    `json:"up_info_speed"`
	UpInfoData       int    `json:"up_info_data"`
	DlRateLimit      int    `json:"dl_rate_limit"`
	UpRateLimit      int    `json:"up_rate_limit"`
	DhtNodes         int    `json:"dht_nodes"`
	ConnectionStatus string `json:"connection_status"`
}
