package qbt_test

import (
	"context"
	"fmt"
	"os"

	qbt "github.com/jfxdev/go-qbt"
)

func ExampleClient_ListTorrents() {
	if os.Getenv("QBT_EXAMPLE_LIVE") == "" {
		fmt.Println("skipped")
		// Output: skipped
		return
	}

	client, _ := qbt.New(qbt.Config{BaseURL: "http://localhost:8080"})
	defer client.Close()

	list, _ := client.ListTorrents(qbt.ListOptions{})
	fmt.Printf("torrents: %d\n", len(list))
}

func ExampleClient_AddTorrentLinkWithContext() {
	if os.Getenv("QBT_EXAMPLE_LIVE") == "" {
		fmt.Println("skipped")
		// Output: skipped
		return
	}

	client, _ := qbt.New(qbt.Config{BaseURL: "http://localhost:8080"})
	defer client.Close()

	ctx := context.Background()
	_ = client.AddTorrentLinkWithContext(ctx, qbt.TorrentConfig{
		MagnetURI:    "magnet:?xt=urn:btih:example",
		Directory:    "/downloads",
		Category:     "movies",
		Paused:       true,
		SkipChecking: true,
	})
}
