/*
Package qbt provides a high-level, production-ready client for the qBittorrent Web API.

Highlights:
  - Smart cookie/session management with in-memory cache and periodic cleanup
  - Automatic retries with exponential backoff for transient failures
  - Configurable timeouts and retry policies
  - Clean, well-typed models for common endpoints

Quick start:

	import (
	    "log"
	    qbt "github.com/jfxdev/go-qbt"
	)

	func main() {
	    client, err := qbt.New(qbt.Config{
	        BaseURL:        "http://localhost:8080",
	        Username:       "admin",
	        Password:       "password",
	    })
	    if err != nil {
	        log.Fatal(err)
	    }
	    defer client.Close()

	    // List all torrents
	    _, _ = client.ListTorrents(qbt.ListOptions{})
	}
*/
package qbt
