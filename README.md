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
}
```

## 💻 Basic Usage

```go
// Create client
client, err := qbt.New(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// List torrents (automatic retry)
torrents, err := client.ListTorrents(qbt.ListOptions{})
if err != nil {
    log.Printf("Error: %v", err)
}
```

## 🔄 Usage with Context

```go
// Context with custom timeout
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

// Add torrent with context
err := client.AddTorrentLinkWithContext(ctx, torrentConfig)
```

## 🚀 Available Operations

### Torrent Management
- `ListTorrents()` - List all torrents
- `AddTorrentLink()` - Add a torrent via magnet link
- `PauseTorrents()` - Pause torrents
- `ResumeTorrents()` - Resume torrents
- `DeleteTorrents()` - Delete torrents
- `IncreaseTorrentsPriority()` - Increase priority
- `DecreaseTorrentsPriority()` - Decrease priority
- `AddTorrentTags()` - Add tags

### System Information
- `GetMainData()` - Main server data
- `GetTransferInfo()` - Transfer information

## ⚙️ Advanced Settings

### Timeouts
```go
// Global timeout
config.RequestTimeout = 60 * time.Second

// Per-operation timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

The client provides detailed logs for:
- Login attempts
- Failures and retries
- Cookie expiration
- Successful operations

## 📊 Performance Metrics

- **Cache hit rate**: Cookie cache effectiveness
- **Retry statistics**: Attempts and failures
- **Response times**: Per operation

## 🧪 Examples

See the `examples/` directory for complete usage examples:
- Basic usage
- Batch operations
- Worker pools
- Advanced settings

## 🔒 Security

- **Secure cookies**: Safe session management
- **Timeouts**: Prevents hanging operations
- **Validation**: Automatic credential verification

## 🚨 Error Handling

The client implements robust error handling:
- **Automatic retry**: For temporary failures
- **Graceful fallback**: Elegant degradation on errors
- **Context cancellation**: Supports operation cancellation

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