package offlineparser

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

type ParsedDocument struct {
	Title    string
	Format   string
	Markdown string
}

func Transform(fileName, contentType string, content []byte, config Config) (ParsedDocument, error) {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	format, err := documentFormat(fileName, contentType)
	if err != nil {
		return ParsedDocument{}, err
	}

	switch format {
	case "markdown":
		markdown := normalizeText(string(content))
		if !config.PreserveFrontMatter {
			markdown = stripFrontMatter(markdown)
		}
		return ParsedDocument{
			Title:    firstMarkdownHeading(markdown, fallbackTitle(fileName)),
			Format:   format,
			Markdown: ensureTrailingNewline(markdown),
		}, nil
	case "text":
		markdown := normalizeText(string(content))
		return ParsedDocument{
			Title:    fallbackTitle(fileName),
			Format:   format,
			Markdown: ensureTrailingNewline(markdown),
		}, nil
	case "html":
		return transformHTML(fileName, content, config)
	default:
		return ParsedDocument{}, fmt.Errorf("unsupported document format %q", format)
	}
}

func documentFormat(fileName, contentType string) (string, error) {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), ".")) {
	case "md", "markdown":
		return "markdown", nil
	case "txt":
		return "text", nil
	case "html", "htm":
		return "html", nil
	}

	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch mediaType {
	case "text/markdown", "text/x-markdown":
		return "markdown", nil
	case "text/plain":
		return "text", nil
	case "text/html", "application/xhtml+xml":
		return "html", nil
	default:
		return "", fmt.Errorf("unsupported file %q with content type %q", fileName, contentType)
	}
}

func transformHTML(fileName string, content []byte, config Config) (ParsedDocument, error) {
	root, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return ParsedDocument{}, fmt.Errorf("parse HTML: %w", err)
	}
	title := strings.TrimSpace(findHTMLTitle(root))
	var output strings.Builder
	body := findElement(root, "body")
	if body == nil {
		body = root
	}
	for child := body.FirstChild; child != nil; child = child.NextSibling {
		renderHTMLNode(&output, child, 0)
	}
	markdown := normalizeRenderedMarkdown(output.String())
	if config.HTMLTitleAsHeading && title != "" && !strings.HasPrefix(strings.TrimSpace(markdown), "# ") {
		markdown = "# " + title + "\n\n" + markdown
	}
	if title == "" {
		title = firstMarkdownHeading(markdown, fallbackTitle(fileName))
	}
	return ParsedDocument{
		Title:    title,
		Format:   "html",
		Markdown: ensureTrailingNewline(markdown),
	}, nil
}

func renderHTMLNode(output *strings.Builder, node *html.Node, listDepth int) {
	if node.Type == html.TextNode {
		output.WriteString(collapseHTMLText(node.Data))
		return
	}
	if node.Type != html.ElementNode && node.Type != html.DocumentNode {
		return
	}

	tag := strings.ToLower(node.Data)
	switch tag {
	case "script", "style", "noscript", "template", "head":
		return
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level, _ := strconv.Atoi(strings.TrimPrefix(tag, "h"))
		writeBlockBoundary(output)
		output.WriteString(strings.Repeat("#", level))
		output.WriteByte(' ')
		renderHTMLChildren(output, node, listDepth)
		output.WriteString("\n\n")
		return
	case "p", "div", "section", "article", "header", "footer", "main", "aside":
		writeBlockBoundary(output)
		renderHTMLChildren(output, node, listDepth)
		output.WriteString("\n\n")
		return
	case "br":
		output.WriteString("  \n")
		return
	case "strong", "b":
		renderWrapped(output, node, "**", "**", listDepth)
		return
	case "em", "i":
		renderWrapped(output, node, "*", "*", listDepth)
		return
	case "del", "s", "strike":
		renderWrapped(output, node, "~~", "~~", listDepth)
		return
	case "code":
		if node.Parent != nil && strings.EqualFold(node.Parent.Data, "pre") {
			output.WriteString(textContent(node))
			return
		}
		code := strings.TrimSpace(textContent(node))
		if code != "" {
			output.WriteByte('`')
			output.WriteString(strings.ReplaceAll(code, "`", "\\`"))
			output.WriteByte('`')
		}
		return
	case "pre":
		writeBlockBoundary(output)
		output.WriteString("```\n")
		output.WriteString(strings.TrimRight(textContent(node), "\r\n"))
		output.WriteString("\n```\n\n")
		return
	case "a":
		label := strings.TrimSpace(renderHTMLFragment(node, listDepth))
		href := strings.TrimSpace(attribute(node, "href"))
		if label == "" {
			label = href
		}
		if href == "" || label == "" {
			output.WriteString(label)
			return
		}
		output.WriteString("[" + label + "](" + href + ")")
		return
	case "img":
		alt := strings.TrimSpace(attribute(node, "alt"))
		source := strings.TrimSpace(attribute(node, "src"))
		if source != "" {
			output.WriteString("![" + alt + "](" + source + ")")
		}
		return
	case "ul", "ol":
		writeBlockBoundary(output)
		ordered := tag == "ol"
		index := 1
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.ElementNode || !strings.EqualFold(child.Data, "li") {
				continue
			}
			output.WriteString(strings.Repeat("  ", listDepth))
			if ordered {
				output.WriteString(strconv.Itoa(index) + ". ")
			} else {
				output.WriteString("- ")
			}
			renderHTMLChildren(output, child, listDepth+1)
			output.WriteByte('\n')
			index++
		}
		output.WriteByte('\n')
		return
	case "blockquote":
		fragment := strings.TrimSpace(renderHTMLFragment(node, listDepth))
		if fragment != "" {
			writeBlockBoundary(output)
			for _, line := range strings.Split(fragment, "\n") {
				output.WriteString("> " + line + "\n")
			}
			output.WriteByte('\n')
		}
		return
	case "hr":
		writeBlockBoundary(output)
		output.WriteString("---\n\n")
		return
	}
	renderHTMLChildren(output, node, listDepth)
}

func renderHTMLChildren(output *strings.Builder, node *html.Node, listDepth int) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		renderHTMLNode(output, child, listDepth)
	}
}

func renderHTMLFragment(node *html.Node, listDepth int) string {
	var output strings.Builder
	renderHTMLChildren(&output, node, listDepth)
	return output.String()
}

func renderWrapped(output *strings.Builder, node *html.Node, prefix, suffix string, listDepth int) {
	fragment := strings.TrimSpace(renderHTMLFragment(node, listDepth))
	if fragment != "" {
		output.WriteString(prefix + fragment + suffix)
	}
}

func writeBlockBoundary(output *strings.Builder) {
	if output.Len() == 0 {
		return
	}
	value := output.String()
	if strings.HasSuffix(value, "\n\n") {
		return
	}
	if strings.HasSuffix(value, "\n") {
		output.WriteByte('\n')
		return
	}
	output.WriteString("\n\n")
}

func collapseHTMLText(value string) string {
	if value == "" {
		return ""
	}
	leading := unicode.IsSpace(rune(value[0]))
	trailing := unicode.IsSpace(rune(value[len(value)-1]))
	collapsed := strings.Join(strings.Fields(value), " ")
	if collapsed == "" {
		return " "
	}
	if leading {
		collapsed = " " + collapsed
	}
	if trailing {
		collapsed += " "
	}
	return collapsed
}

func normalizeRenderedMarkdown(value string) string {
	value = normalizeText(value)
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	empty := 0
	for _, line := range lines {
		line = strings.TrimRightFunc(line, unicode.IsSpace)
		if line == "" {
			empty++
			if empty > 1 {
				continue
			}
		} else {
			empty = 0
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func normalizeText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\r", "\n")
}

func stripFrontMatter(markdown string) string {
	lines := strings.Split(markdown, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return markdown
	}
	for index := 1; index < len(lines); index++ {
		marker := strings.TrimSpace(lines[index])
		if marker == "---" || marker == "..." {
			return strings.TrimLeft(strings.Join(lines[index+1:], "\n"), "\n")
		}
	}
	return markdown
}

func firstMarkdownHeading(markdown, fallback string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			if title := strings.TrimSpace(strings.TrimPrefix(line, "# ")); title != "" {
				return title
			}
		}
	}
	return fallback
}

func fallbackTitle(fileName string) string {
	base := filepath.Base(fileName)
	extension := filepath.Ext(base)
	title := strings.TrimSpace(strings.TrimSuffix(base, extension))
	if title == "" || title == "." {
		return "未命名文档"
	}
	return title
}

func ensureTrailingNewline(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return value + "\n"
}

func findHTMLTitle(node *html.Node) string {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "title") {
		return textContent(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if title := findHTMLTitle(child); title != "" {
			return title
		}
	}
	return ""
}

func findElement(node *html.Node, tag string) *html.Node {
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, tag) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}

func textContent(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	var output strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		output.WriteString(textContent(child))
	}
	return output.String()
}

func attribute(node *html.Node, key string) string {
	for _, item := range node.Attr {
		if strings.EqualFold(item.Key, key) {
			return item.Val
		}
	}
	return ""
}

var errEmptyDocument = errors.New("document contains no parseable text")
