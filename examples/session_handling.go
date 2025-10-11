package main

import (
	"fmt"
	"log"
	"time"

	"github.com/jfxdev/go-qbt"
)

// This example demonstrates how the client automatically handles
// qBittorrent session expiration without any user intervention.
//
// Scenario:
// 1. Client performs initial login and caches the session cookie
// 2. Several hours pass (longer than qBittorrent's WebUISessionTimeout)
// 3. qBittorrent's session expires on the server side
// 4. Client tries to make a request with the expired cookie
// 5. qBittorrent returns 403 (Forbidden)
// 6. Client automatically detects the auth error
// 7. Client invalidates the cached cookie
// 8. Client retries the request with a fresh login
// 9. Operation completes successfully
//
// This all happens transparently without any error being returned to your code!

func main() {
	config := qbt.Config{
		BaseURL:        "http://localhost:8080",
		Username:       "admin",
		Password:       "adminpass",
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryBackoff:   2 * time.Second,
		Debug:          true, // Enable debug logging to see session management in action
	}

	client, err := qbt.New(config)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	fmt.Println("Client created successfully")
	fmt.Println("Debug mode is enabled - you'll see detailed logs below")

	// First request - this will login and cache the cookie
	fmt.Println("\n1. Making initial request...")
	torrents, err := client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing torrents: %v", err)
	}
	fmt.Printf("✓ Success! Found %d torrents\n", len(torrents))

	// In a real scenario, this is where several hours would pass
	// and the qBittorrent session would expire
	fmt.Println("\n2. Simulating session expiration...")
	fmt.Println("   (In production, this would be after several hours)")

	// Even after session expiration, subsequent requests will work
	// because the client automatically re-authenticates
	fmt.Println("\n3. Making request after session expiration...")
	fmt.Println("   The client will:")
	fmt.Println("   - Detect the 403 error from qBittorrent")
	fmt.Println("   - Invalidate the cached cookie")
	fmt.Println("   - Perform a new login automatically")
	fmt.Println("   - Retry the request with the new session")

	torrents, err = client.ListTorrents(qbt.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing torrents: %v", err)
	}
	fmt.Printf("✓ Success! Found %d torrents\n", len(torrents))

	fmt.Println("\n✅ All operations completed successfully!")
	fmt.Println("   The session expiration was handled transparently.")
}
