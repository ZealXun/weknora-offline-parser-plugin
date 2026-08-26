package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"os/signal"
	"syscall"

	plugin "github.com/Tencent/WeKnora/sdk/plugin/go"
	"github.com/ZealXun/weknora-offline-parser-plugin/internal/offlineparser"
)

//go:embed plugin.yaml
var manifestYAML []byte

func main() {
	manifest, err := plugin.ParseManifest(manifestYAML)
	if err != nil {
		log.Fatalf("invalid plugin manifest: %v", err)
	}

	handler := offlineparser.NewHandler()
	server, err := plugin.NewServer(manifest, plugin.ServerOptions{
		Lifecycle:      handler,
		DocumentParser: handler,
	})
	if err != nil {
		log.Fatalf("create plugin server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	address := os.Getenv("WEKNORA_PLUGIN_LISTEN")
	if address == "" {
		address = ":9000"
	}
	if err := server.Serve(ctx, address); err != nil {
		log.Fatalf("serve plugin: %v", err)
	}
}
