package offlineparser

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	plugin "github.com/Tencent/WeKnora/sdk/plugin/go"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (function doerFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

type recordingParseStream struct {
	chunks   [][]byte
	position int
	metadata plugin.ParsedMetadata
	markdown strings.Builder
}

func (s *recordingParseStream) RecvContent() ([]byte, error) {
	if s.position >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.position]
	s.position++
	return chunk, nil
}

func (s *recordingParseStream) EmitMetadata(metadata plugin.ParsedMetadata) error {
	s.metadata = metadata
	return nil
}

func (s *recordingParseStream) EmitMarkdown(markdown string) error {
	s.markdown.WriteString(markdown)
	return nil
}

func (s *recordingParseStream) EmitAttachment(plugin.ParsedAttachment) error {
	return nil
}

func TestHandlerParsesDocumentWhenProbeIsDenied(t *testing.T) {
	handler := &Handler{
		probeURL: "https://example.com/",
		httpClient: doerFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "https://example.com/" {
				t.Fatalf("unexpected probe URL %q", request.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Body:       io.NopCloser(strings.NewReader("plugin network access denied")),
			}, nil
		}),
	}
	stream := &recordingParseStream{chunks: [][]byte{[]byte("# 标题\n"), []byte("正文")}}
	input := plugin.ParseInput{
		Config:      json.RawMessage(`{"outbound_probe":true}`),
		FileName:    "demo.md",
		ContentType: "text/markdown",
		ContentSize: int64(len("# 标题\n正文")),
	}
	if err := handler.Parse(context.Background(), input, stream); err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if stream.metadata.Title != "标题" {
		t.Fatalf("unexpected title %q", stream.metadata.Title)
	}
	if stream.metadata.Values["network_probe"] != "denied:http_403" {
		t.Fatalf("unexpected probe result %q", stream.metadata.Values["network_probe"])
	}
	if stream.markdown.String() != "# 标题\n正文\n" {
		t.Fatalf("unexpected markdown %q", stream.markdown.String())
	}
}

func TestHandlerRejectsContentSizeMismatch(t *testing.T) {
	handler := NewHandler()
	stream := &recordingParseStream{chunks: [][]byte{[]byte("short")}}
	err := handler.Parse(context.Background(), plugin.ParseInput{
		Config:      json.RawMessage(`{}`),
		FileName:    "demo.txt",
		ContentType: "text/plain",
		ContentSize: 10,
	}, stream)
	if err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("expected size mismatch, got %v", err)
	}
}
