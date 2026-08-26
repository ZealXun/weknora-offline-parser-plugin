package offlineparser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	pluginv1 "github.com/Tencent/WeKnora/api/proto/plugin/v1"
	plugin "github.com/Tencent/WeKnora/sdk/plugin/go"
)

const (
	maxInputBytes   = 100 * 1024 * 1024
	defaultProbeURL = "https://example.com/"
)

type httpDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Handler owns no tenant state. Every invocation receives its complete config
// and document stream, so one process can safely serve concurrent tenants.
type Handler struct {
	httpClient httpDoer
	probeURL   string
}

func NewHandler() *Handler {
	return &Handler{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		probeURL:   defaultProbeURL,
	}
}

func (h *Handler) ValidateConfig(_ context.Context, data json.RawMessage) []plugin.FieldViolation {
	if _, err := ParseConfig(data); err != nil {
		return []plugin.FieldViolation{{Field: "$", Description: err.Error()}}
	}
	return nil
}

func (h *Handler) HealthCheck(_ context.Context) plugin.Health {
	return plugin.Health{
		Status:  pluginv1.HealthCheckResponse_STATUS_SERVING,
		Message: "offline document parser is ready",
		Details: map[string]string{
			"formats":            "md,markdown,txt,html,htm",
			"network_permission": "disabled",
		},
	}
}

func (h *Handler) Parse(ctx context.Context, input plugin.ParseInput, stream plugin.ParseStream) error {
	config, err := ParseConfig(input.Config)
	if err != nil {
		return fmt.Errorf("invalid parser config: %w", err)
	}
	content, err := receiveDocument(input.ContentSize, stream)
	if err != nil {
		return err
	}
	document, err := Transform(input.FileName, input.ContentType, content, config)
	if err != nil {
		return err
	}
	if strings.TrimSpace(document.Markdown) == "" {
		return errEmptyDocument
	}

	metadata := plugin.ParsedMetadata{
		Title: document.Title,
		Values: map[string]string{
			"parser":      "io.zealxun.parser.offline",
			"format":      document.Format,
			"source_file": input.FileName,
		},
	}
	if config.OutboundProbe {
		metadata.Values["network_probe"] = h.runOutboundProbe(ctx)
	} else {
		metadata.Values["network_probe"] = "disabled"
	}
	if err := stream.EmitMetadata(metadata); err != nil {
		return fmt.Errorf("emit parsed metadata: %w", err)
	}
	if err := stream.EmitMarkdown(document.Markdown); err != nil {
		return fmt.Errorf("emit parsed markdown: %w", err)
	}
	return nil
}

func receiveDocument(expectedSize int64, stream plugin.ParseStream) ([]byte, error) {
	if expectedSize < 0 {
		return nil, errors.New("document size cannot be negative")
	}
	if expectedSize > maxInputBytes {
		return nil, fmt.Errorf("document exceeds %d bytes", maxInputBytes)
	}

	var content bytes.Buffer
	if expectedSize > 0 {
		content.Grow(int(expectedSize))
	}
	for {
		chunk, err := stream.RecvContent()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("receive document content: %w", err)
		}
		if int64(content.Len()+len(chunk)) > maxInputBytes {
			return nil, fmt.Errorf("document exceeds %d bytes", maxInputBytes)
		}
		_, _ = content.Write(chunk)
	}
	if int64(content.Len()) != expectedSize {
		return nil, fmt.Errorf("document size mismatch: received %d bytes, expected %d", content.Len(), expectedSize)
	}
	return content.Bytes(), nil
}

func (h *Handler) runOutboundProbe(ctx context.Context) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, h.probeURL, nil)
	if err != nil {
		return "request_error"
	}
	response, err := h.httpClient.Do(request)
	if err != nil {
		return "blocked:" + compactProbeError(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4*1024))
	if response.StatusCode == http.StatusForbidden {
		return "denied:http_403"
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return "unexpectedly_allowed:http_" + strconv.Itoa(response.StatusCode)
	}
	return "rejected:http_" + strconv.Itoa(response.StatusCode)
}

func compactProbeError(err error) string {
	value := strings.ToLower(err.Error())
	switch {
	case strings.Contains(value, "forbidden"), strings.Contains(value, "proxyconnect"):
		return "proxy_denied"
	case strings.Contains(value, "network is unreachable"), strings.Contains(value, "no route to host"):
		return "network_unreachable"
	case strings.Contains(value, "timeout"), strings.Contains(value, "deadline exceeded"):
		return "timeout"
	default:
		return "request_failed"
	}
}

var (
	_ plugin.LifecycleHooks        = (*Handler)(nil)
	_ plugin.DocumentParserHandler = (*Handler)(nil)
)
