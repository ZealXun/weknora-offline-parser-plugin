package offlineparser

import (
	"strings"
	"testing"
)

func TestTransformMarkdownRemovesFrontMatter(t *testing.T) {
	document, err := Transform("guide.md", "text/markdown", []byte("---\ntags: [demo]\n---\n# 使用指南\n\n正文"), Config{
		PreserveFrontMatter: false,
	})
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	if document.Title != "使用指南" {
		t.Fatalf("unexpected title %q", document.Title)
	}
	if strings.Contains(document.Markdown, "tags:") {
		t.Fatalf("front matter was not removed: %q", document.Markdown)
	}
}

func TestTransformTextNormalizesLineEndings(t *testing.T) {
	document, err := Transform("notes.txt", "text/plain", []byte("第一行\r\n第二行\r"), Config{})
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	if document.Title != "notes" || document.Markdown != "第一行\n第二行\n" {
		t.Fatalf("unexpected document: %+v", document)
	}
}

func TestTransformHTMLPreservesCommonSemantics(t *testing.T) {
	content := `<!doctype html><html><head><title>插件指南</title></head><body>
		<p>这是 <strong>离线</strong> 解析器。</p>
		<ul><li>Markdown</li><li><a href="https://example.com/docs">HTML</a></li></ul>
		<pre>go test ./...</pre><blockquote>网络默认关闭</blockquote>
	</body></html>`
	document, err := Transform("guide.html", "text/html", []byte(content), Config{HTMLTitleAsHeading: true})
	if err != nil {
		t.Fatalf("Transform returned error: %v", err)
	}
	for _, expected := range []string{
		"# 插件指南",
		"这是 **离线** 解析器。",
		"- Markdown",
		"[HTML](https://example.com/docs)",
		"```\ngo test ./...\n```",
		"> 网络默认关闭",
	} {
		if !strings.Contains(document.Markdown, expected) {
			t.Fatalf("markdown does not contain %q:\n%s", expected, document.Markdown)
		}
	}
}

func TestTransformRejectsUnsupportedFormat(t *testing.T) {
	if _, err := Transform("report.pdf", "application/pdf", []byte("pdf"), Config{}); err == nil {
		t.Fatal("Transform should reject unsupported files")
	}
}
