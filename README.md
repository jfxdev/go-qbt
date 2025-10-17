# go-qbt - Optimized Go client for qBittorrent

[![Go Reference](https://pkg.go.dev/badge/github.com/jfxdev/go-qbt.svg)](https://pkg.go.dev/github.com/jfxdev/go-qbt)
[![Go Report Card](https://goreportcard.com/badge/github.com/jfxdev/go-qbt)](https://goreportcard.com/report/github.com/jfxdev/go-qbt)
[![Build](https://github.com/jfxdev/go-qbt/actions/workflows/ci.yml/badge.svg)](https://github.com/jfxdev/go-qbt/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/jfxdev/go-qbt)](https://github.com/jfxdev/go-qbt/blob/main/apps/backend/modules/go-qbt/go.mod)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Latest Tag](https://img.shields.io/github/v/tag/jfxdev/go-qbt?label=release)](https://github.com/jfxdev/go-qbt/tags)

A high-performance Go client for the qBittorrent Web API with advanced optimizations for cookies and retries.

## 🚀 Key Improvements

### 1. **Smart Cookie Management**
- **Cookie cache**: Avoids unnecessary validation requests
- **Auto expiration**: Cookies are automatically cleared after 24 hours
- **Optimized validation**: Verify cookies only when needed
- **Periodic cleanup**: Dedicated goroutine to clear expired cookies

### 2. **Advanced Retry System**
- **Exponential backoff**: Increasing delay between attempts
- **Flexible configuration**: Customizable number of retries and delays
- **Smart retry**: Only for retryable status codes (408, 429, 500, 502, 503, 504)
- **Detailed logging**: Information about attempts and failures

### 3. **Performance Optimizations**
- **Configurable timeouts**: Per operation and global
- **Context with timeout**: Granular control of operations
- **Optimized mutexes**: RWMutex for better concurrency
- **Resource management**: Automatic cleanup and memory control

## 📦 Installation

```bash
go get github.com/jfxdev/go-qbt
```

## 🔧 Configuration

```go
config := qbt.Config{
    BaseURL:        "http://localhost:8080",
    Username:       "admin",
    Password:       "password",
    RequestTimeout: 45 * time.Second,  // Custom timeout
    MaxRetries:     5,                 // Number of attempts
    RetryBackoff:   2 * time.Second,   // Base delay between attempts
    Debug:          false,             // Enable debug logging (default: false)
}
```

### Debug Mode

Enable debug logging to see detailed information about:
- Login attempts and success
- Cookie expiration events
- Retry attempts with delays
- Operation failures and retries

```go
config := qbt.Config{
    BaseURL:  "http://localhost:8080",
    Username: "admin",
    Password: "password",
    Debug:    true,  // Enable verbose logging
}
```

**Note:** In production environments, keep `Debug: false` to avoid excessive logging.

## 💻 Basic Usage

```go
// Create client
client, err := qbt.New(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// List torrents (automatic retry and cookie management)
torrents, err := client.ListTorrents(qbt.ListOptions{})
if err != nil {
    log.Printf("Error: %v", err)
}

// Add torrent via magnet link
err = client.AddTorrentLink(qbt.TorrentConfig{
    MagnetURI: "magnet:?xt=urn:btih:...",
    Directory: "/downloads",
    Category:  "movies",
    Paused:    false,
})
if err != nil {
    log.Printf("Error adding torrent: %v", err)
}
```

## 🚀 Available Operations

### Torrent Management
- `ListTorrents(opts ListOptions)` - List all torrents with optional filtering
- `AddTorrentLink(opts TorrentConfig)` - Add a torrent via magnet link
- `PauseTorrents(hash string)` - Pause specific torrent
- `ResumeTorrents(hash string)` - Resume specific torrent
- `DeleteTorrents(hash string, deleteFiles bool)` - Delete torrent with optional file deletion
- `IncreaseTorrentsPriority(hash string)` - Increase torrent priority
- `DecreaseTorrentsPriority(hash string)` - Decrease torrent priority
- `AddTorrentTags(hash string, tags []string)` - Add tags to torrent
- `DeleteTorrentTags(hash string, tags []string)` - Remove tags from torrent
- `SetCategory(hash string, category string)` - Set torrent category
- `RemoveCategory(hash string)` - Remove torrent category
- `GetTorrent(hash string)` - Get specific torrent information
- `GetTorrentProperties(hash string)` - Get detailed torrent properties
- `GetTorrentFiles(hash string)` - Get torrent file list
- `GetTorrentTrackers(hash string)` - Get torrent tracker information
- `GetTorrentPeers(hash string)` - Get torrent peer information
- `ForceRecheck(hash string)` - Force torrent recheck
- `ForceReannounce(hash string)` - Force torrent reannounce
- `ForceStart(hash string)` - Force start torrent
- `SetTorrentDownloadLimit(hash string, limit int)` - Set torrent download speed limit
- `SetTorrentUploadLimit(hash string, limit int)` - Set torrent upload speed limit
- `SetTorrentShareLimit(hash string, ratioLimit float64, seedingTimeLimit int)` - Set torrent share limits

### Categories Management
- `GetCategories()` - Get all categories
- `CreateCategory(name, savePath string)` - Create new category
- `DeleteCategory(name string)` - Delete category

### Global Settings & Configuration
- `GetGlobalSettings()` - Get global qBittorrent settings
- `SetGlobalSettings(settings GlobalSettings)` - Set global qBittorrent settings
- `SetDownloadSpeedLimit(limit int)` - Set global download speed limit
- `SetUploadSpeedLimit(limit int)` - Set global upload speed limit
- `ToggleSpeedLimits()` - Toggle speed limits mode

### Maximum Active Torrent Management
- `SetMaxActiveDownloads(maxDownloads int)` - Set maximum number of active downloads
- `SetMaxActiveUploads(maxUploads int)` - Set maximum number of active uploads
- `SetMaxActiveTorrents(maxTorrents int)` - Set maximum number of active torrents
- `SetMaxActiveCheckingTorrents(maxChecking int)` - Set maximum number of active checking torrents
- `SetMaxActiveTorrentLimits(maxDownloads, maxUploads, maxTorrents, maxChecking int)` - Set all maximum active torrent limits at once
- `GetMaxActiveDownloads()` - Get current maximum number of active downloads
- `GetMaxActiveUploads()` - Get current maximum number of active uploads
- `GetMaxActiveTorrents()` - Get current maximum number of active torrents
- `GetMaxActiveCheckingTorrents()` - Get current maximum number of active checking torrents

### System Information & Monitoring
- `GetMainData()` - Get main server data and sync information
- `GetTransferInfo()` - Get transfer statistics and information
- `GetNetworkInfo()` - Get network information
- `GetAppVersion()` - Get qBittorrent application version
- `GetAPIVersion()` - Get Web API version
- `GetBuildInfo()` - Get build information
- `GetLogs(normal, info, warning, critical bool, lastKnownID int)` - Get system logs

### RSS Feeds Management
- `GetRSSFeeds(withData bool)` - Get RSS feeds
- `AddRSSFeed(url, path string)` - Add RSS feed
- `RemoveRSSFeed(path string)` - Remove RSS feed

## 🌱 Essential Features for Seedbox

This SDK has been specially optimized for seedbox usage, including essential features for daily management:

### 📊 Advanced Monitoring
- **Trackers**: Monitor status and performance of all trackers
- **Peers**: Track connections, speeds and countries of peers
- **Logs**: Access detailed system logs for debugging
- **Network**: Monitor network information and DHT connections

### ⚙️ Speed Control
- **Global Limits**: Configure speed limits for download/upload
- **Per-Torrent Limits**: Individual speed control per torrent
- **Ratio Management**: Configure ratio limits and seeding time
- **Toggle Limits**: Quickly enable/disable speed limits

### 🗂️ Organization and Categorization
- **Categories**: Create and manage categories to organize torrents
- **Tags**: Add and remove tags for better organization
- **Paths**: Configure specific paths per category

### 🔧 Advanced Settings
- **Global Settings**: Access and modify all qBittorrent settings
- **RSS Feeds**: Configure RSS feeds for download automation
- **Recheck/Reannounce**: Force checks and announcements when needed

### 💡 Typical Seedbox Use Cases
```go
// Monitor tracker performance
trackers, err := client.GetTorrentTrackers("torrent_hash")
if err != nil {
    log.Printf("Error getting trackers: %v", err)
}

// Configure ratio limit for seeding
err = client.SetTorrentShareLimit("torrent_hash", 2.0, 168) // 2.0 ratio, 168 hours
if err != nil {
    log.Printf("Error configuring ratio: %v", err)
}

// Get system logs for debugging
logs, err := client.GetLogs(true, true, true, true, 0)
if err != nil {
    log.Printf("Error getting logs: %v", err)
}

// Configure category for organization
err = client.CreateCategory("movies", "/downloads/movies")
if err != nil {
    log.Printf("Error creating category: %v", err)
}
```

## ⚙️ Advanced Settings

### Timeouts
```go
// Global timeout for all operations
config.RequestTimeout = 60 * time.Second
```

### Retries
```go
// Retry configuration
config.MaxRetries = 10            // Max attempts
config.RetryBackoff = 1 * time.Second  // Base delay
```

### Cookies
```go
// Cookie settings are automatic:
// - Expiration: 24 hours
// - Check: every 5 minutes
// - Cache: Smart with automatic invalidation
```

## 🔍 Monitoring and Logs

When debug mode is enabled (`Debug: true`), the client provides detailed logs for:
- Login attempts and success
- Failures and retries with attempt counts
- Cookie expiration events
- Successful operations after retries

**Example debug output:**
```
Login successful, cookies cached
GET /api/v2/torrents/info failed (attempt 1/3), retrying in 2s: authentication error: status code 403
Login successful, cookies cached
GET /api/v2/torrents/info succeeded after 1 retries
Cookies expired, cleared from cache
```

## 📊 Performance Metrics

- **Cache hit rate**: Cookie cache effectiveness
- **Retry statistics**: Attempts and failures
- **Response times**: Per operation

## 🧪 Examples

### Maximum Active Torrent Management Example
```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/jfxdev/go-qbt"
)

func main() {
    config := qbt.Config{
        BaseURL:        "http://localhost:8080",
        Username:       "admin",
        Password:       "password",
        RequestTimeout: 30 * time.Second,
        MaxRetries:     3,
        RetryBackoff:   2 * time.Second,
        Debug:          false,
    }
    
    client, err := qbt.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Get current maximum active torrent limits
    maxDownloads, _ := client.GetMaxActiveDownloads()
    maxUploads, _ := client.GetMaxActiveUploads()
    maxTorrents, _ := client.GetMaxActiveTorrents()
    maxChecking, _ := client.GetMaxActiveCheckingTorrents()
    
    fmt.Printf("Current limits - Downloads: %d, Uploads: %d, Torrents: %d, Checking: %d\n",
        maxDownloads, maxUploads, maxTorrents, maxChecking)
    
    // Set individual limits
    err = client.SetMaxActiveDownloads(5)
    if err != nil {
        log.Printf("Error setting max downloads: %v", err)
    }
    
    err = client.SetMaxActiveUploads(3)
    if err != nil {
        log.Printf("Error setting max uploads: %v", err)
    }
    
    // Set all limits at once
    err = client.SetMaxActiveTorrentLimits(8, 4, 15, 2)
    if err != nil {
        log.Printf("Error setting all limits: %v", err)
    } else {
        fmt.Println("Successfully set all maximum active torrent limits")
    }
}
```

### Complete Usage Example
```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/jfxdev/go-qbt"
)

func main() {
    // Configure client
    config := qbt.Config{
        BaseURL:        "http://localhost:8080",
        Username:       "admin",
        Password:       "password",
        RequestTimeout: 30 * time.Second,
        MaxRetries:     3,
        RetryBackoff:   2 * time.Second,
        Debug:          false, // Set to true for verbose logging
    }
    
    // Create client
    client, err := qbt.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Get system information
    version, err := client.GetAppVersion()
    if err != nil {
        log.Printf("Error getting version: %v", err)
    } else {
        fmt.Printf("qBittorrent version: %s\n", version)
    }
    
    // List torrents
    torrents, err := client.ListTorrents(qbt.ListOptions{})
    if err != nil {
        log.Printf("Error listing torrents: %v", err)
        return
    }
    
    fmt.Printf("Found %d torrents\n", len(torrents))
    
    // Add a new torrent
    err = client.AddTorrentLink(qbt.TorrentConfig{
        MagnetURI: "magnet:?xt=urn:btih:...",
        Directory: "/downloads",
        Category:  "movies",
        Paused:    false,
    })
    if err != nil {
        log.Printf("Error adding torrent: %v", err)
    }
    
    // Get transfer info
    info, err := client.GetTransferInfo()
    if err != nil {
        log.Printf("Error getting transfer info: %v", err)
    } else {
        fmt.Printf("Download speed: %d bytes/s\n", info.DlSpeed)
        fmt.Printf("Upload speed: %d bytes/s\n", info.UpSpeed)
    }
}
```

## 🔒 Security

- **Secure cookies**: Safe session management
- **Timeouts**: Prevents hanging operations
- **Validation**: Automatic credential verification

## 🚨 Error Handling

The client implements robust error handling:
- **Automatic retry**: For temporary failures with exponential backoff
- **Graceful fallback**: Elegant degradation on errors
- **Smart cookie management**: Automatic re-authentication when needed
- **Session expiration handling**: Detects 401/403 errors and automatically re-authenticates
- **Timeout protection**: Prevents hanging operations

### Session Expiration Fix

The client now properly handles qBittorrent session timeouts. When the server returns a 403 (Forbidden) or 401 (Unauthorized) error due to an expired session:

1. The client automatically **invalidates the cached cookies**
2. The retry mechanism **forces a new login** on the next attempt
3. The operation is **retried seamlessly** without user intervention

This fixes the issue where, after several hours, the client would continuously return "forbidden" errors because the qBittorrent Web UI session had expired (configured via `WebUISessionTimeout` in qBittorrent settings), while the client still considered its cached cookies as valid.

## 📈 Benefits of the Improvements

1. **Lower latency**: Cookie cache avoids re-authentication
2. **Higher reliability**: Automatic retry on temporary failures
3. **Better performance**: Fewer unnecessary requests
4. **Scalability**: Supports multiple concurrent operations
5. **Maintainability**: Cleaner and more organized code

## 🤝 Contributing

Contributions are welcome! Please open an issue or pull request.

## 📄 License

This project is licensed under the GNU General Public License v3.0 - see [LICENSE](LICENSE) for details.