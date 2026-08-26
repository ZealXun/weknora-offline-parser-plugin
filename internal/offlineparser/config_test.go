package offlineparser

import (
	"encoding/json"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	config, err := ParseConfig(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	if !config.PreserveFrontMatter || !config.HTMLTitleAsHeading || config.OutboundProbe {
		t.Fatalf("unexpected defaults: %+v", config)
	}
}

func TestParseConfigAppliesExplicitValues(t *testing.T) {
	config, err := ParseConfig(json.RawMessage(`{
		"preserve_front_matter": false,
		"html_title_as_heading": false,
		"outbound_probe": true
	}`))
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}
	if config.PreserveFrontMatter || config.HTMLTitleAsHeading || !config.OutboundProbe {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestParseConfigRejectsInvalidObjects(t *testing.T) {
	for _, value := range []string{
		`null`,
		`{"unknown": true}`,
		`{} {}`,
	} {
		if _, err := ParseConfig(json.RawMessage(value)); err == nil {
			t.Fatalf("ParseConfig(%s) should fail", value)
		}
	}
}
