package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleYAML = `
server:
  addr: ":8080"
  data_dir: "./data"
database:
  driver: "sqlite"
  dsn: "dulizhan.db"
site:
  name: "测试站点"
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh", "en"]
  theme: "default"
  themes_dir: "./themes"
media:
  driver: "local"
`

func TestLoadFromYAML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(sampleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Site.Name != "测试站点" {
		t.Errorf("Name = %q, want 测试站点", cfg.Site.Name)
	}
	if cfg.Site.DefaultLang != "zh" || len(cfg.Site.Languages) != 2 {
		t.Errorf("languages wrong: %+v", cfg.Site.Languages)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("driver = %q", cfg.Database.Driver)
	}
}

func TestEnvOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(sampleYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DULIZHAN_SERVER_ADDR", ":9999")
	t.Setenv("DULIZHAN_SITE_URL", "https://override.example.com")
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Server.Addr != ":9999" {
		t.Errorf("addr = %q, want :9999", cfg.Server.Addr)
	}
	if cfg.Site.URL != "https://override.example.com" {
		t.Errorf("url = %q", cfg.Site.URL)
	}
}

func TestValidate(t *testing.T) {
	cfg := &Config{}
	cfg.Site.URL = ""
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh"}
	cfg.Site.Theme = "default"
	if err := cfg.Validate(); err == nil {
		t.Error("want error for empty URL")
	}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "fr"
	if err := cfg.Validate(); err == nil {
		t.Error("want error when default lang not in languages")
	}
	cfg.Site.DefaultLang = "zh"
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateS3Config(t *testing.T) {
	cfg := &Config{}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh"}
	cfg.Media.Driver = "s3"
	cfg.Media.S3 = &S3Config{Region: "cn-hangzhou"} // 缺 bucket
	if err := cfg.Validate(); err == nil {
		t.Error("s3 driver 缺 bucket 应报错")
	}
	cfg.Media.S3 = &S3Config{Region: "cn-hangzhou", Bucket: "b"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("s3 driver 配 bucket 应通过: %v", err)
	}
}

func TestServerDebugDefault(t *testing.T) {
	cfg := &Config{}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Debug {
		t.Error("Debug 默认应为 false")
	}
}

func TestHomeCategoryConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
site:
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh"]
  home_products_category: "products"
  home_news_category: "news"
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Site.HomeProductsCategory != "products" {
		t.Errorf("HomeProductsCategory = %q, want products", cfg.Site.HomeProductsCategory)
	}
	if cfg.Site.HomeNewsCategory != "news" {
		t.Errorf("HomeNewsCategory = %q, want news", cfg.Site.HomeNewsCategory)
	}
}

func TestLoadS3Config(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
site:
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh"]
media:
  driver: "s3"
  s3:
    region: "cn-hangzhou"
    bucket: "my-bucket"
    access_key: "ak"
    secret_key: "sk"
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Media.Driver != "s3" || cfg.Media.S3 == nil || cfg.Media.S3.Bucket != "my-bucket" {
		t.Errorf("S3 配置加载错误: %+v", cfg.Media)
	}
}

func TestTranslateConfigLoad(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
site:
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh", "en"]
translate:
  enabled: true
  api_key: "sk-test"
  base_url: "https://api.openai.com/v1"
  model: "gpt-4o-mini"
  source_lang: "zh"
  target_lang: "en"
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Translate.Enabled {
		t.Error("Translate.Enabled = false, want true")
	}
	if cfg.Translate.APIKey != "sk-test" || cfg.Translate.BaseURL != "https://api.openai.com/v1" || cfg.Translate.Model != "gpt-4o-mini" {
		t.Errorf("translate 配置错误: %+v", cfg.Translate)
	}
	if cfg.Translate.SourceLang != "zh" || cfg.Translate.TargetLang != "en" {
		t.Errorf("源/目标语言错误: %+v", cfg.Translate)
	}
}

func TestTranslateValidate(t *testing.T) {
	// enabled 但缺 api_key → 报错
	cfg := &Config{}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh", "en"}
	cfg.Site.Theme = "default"
	cfg.Translate.Enabled = true
	if err := cfg.Validate(); err == nil {
		t.Error("enabled 缺 api_key 应报错")
	}
	cfg.Translate.APIKey = "sk"
	cfg.Translate.BaseURL = "https://api.openai.com/v1"
	cfg.Translate.Model = "gpt-4o-mini"
	cfg.Translate.TargetLang = "en"
	if err := cfg.Validate(); err != nil {
		t.Errorf("补齐后应通过: %v", err)
	}
}

func TestTranslateValidateLangDomain(t *testing.T) {
	base := func() *Config {
		cfg := &Config{}
		cfg.Site.URL = "https://example.com"
		cfg.Site.DefaultLang = "zh"
		cfg.Site.Languages = []string{"zh", "en"}
		cfg.Site.Theme = "default"
		cfg.Translate.Enabled = true
		cfg.Translate.APIKey = "sk"
		cfg.Translate.BaseURL = "https://api.openai.com/v1"
		cfg.Translate.Model = "gpt-4o-mini"
		cfg.Translate.SourceLang = "zh"
		cfg.Translate.TargetLang = "en"
		return cfg
	}
	if err := base().Validate(); err != nil {
		t.Errorf("合法配置应通过: %v", err)
	}
	// source_lang 不在 site.languages → 报错
	cfg := base()
	cfg.Translate.SourceLang = "fr"
	if err := cfg.Validate(); err == nil {
		t.Error("source_lang 不在 site.languages 应报错")
	}
	// target_lang 不在 site.languages → 报错
	cfg = base()
	cfg.Translate.TargetLang = "de"
	if err := cfg.Validate(); err == nil {
		t.Error("target_lang 不在 site.languages 应报错")
	}
}
