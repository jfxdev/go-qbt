package main

import (
	"context"
	"fmt"
	"log"
	"time"

	qbt "github.com/jfxdev/go-qbt"
)

func main() {
	// Optimized configuration with custom timeouts and retries
	config := qbt.Config{
		BaseURL:        "http://localhost:8080",
		Username:       "admin",
		Password:       "password",
		RequestTimeout: 45 * time.Second, // Longer timeout for heavy operations
		MaxRetries:     5,                // More attempts for critical operations
		RetryBackoff:   2 * time.Second,  // More aggressive backoff
	}

	// Create optimized client
	client, err := qbt.New(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Example 1: List torrents with automatic retry
	fmt.Println("=== Listing torrents ===")
	torrents, err := client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		log.Printf("Failed to list torrents: %v", err)
	} else {
		fmt.Printf("Found %d torrents\n", len(torrents))
	}

	// Example 2: Add torrent with context and custom timeout
	fmt.Println("\n=== Adding torrent ===")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	torrentConfig := qbt.TorrentConfig{
		MagnetURI:    "magnet:?xt=urn:btih:example",
		Directory:    "/downloads/movies",
		Category:     "movies",
		Paused:       false,
		SkipChecking: true,
	}

	err = client.AddTorrentLinkWithContext(ctx, torrentConfig)
	if err != nil {
		log.Printf("Failed to add torrent: %v", err)
	} else {
		fmt.Println("Torrent added successfully")
	}

	// Example 3: Batch operations with automatic retry
	fmt.Println("\n=== Batch operations ===")

	// Pause multiple torrents
	hash := "example_hash_123"
	err = client.PauseTorrents(hash)
	if err != nil {
		log.Printf("Failed to pause torrents: %v", err)
	} else {
		fmt.Println("Torrents paused successfully")
	}

	// Increase priority
	err = client.IncreaseTorrentsPriority(hash)
	if err != nil {
		log.Printf("Failed to increase priority: %v", err)
	} else {
		fmt.Println("Priority increased successfully")
	}

	// Example 4: Get system information
	fmt.Println("\n=== System information ===")

	// Main data
	mainData, err := client.GetMainData()
	if err != nil {
		log.Printf("Failed to get main data: %v", err)
	} else {
		fmt.Printf("Free disk space: %d bytes\n", mainData.ServerState.FreeSpaceOnDisk)
	}

	// Transfer information
	transferInfo, err := client.GetTransferInfo()
	if err != nil {
		log.Printf("Failed to get transfer information: %v", err)
	} else {
		fmt.Printf("Connection status: %s\n", transferInfo.ConnectionStatus)
		fmt.Printf("Download speed: %d B/s\n", transferInfo.DlInfoSpeed)
		fmt.Printf("Upload speed: %d B/s\n", transferInfo.UpInfoSpeed)
	}

	// Example 5: Cookie cache demonstration
	fmt.Println("\n=== Cookie cache demonstration ===")

	// Perform multiple requests to demonstrate cache
	for i := 0; i < 3; i++ {
		start := time.Now()
		_, err := client.ListTorrents(qbt.ListOptions{})
		duration := time.Since(start)

		if err != nil {
			log.Printf("Request %d failed: %v", i+1, err)
		} else {
			fmt.Printf("Request %d completed in %v\n", i+1, duration)
		}

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n=== Demonstration completed ===")
}
