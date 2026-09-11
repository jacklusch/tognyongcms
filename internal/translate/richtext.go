package translate

import (
	"context"
	"strings"

	"golang.org/x/net/html"
)

// TranslateRichText 按块翻译 HTML：提取 <p>/<li>/<h1-h6> 文本块逐个翻译（块内 strong/em/u/a/span 等内联元素
// 的文本也翻译，标签与属性保留），非文本标签（img/a/br）保留。
// 无法解析时退化为整体当纯文本翻译并包回 <p>。
func (s *Service) TranslateRichText(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error) {
	if strings.TrimSpace(htmlStr) == "" {
		return htmlStr, nil
	}
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return s.wrapParagraphFallback(ctx, htmlStr, sourceLang, targetLang)
	}
	var translateErr error
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode && n.Parent != nil {
			if nearestBlockAncestor(n) != "" {
				txt := strings.TrimSpace(n.Data)
				if txt != "" {
					tr, err := s.TranslateText(ctx, txt, sourceLang, targetLang)
					if err != nil {
						translateErr = err
						return
					}
					n.Data = tr
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if translateErr != nil {
		return s.wrapParagraphFallback(ctx, htmlStr, sourceLang, targetLang)
	}
	var b strings.Builder
	html.Render(&b, doc)
	return b.String(), nil
}

// nearestBlockAncestor 从文本节点向上找最近的块级容器（p/li/h1-h6），
// 块级容器之内的文本无论父标签（strong/em/u/a/span/b/i 等）都算该块内容；找不到返回空串。
func nearestBlockAncestor(n *html.Node) string {
	for p := n.Parent; p != nil; p = p.Parent {
		switch p.Data {
		case "p", "li", "h1", "h2", "h3", "h4", "h5", "h6":
			return p.Data
		case "body", "html":
			return ""
		}
	}
	return ""
}

// wrapParagraphFallback 降级：整体当纯文本翻译，包回 <p>。
func (s *Service) wrapParagraphFallback(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error) {
	text := stripTags(htmlStr)
	tr, err := s.TranslateText(ctx, text, sourceLang, targetLang)
	if err != nil {
		return htmlStr, err
	}
	return "<p>" + tr + "</p>", nil
}

func stripTags(s string) string {
	var b strings.Builder
	r := strings.NewReader(s)
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt == html.TextToken {
			b.WriteString(z.Token().Data)
		}
	}
	return b.String()
}
