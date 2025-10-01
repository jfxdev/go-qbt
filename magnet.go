package qbt

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

// ParseMagnetLink extracts information from a magnet link
func ParseMagnetLink(magnetURI string) (*MagnetLink, error) {
	if !strings.HasPrefix(magnetURI, "magnet:?") {
		return nil, errors.New("invalid magnet link format")
	}

	// Remove the "magnet:?" prefix and parse the URL
	queryString := strings.TrimPrefix(magnetURI, "magnet:?")
	values, err := url.ParseQuery(queryString)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse magnet link query")
	}

	magnet := &MagnetLink{}

	// Extract the hash (btih)
	if hash := values.Get("xt"); hash != "" {
		// Remove the "urn:btih:" prefix if present
		if strings.HasPrefix(hash, "urn:btih:") {
			magnet.Hash = strings.TrimPrefix(hash, "urn:btih:")
		} else {
			magnet.Hash = hash
		}
	}

	// Extract the display name (dn)
	magnet.DisplayName = values.Get("dn")

	// Extract the trackers (tr)
	magnet.Trackers = values["tr"]

	// Extract other optional fields
	magnet.ExactLength = values.Get("xl")
	magnet.ExactSource = values.Get("xs")
	magnet.Keywords = values.Get("kt")
	magnet.AcceptableSource = values.Get("as")

	return magnet, nil
}
