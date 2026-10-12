package qbt

import (
	"encoding/json"
	"testing"
)

func TestTorrentCompletedBytes(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		present    bool
		want       int64
	}{
		{"missing", `{}`, false, 0},
		{"null", `{"completed":null}`, false, 0},
		{"empty", `{"completed":0}`, true, 0},
		{"partial", `{"completed":1234,"size":9999,"downloaded":5000}`, true, 1234},
		{"large", `{"completed":5497558138880}`, true, 5497558138880},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var torrent TorrentResponse
			if err := json.Unmarshal([]byte(tc.body), &torrent); err != nil {
				t.Fatal(err)
			}
			if (torrent.Completed != nil) != tc.present {
				t.Fatalf("unexpected presence: %v", torrent.Completed)
			}
			if tc.present && *torrent.Completed != tc.want {
				t.Fatalf("got %d want %d", *torrent.Completed, tc.want)
			}
		})
	}
}
