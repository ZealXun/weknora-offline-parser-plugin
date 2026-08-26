package main

import (
	"testing"

	plugin "github.com/Tencent/WeKnora/sdk/plugin/go"
)

func TestEmbeddedManifest(t *testing.T) {
	manifest, err := plugin.ParseManifest(manifestYAML)
	if err != nil {
		t.Fatalf("ParseManifest returned error: %v", err)
	}
	if manifest.Metadata.ID != "io.zealxun.parser.offline" {
		t.Fatalf("unexpected plugin id %q", manifest.Metadata.ID)
	}
	if len(manifest.NormalizedTypes()) != 1 || manifest.NormalizedTypes()[0] != "document_parser" {
		t.Fatalf("unexpected plugin types %v", manifest.NormalizedTypes())
	}
	if manifest.Spec.Permissions.Network {
		t.Fatal("offline parser must not request network permission")
	}
	if len(manifest.Spec.Permissions.AllowedHosts) != 0 {
		t.Fatalf("offline parser must not allow hosts: %v", manifest.Spec.Permissions.AllowedHosts)
	}
}
