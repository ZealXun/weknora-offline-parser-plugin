package main

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	pluginv1 "github.com/Tencent/WeKnora/api/proto/plugin/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const integrationPluginID = "io.zealxun.parser.offline"

type integrationBearerToken string

func (token integrationBearerToken) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(token)}, nil
}

func (integrationBearerToken) RequireTransportSecurity() bool { return false }

func TestPluginRuntimeIntegration(t *testing.T) {
	if os.Getenv("WEKNORA_PLUGIN_INTEGRATION") != "1" {
		t.Skip("set WEKNORA_PLUGIN_INTEGRATION=1 to run against plugin-runtime")
	}
	token := strings.TrimSpace(os.Getenv("WEKNORA_PLUGIN_RUNTIME_AUTH_TOKEN"))
	if token == "" {
		t.Fatal("WEKNORA_PLUGIN_RUNTIME_AUTH_TOKEN is required")
	}
	address := strings.TrimSpace(os.Getenv("WEKNORA_PLUGIN_RUNTIME_ADDR"))
	if address == "" {
		address = "127.0.0.1:9092"
	}

	connection, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(integrationBearerToken(token)),
	)
	if err != nil {
		t.Fatalf("create runtime client: %v", err)
	}
	defer connection.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	content := []byte("# 隔离验证\n\n文档解析不应被联网探针失败影响。")
	stream, err := pluginv1.NewDocumentParserPluginClient(connection).Parse(ctx)
	if err != nil {
		t.Fatalf("open parser stream: %v", err)
	}
	invocation := &pluginv1.InvocationContext{
		PluginId:  integrationPluginID,
		TenantId:  10001,
		RequestId: "offline-parser-integration",
	}
	if err := stream.Send(&pluginv1.ParseRequest{
		Context: invocation,
		Frame: &pluginv1.ParseRequest_Begin{Begin: &pluginv1.ParseBegin{
			ConfigJson:  []byte(`{"outbound_probe":true}`),
			FileName:    "network-isolation.md",
			ContentType: "text/markdown",
			ContentSize: int64(len(content)),
		}},
	}); err != nil {
		t.Fatalf("send parser begin: %v", err)
	}
	if err := stream.Send(&pluginv1.ParseRequest{
		Frame: &pluginv1.ParseRequest_ContentChunk{ContentChunk: content},
	}); err != nil {
		t.Fatalf("send parser content: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("close parser input: %v", err)
	}

	var markdown, probeResult string
	for {
		event, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			t.Fatalf("receive parser event: %v", recvErr)
		}
		switch value := event.GetEvent().(type) {
		case *pluginv1.ParseEvent_Metadata:
			probeResult = value.Metadata.GetValues()["network_probe"]
		case *pluginv1.ParseEvent_MarkdownChunk:
			markdown += value.MarkdownChunk
		}
	}
	if !strings.Contains(markdown, "# 隔离验证") {
		t.Fatalf("unexpected parsed markdown %q", markdown)
	}
	if !strings.HasPrefix(probeResult, "denied:") && !strings.HasPrefix(probeResult, "blocked:") {
		t.Fatalf("outbound probe was not denied: %q", probeResult)
	}

	runtimeClient := pluginv1.NewPluginRuntimeClient(connection)
	deadline := time.Now().Add(3 * time.Second)
	for {
		response, listErr := runtimeClient.ListEvents(ctx, &pluginv1.ListRuntimeEventsRequest{
			PluginId: integrationPluginID,
			Limit:    100,
		})
		if listErr != nil {
			t.Fatalf("list runtime events: %v", listErr)
		}
		for _, event := range response.GetEvents() {
			if event.GetKind() == "network_denied" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("network_denied runtime event was not recorded")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
