package offlineparser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Config is the normalized per-tenant parser configuration. Boolean pointers
// in rawConfig preserve the difference between an omitted value and false.
type Config struct {
	PreserveFrontMatter bool
	HTMLTitleAsHeading  bool
	OutboundProbe       bool
}

type rawConfig struct {
	PreserveFrontMatter *bool `json:"preserve_front_matter"`
	HTMLTitleAsHeading  *bool `json:"html_title_as_heading"`
	OutboundProbe       *bool `json:"outbound_probe"`
}

func ParseConfig(data json.RawMessage) (Config, error) {
	config := Config{
		PreserveFrontMatter: true,
		HTMLTitleAsHeading:  true,
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return config, nil
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return Config{}, errors.New("parser config must be a JSON object")
	}

	var raw rawConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Config{}, fmt.Errorf("decode parser config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("parser config must contain one JSON object")
		}
		return Config{}, fmt.Errorf("decode trailing parser config: %w", err)
	}

	if raw.PreserveFrontMatter != nil {
		config.PreserveFrontMatter = *raw.PreserveFrontMatter
	}
	if raw.HTMLTitleAsHeading != nil {
		config.HTMLTitleAsHeading = *raw.HTMLTitleAsHeading
	}
	if raw.OutboundProbe != nil {
		config.OutboundProbe = *raw.OutboundProbe
	}
	return config, nil
}
