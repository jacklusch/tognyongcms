package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Translator 可插拔翻译器（便于测试 mock）。
type Translator interface {
	TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error)
	// TranslateRichText 按块翻译 HTML，非文本标签（img/a/br）保留。
	TranslateRichText(ctx context.Context, htmlStr, sourceLang, targetLang string) (string, error)
}

// Service OpenAI 兼容接口翻译实现。
type Service struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
}

func New(apiKey, baseURL, model string) *Service {
	return &Service{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// TranslateText 调用 chat/completions 翻译单段文本。
func (s *Service) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	if text == "" {
		return "", nil
	}
	reqBody := map[string]any{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a professional translator specializing in " + sourceLang + "-" + targetLang + " translation. Translate the user's text from " + sourceLang + " to " + targetLang + " accurately and naturally. Preserve proper nouns, brand names, numbers, and formatting. Output ONLY the translated text with no explanations, no preamble, no quotation marks."},
			{"role": "user", "content": text},
		},
		"temperature": 0.3,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("翻译接口返回 %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("翻译接口无结果")
	}
	return parsed.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
