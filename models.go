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
	Debug          bool // Enable debug logging for session management
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
	AddedOn       int         `json:"added_on"`
	Category      string      `json:"category"`
	CompletionOn  int64       `json:"completion_on"`
	Dlspeed       int         `json:"dlspeed"`
	Downloaded    int         `json:"downloaded"`
	Eta           int         `json:"eta"`
	ForceStart    bool        `json:"force_start"`
	Hash          string      `json:"hash"`
	InfoHashV1    string      `json:"infohash_v1"`
	InfoHashV2    string      `json:"infohash_v2"`
	MagnetURI     string      `json:"magnet_uri"`
	MagnetLink    *MagnetLink `json:"magnet_link"`
	Name          string      `json:"name"`
	NumComplete   int         `json:"num_complete"`
	NumIncomplete int         `json:"num_incomplete"`
	NumLeechs     int         `json:"num_leechs"`
	NumSeeds      int         `json:"num_seeds"`
	Popularity    float64     `json:"popularity"`
	Priority      int         `json:"priority"`
	Progress      float64     `json:"progress"`
	Ratio         float64     `json:"ratio"`
	SavePath      string      `json:"save_path"`
	SeqDl         bool        `json:"seq_dl"`
	Size          int         `json:"size"`
	State         string      `json:"state"`
	SuperSeeding  bool        `json:"super_seeding"`
	Upspeed       int         `json:"upspeed"`
	Uploaded      int         `json:"uploaded"`
	Tags          string      `json:"tags"`
}

// MainDataResponse represents a subset of sync/maindata response.
type MainDataResponse struct {
	ServerState MainDataServerStateResponse `json:"server_state"`
}

// MainDataServerStateResponse contains server metrics.
type MainDataServerStateResponse struct {
	FreeSpaceOnDisk       int    `json:"free_space_on_disk"`
	AllTimeDownloaded     int    `json:"alltime_dl"`
	AllTimeUploaded       int    `json:"alltime_ul"`
	ConnectionStatus      string `json:"connection_status"`
	GlobalRatio           string `json:"global_ratio"`
	LastExternalAddressV4 string `json:"last_external_address_v4"`
	LastExternalAddressV6 string `json:"last_external_address_v6"`
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

type BuildInfoResponse struct {
	DlInfoSpeed      int    `json:"dl_info_speed"`
	DlInfoData       int    `json:"dl_info_data"`
	UpInfoSpeed      int    `json:"up_info_speed"`
	UpInfoData       int    `json:"up_info_data"`
	DlRateLimit      int    `json:"dl_rate_limit"`
	UpRateLimit      int    `json:"up_rate_limit"`
	DhtNodes         int    `json:"dht_nodes"`
	ConnectionStatus string `json:"connection_status"`
}

// MagnetLink represents the data extracted from a magnet link
type MagnetLink struct {
	Hash             string   `json:"hash"`              // Hash of the torrent (btih)
	DisplayName      string   `json:"display_name"`      // File/torrent name (dn)
	Trackers         []string `json:"trackers"`          // List of trackers (tr)
	ExactLength      string   `json:"exact_length"`      // Exact length (xl)
	ExactSource      string   `json:"exact_source"`      // Exact source (xs)
	Keywords         string   `json:"keywords"`          // Keywords (kt)
	AcceptableSource string   `json:"acceptable_source"` // Acceptable source (as)
}

// TorrentFile represents a file within a torrent
type TorrentFile struct {
	Name         string  `json:"name"`         // File name
	Size         int64   `json:"size"`         // File size in bytes
	Progress     float64 `json:"progress"`     // Download progress (0.0 to 1.0)
	Priority     int     `json:"priority"`     // File priority
	IsSeed       bool    `json:"is_seed"`      // Whether the file is seeded
	PieceRange   [2]int  `json:"piece_range"`  // Piece range [start, end]
	Availability float64 `json:"availability"` // File availability (0.0 to 1.0)
}

// TorrentProperties represents detailed properties of a torrent
type TorrentProperties struct {
	SavePath           string  `json:"save_path"`            // Save path
	CreationDate       int64   `json:"creation_date"`        // Creation date
	PieceSize          int64   `json:"piece_size"`           // Piece size
	Comment            string  `json:"comment"`              // Comment
	TotalWasted        int64   `json:"total_wasted"`         // Total wasted
	TotalUploaded      int64   `json:"total_uploaded"`       // Total uploaded
	TotalDownloaded    int64   `json:"total_downloaded"`     // Total downloaded
	UpLimit            int     `json:"up_limit"`             // Upload limit
	DlLimit            int     `json:"dl_limit"`             // Download limit
	TimeElapsed        int     `json:"time_elapsed"`         // Time elapsed
	SeedingTime        int     `json:"seeding_time"`         // Seeding time
	NbConnections      int     `json:"nb_connections"`       // Number of connections
	NbConnectionsLimit int     `json:"nb_connections_limit"` // Number of connections limit
	ShareRatio         float64 `json:"share_ratio"`          // Share ratio
	AdditionDate       int64   `json:"addition_date"`        // Addition date
	CompletionDate     int64   `json:"completion_date"`      // Completion date
	CreatedBy          string  `json:"created_by"`           // Created by
	DlSpeedAvg         int     `json:"dl_speed_avg"`         // Download speed average
	DlSpeed            int     `json:"dl_speed"`             // Download speed
	Eta                int     `json:"eta"`                  // ETA
	LastSeen           int     `json:"last_seen"`            // Last seen
	Peers              int     `json:"peers"`                // Peers
	PeersTotal         int     `json:"peers_total"`          // Total peers
	PiecesHave         int     `json:"pieces_have"`          // Pieces have
	PiecesNum          int     `json:"pieces_num"`           // Total pieces
	Reannounce         int     `json:"reannounce"`           // Reannounce
	Seeds              int     `json:"seeds"`                // Seeds
	SeedsTotal         int     `json:"seeds_total"`          // Total seeds
	ShareLimit         int     `json:"share_limit"`          // Share limit
	UpSpeedAvg         int     `json:"up_speed_avg"`         // Upload speed average
	UpSpeed            int     `json:"up_speed"`             // Upload speed
}

// ===== STRUCTURES FOR SEEDBOX FUNCTIONALITIES =====

// TorrentTracker represents tracker information
type TorrentTracker struct {
	URL           string `json:"url"`            // Tracker URL
	Status        int    `json:"status"`         // Tracker status
	Tier          int    `json:"tier"`           // Tracker tier
	NumPeers      int    `json:"num_peers"`      // Number of peers
	NumSeeds      int    `json:"num_seeds"`      // Number of seeds
	NumLeeches    int    `json:"num_leeches"`    // Number of leeches
	NumDownloaded int    `json:"num_downloaded"` // Number of downloads
	Msg           string `json:"msg"`            // Tracker message
}

// TorrentPeer represents peer information
type TorrentPeer struct {
	IP            string  `json:"ip"`           // Peer IP address
	Port          int     `json:"port"`         // Peer port
	Client        string  `json:"client"`       // Client name
	Flags         string  `json:"flags"`        // Peer flags
	FlagsDesc     string  `json:"flags_desc"`   // Flags description
	Connection    string  `json:"connection"`   // Connection type
	Country       string  `json:"country"`      // Country code
	CountryCode   string  `json:"country_code"` // Country code
	Downloaded    int64   `json:"downloaded"`   // Downloaded bytes
	DownloadSpeed int     `json:"dl_speed"`     // Download speed
	Files         string  `json:"files"`        // Files
	Progress      float64 `json:"progress"`     // Progress (0.0 to 1.0)
	Relevance     int     `json:"relevance"`    // Relevance
	Uploaded      int64   `json:"uploaded"`     // Uploaded bytes
	UploadSpeed   int     `json:"up_speed"`     // Upload speed
}

// GlobalSettings represents qBittorrent global settings
type GlobalSettings struct {
	Locale                             string            `json:"locale"`                                 // Interface language
	CreateSubfolderEnabled             bool              `json:"create_subfolder_enabled"`               // Create subfolder
	StartPausedEnabled                 bool              `json:"start_paused_enabled"`                   // Start paused
	AutoDeleteMode                     int               `json:"auto_delete_mode"`                       // Auto delete mode
	PreallocateAll                     bool              `json:"preallocate_all"`                        // Preallocate all
	IncompleteFilesExt                 bool              `json:"incomplete_files_ext"`                   // Incomplete files extension
	AutoTMMEnabled                     bool              `json:"auto_tmm_enabled"`                       // Automatic torrent management
	TorrentChangedTMMEnabled           bool              `json:"torrent_changed_tmm_enabled"`            // Torrent changed TMM
	SavePathChangedTMMEnabled          bool              `json:"save_path_changed_tmm_enabled"`          // Save path changed TMM
	CategoryChangedTMMEnabled          bool              `json:"category_changed_tmm_enabled"`           // Category changed TMM
	SavePath                           string            `json:"save_path"`                              // Default save path
	TempPathEnabled                    bool              `json:"temp_path_enabled"`                      // Use temp path
	TempPath                           string            `json:"temp_path"`                              // Temp path
	ExportDir                          string            `json:"export_dir"`                             // Export directory
	ExportDirFin                       string            `json:"export_dir_fin"`                         // Export directory finished
	ScanDirs                           map[string]string `json:"scan_dirs"`                              // Scan directories
	MailNotificationEnabled            bool              `json:"mail_notification_enabled"`              // Mail notification
	MailNotificationSender             string            `json:"mail_notification_sender"`               // Mail sender
	MailNotificationEmail              string            `json:"mail_notification_email"`                // Mail email
	MailNotificationSMTP               string            `json:"mail_notification_smtp"`                 // Mail SMTP
	MailNotificationSSLEnabled         bool              `json:"mail_notification_ssl_enabled"`          // Mail SSL
	MailNotificationAuthEnabled        bool              `json:"mail_notification_auth_enabled"`         // Mail auth
	MailNotificationUsername           string            `json:"mail_notification_username"`             // Mail username
	MailNotificationPassword           string            `json:"mail_notification_password"`             // Mail password
	AutorunEnabled                     bool              `json:"autorun_enabled"`                        // Autorun enabled
	AutorunProgram                     string            `json:"autorun_program"`                        // Autorun program
	QueueingEnabled                    bool              `json:"queueing_enabled"`                       // Queueing enabled
	MaxActiveDownloads                 int               `json:"max_active_downloads"`                   // Max active downloads
	MaxActiveUploads                   int               `json:"max_active_uploads"`                     // Max active uploads
	MaxActiveTorrents                  int               `json:"max_active_torrents"`                    // Max active torrents
	DontCountSlowTorrents              bool              `json:"dont_count_slow_torrents"`               // Don't count slow torrents
	SlowTorrentsDownloadRate           int               `json:"slow_torrents_dl_rate_threshold"`        // Slow torrents DL rate
	SlowTorrentsUploadRate             int               `json:"slow_torrents_ul_rate_threshold"`        // Slow torrents UL rate
	SlowTorrentsInactivityTimer        int               `json:"slow_torrents_inactivity_timer"`         // Slow torrents inactivity timer
	MaxRatioEnabled                    bool              `json:"max_ratio_enabled"`                      // Max ratio enabled
	MaxRatio                           float64           `json:"max_ratio"`                              // Max ratio
	MaxRatioAct                        int               `json:"max_ratio_act"`                          // Max ratio action
	ListenPort                         int               `json:"listen_port"`                            // Listen port
	UPnP                               bool              `json:"upnp"`                                   // UPnP
	RandomPort                         bool              `json:"random_port"`                            // Random port
	DownloadPath                       string            `json:"download_path"`                          // Download path
	UseSubcategories                   bool              `json:"use_subcategories"`                      // Use subcategories
	SaveResumeDataInterval             int               `json:"save_resume_data_interval"`              // Save resume data interval
	RecheckCompletedTorrents           bool              `json:"recheck_completed_torrents"`             // Recheck completed torrents
	ResumeDataStorageType              int               `json:"resume_data_storage_type"`               // Resume data storage type
	TorrentExportDirectory             string            `json:"torrent_export_dir"`                     // Torrent export directory
	TorrentExportDirectoryFin          string            `json:"torrent_export_dir_fin"`                 // Torrent export directory finished
	SendBufferWatermark                int               `json:"send_buffer_watermark"`                  // Send buffer watermark
	SendBufferLowWatermark             int               `json:"send_buffer_low_watermark"`              // Send buffer low watermark
	SendBufferWatermarkFactor          int               `json:"send_buffer_watermark_factor"`           // Send buffer watermark factor
	SocketBacklogSize                  int               `json:"socket_backlog_size"`                    // Socket backlog size
	OutgoingPortsMin                   int               `json:"outgoing_ports_min"`                     // Outgoing ports min
	OutgoingPortsMax                   int               `json:"outgoing_ports_max"`                     // Outgoing ports max
	UPnPLeaseDuration                  int               `json:"upnp_lease_duration"`                    // UPnP lease duration
	PeerTOS                            int               `json:"peer_tos"`                               // Peer TOS
	UTPRateLimitEnabled                bool              `json:"utp_rate_limit_enabled"`                 // UTP rate limit enabled
	UTPRateLimit                       int               `json:"utp_rate_limit"`                         // UTP rate limit
	AlternativeGlobalSpeedLimit        int               `json:"alternative_global_speed_limit"`         // Alternative global speed limit
	AlternativeGlobalSpeedLimitEnabled bool              `json:"alternative_global_speed_limit_enabled"` // Alternative global speed limit enabled
	GlobalSpeedLimit                   int               `json:"global_speed_limit"`                     // Global speed limit
	GlobalSpeedLimitEnabled            bool              `json:"global_speed_limit_enabled"`             // Global speed limit enabled
	BittorrentProtocol                 int               `json:"bittorrent_protocol"`                    // BitTorrent protocol
	LimitUTPRate                       bool              `json:"limit_utp_rate"`                         // Limit UTP rate
	LimitTCPOverhead                   bool              `json:"limit_tcp_overhead"`                     // Limit TCP overhead
	LimitLANPeers                      bool              `json:"limit_lan_peers"`                        // Limit LAN peers
	AltGlobalSpeedLimit                int               `json:"alt_global_speed_limit"`                 // Alt global speed limit
	AltGlobalSpeedLimitEnabled         bool              `json:"alt_global_speed_limit_enabled"`         // Alt global speed limit enabled
	GlobalDLSpeedLimit                 int               `json:"global_dl_speed_limit"`                  // Global download speed limit
	GlobalDLSpeedLimitEnabled          bool              `json:"global_dl_speed_limit_enabled"`          // Global download speed limit enabled
	GlobalUPSpeedLimit                 int               `json:"global_up_speed_limit"`                  // Global upload speed limit
	GlobalUPSpeedLimitEnabled          bool              `json:"global_up_speed_limit_enabled"`          // Global upload speed limit enabled
	BypassLocalAuth                    bool              `json:"bypass_local_auth"`                      // Bypass local auth
	BypassAuthSubnetWhitelistEnabled   bool              `json:"bypass_auth_subnet_whitelist_enabled"`   // Bypass auth subnet whitelist enabled
	BypassAuthSubnetWhitelist          string            `json:"bypass_auth_subnet_whitelist"`           // Bypass auth subnet whitelist
	WebUIAddress                       string            `json:"web_ui_address"`                         // Web UI address
	WebUIPort                          int               `json:"web_ui_port"`                            // Web UI port
	WebUIUPnP                          bool              `json:"web_ui_upnp"`                            // Web UI UPnP
	WebUIUsername                      string            `json:"web_ui_username"`                        // Web UI username
	WebUIPassword                      string            `json:"web_ui_password"`                        // Web UI password
	WebUICSRFProtectionEnabled         bool              `json:"web_ui_csrf_protection_enabled"`         // Web UI CSRF protection
	WebUIClickjackingProtectionEnabled bool              `json:"web_ui_clickjacking_protection_enabled"` // Web UI clickjacking protection
	WebUISecureCookieEnabled           bool              `json:"web_ui_secure_cookie_enabled"`           // Web UI secure cookie
	WebUIMaxAuthFailCount              int               `json:"web_ui_max_auth_fail_count"`             // Web UI max auth fail count
	WebUIBanDuration                   int               `json:"web_ui_ban_duration"`                    // Web UI ban duration
	WebUISessionTimeout                int               `json:"web_ui_session_timeout"`                 // Web UI session timeout
	WebUIHostHeaderValidationEnabled   bool              `json:"web_ui_host_header_validation_enabled"`  // Web UI host header validation
	WebUIHTTPSEnabled                  bool              `json:"web_ui_https_enabled"`                   // Web UI HTTPS enabled
	WebUIHTTPSKeyPath                  string            `json:"web_ui_https_key_path"`                  // Web UI HTTPS key path
	WebUIHTTPSCertPath                 string            `json:"web_ui_https_cert_path"`                 // Web UI HTTPS cert path
	DyDNSEnabled                       bool              `json:"dydns_enabled"`                          // Dynamic DNS enabled
	DyDNSService                       int               `json:"dydns_service"`                          // Dynamic DNS service
	DyDNSUsername                      string            `json:"dydns_username"`                         // Dynamic DNS username
	DyDNSPassword                      string            `json:"dydns_password"`                         // Dynamic DNS password
	DyDNSDomain                        string            `json:"dydns_domain"`                           // Dynamic DNS domain
	RSSRefreshInterval                 int               `json:"rss_refresh_interval"`                   // RSS refresh interval
	RSSMaxArticlesPerFeed              int               `json:"rss_max_articles_per_feed"`              // RSS max articles per feed
	RSSProcessingEnabled               bool              `json:"rss_processing_enabled"`                 // RSS processing enabled
	RSSAutoDownloadingEnabled          bool              `json:"rss_auto_downloading_enabled"`           // RSS auto downloading enabled
	RSSDownloadRepackProperEpisodes    bool              `json:"rss_download_repack_proper_episodes"`    // RSS download repack proper episodes
	RSSSmartEpisodeFilter              bool              `json:"rss_smart_episode_filter"`               // RSS smart episode filter
	I2PEnabled                         bool              `json:"i2p_enabled"`                            // I2P enabled
	I2PAddress                         string            `json:"i2p_address"`                            // I2P address
	I2PPort                            int               `json:"i2p_port"`                               // I2P port
	I2PInboundLength                   int               `json:"i2p_inbound_length"`                     // I2P inbound length
	I2POutboundLength                  int               `json:"i2p_outbound_length"`                    // I2P outbound length
	I2PInboundQuantity                 int               `json:"i2p_inbound_quantity"`                   // I2P inbound quantity
	I2POutboundQuantity                int               `json:"i2p_outbound_quantity"`                  // I2P outbound quantity
	I2PInboundVariance                 int               `json:"i2p_inbound_variance"`                   // I2P inbound variance
	I2POutboundVariance                int               `json:"i2p_outbound_variance"`                  // I2P outbound variance
	I2PInboundLengthVariance           int               `json:"i2p_inbound_length_variance"`            // I2P inbound length variance
	I2POutboundLengthVariance          int               `json:"i2p_outbound_length_variance"`           // I2P outbound length variance
}

// Category represents a torrent category
type Category struct {
	Name     string `json:"name"`     // Category name
	SavePath string `json:"savePath"` // Save path for this category
}

// LogEntry represents a log entry
type LogEntry struct {
	ID        int    `json:"id"`        // Log entry ID
	Message   string `json:"message"`   // Log message
	Timestamp int64  `json:"timestamp"` // Timestamp
	Type      int    `json:"type"`      // Log type (normal=1, info=2, warning=4, critical=8)
}

// NetworkInfo represents network information
type NetworkInfo struct {
	ConnectionStatus string `json:"connection_status"` // Connection status
	DhtNodes         int    `json:"dht_nodes"`         // DHT nodes
	DlInfoData       int64  `json:"dl_info_data"`      // Downloaded data
	DlInfoSpeed      int    `json:"dl_info_speed"`     // Download speed
	DlRateLimit      int    `json:"dl_rate_limit"`     // Download rate limit
	UpInfoData       int64  `json:"up_info_data"`      // Uploaded data
	UpInfoSpeed      int    `json:"up_info_speed"`     // Upload speed
	UpRateLimit      int    `json:"up_rate_limit"`     // Upload rate limit
}

// RSSFeed represents an RSS feed
type RSSFeed struct {
	URL       string       `json:"url"`       // Feed URL
	Title     string       `json:"title"`     // Feed title
	LastBuild string       `json:"lastBuild"` // Last build date
	IsLoading bool         `json:"isLoading"` // Is loading
	HasError  bool         `json:"hasError"`  // Has error
	Articles  []RSSArticle `json:"articles"`  // Articles
}

// RSSArticle represents an RSS article
type RSSArticle struct {
	ID          string `json:"id"`          // Article ID
	Title       string `json:"title"`       // Article title
	Summary     string `json:"summary"`     // Article summary
	Link        string `json:"link"`        // Article link
	IsRead      bool   `json:"isRead"`      // Is read
	Date        string `json:"date"`        // Article date
	Description string `json:"description"` // Article description
	TorrentURL  string `json:"torrentURL"`  // Torrent URL
	Size        int64  `json:"size"`        // File size
}
