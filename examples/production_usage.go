package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jfxdev/go-qbt"
)

// This example demonstrates production usage without debug logging.
// All session management happens silently in the background.

func main() {
	config := qbt.Config{
		BaseURL:        "http://localhost:8080",
		Username:       "admin",
		Password:       "adminpass",
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   2 * time.Second,
		Debug:          false, // Disabled for production - no verbose logging
	}

	client, err := qbt.New(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("✓ Client initialized")

	// Get application version
	version, err := client.GetAppVersion()
	if err != nil {
		log.Fatalf("Error getting version: %v", err)
	}
	fmt.Printf("✓ qBittorrent version: %s\n", version)

	// List torrents
	torrents, err := client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing torrents: %v", err)
	}
	fmt.Printf("✓ Found %d torrents\n", len(torrents))

	// Get transfer info
	info, err := client.GetTransferInfo()
	if err != nil {
		log.Fatalf("Error getting transfer info: %v", err)
	}
	fmt.Printf("✓ Download speed: %d bytes/s\n", info.DlInfoSpeed)
	fmt.Printf("✓ Upload speed: %d bytes/s\n", info.UpInfoSpeed)

	// Get categories
	categories, err := client.GetCategories()
	if err != nil {
		log.Fatalf("Error getting categories: %v", err)
	}
	fmt.Printf("✓ Found %d categories\n", len(categories))

	fmt.Println("\n✅ All operations completed successfully!")
	fmt.Println("   No debug logs were shown because Debug: false")
	fmt.Println("   Session management happened transparently in the background")
}

