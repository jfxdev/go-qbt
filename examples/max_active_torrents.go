package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jfxdev/go-qbt"
)

// This example demonstrates how to manage maximum active torrent limits
// including downloads, uploads, torrents, and checking torrents.

func main() {
	config := qbt.Config{
		BaseURL:        "http://localhost:8080",
		Username:       "admin",
		Password:       "adminpass",
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   2 * time.Second,
		Debug:          false,
	}

	client, err := qbt.New(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("✓ Client initialized for maximum active torrent management")

	// Get current maximum active torrent limits
	fmt.Println("\n📊 Current Maximum Active Torrent Limits:")

	maxDownloads, err := client.GetMaxActiveDownloads()
	if err != nil {
		log.Printf("Error getting max active downloads: %v", err)
	} else {
		fmt.Printf("  • Max Active Downloads: %d\n", maxDownloads)
	}

	maxUploads, err := client.GetMaxActiveUploads()
	if err != nil {
		log.Printf("Error getting max active uploads: %v", err)
	} else {
		fmt.Printf("  • Max Active Uploads: %d\n", maxUploads)
	}

	maxTorrents, err := client.GetMaxActiveTorrents()
	if err != nil {
		log.Printf("Error getting max active torrents: %v", err)
	} else {
		fmt.Printf("  • Max Active Torrents: %d\n", maxTorrents)
	}

	maxChecking, err := client.GetMaxActiveCheckingTorrents()
	if err != nil {
		log.Printf("Error getting max active checking torrents: %v", err)
	} else {
		fmt.Printf("  • Max Active Checking Torrents: %d\n", maxChecking)
	}

	// Set individual maximum active torrent limits
	fmt.Println("\n🔧 Setting Individual Maximum Active Torrent Limits:")

	err = client.SetMaxActiveDownloads(5)
	if err != nil {
		log.Printf("Error setting max active downloads: %v", err)
	} else {
		fmt.Println("  ✓ Set Max Active Downloads to 5")
	}

	err = client.SetMaxActiveUploads(3)
	if err != nil {
		log.Printf("Error setting max active uploads: %v", err)
	} else {
		fmt.Println("  ✓ Set Max Active Uploads to 3")
	}

	err = client.SetMaxActiveTorrents(10)
	if err != nil {
		log.Printf("Error setting max active torrents: %v", err)
	} else {
		fmt.Println("  ✓ Set Max Active Torrents to 10")
	}

	err = client.SetMaxActiveCheckingTorrents(2)
	if err != nil {
		log.Printf("Error setting max active checking torrents: %v", err)
	} else {
		fmt.Println("  ✓ Set Max Active Checking Torrents to 2")
	}

	// Set all maximum active torrent limits at once
	fmt.Println("\n🎯 Setting All Maximum Active Torrent Limits at Once:")

	err = client.SetMaxActiveTorrentLimits(8, 4, 15, 3)
	if err != nil {
		log.Printf("Error setting all max active torrent limits: %v", err)
	} else {
		fmt.Println("  ✓ Set all limits: Downloads=8, Uploads=4, Torrents=15, Checking=3")
	}

	// Verify the changes
	fmt.Println("\n✅ Verifying Updated Maximum Active Torrent Limits:")

	maxDownloads, err = client.GetMaxActiveDownloads()
	if err != nil {
		log.Printf("Error getting max active downloads: %v", err)
	} else {
		fmt.Printf("  • Max Active Downloads: %d\n", maxDownloads)
	}

	maxUploads, err = client.GetMaxActiveUploads()
	if err != nil {
		log.Printf("Error getting max active uploads: %v", err)
	} else {
		fmt.Printf("  • Max Active Uploads: %d\n", maxUploads)
	}

	maxTorrents, err = client.GetMaxActiveTorrents()
	if err != nil {
		log.Printf("Error getting max active torrents: %v", err)
	} else {
		fmt.Printf("  • Max Active Torrents: %d\n", maxTorrents)
	}

	maxChecking, err = client.GetMaxActiveCheckingTorrents()
	if err != nil {
		log.Printf("Error getting max active checking torrents: %v", err)
	} else {
		fmt.Printf("  • Max Active Checking Torrents: %d\n", maxChecking)
	}

	fmt.Println("\n🎉 Maximum Active Torrent Management Example Completed!")
	fmt.Println("   All maximum active torrent limits have been configured successfully.")
}
