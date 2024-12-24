package qbt

import (
	"net/http"
	"time"
)

type Client struct {
	BaseURL         string
	Username        string
	Password        string
	client          *http.Client
	MaxLoginRetries int
	RetryDelay      time.Duration
}

type ListOptions struct {
	Category string
}

type ListFilter struct {
	Category string
}

type TorrentConfig struct {
	Source       string
	Directory    string
	Category     string
	Paused       bool
	SkipChecking bool
}

type Torrent struct {
	AddedOn       int     `json:"added_on"`
	Category      string  `json:"category"`
	CompletionOn  int64   `json:"completion_on"`
	Dlspeed       int     `json:"dlspeed"`
	Eta           int     `json:"eta"`
	ForceStart    bool    `json:"force_start"`
	Hash          string  `json:"hash"`
	InfoHashV1    string  `json:"infohash_v1"`
	InfoHashV2    string  `json:"infohash_v2"`
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
}
