package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Site      SiteConfig      `yaml:"site"`
	Media     MediaConfig     `yaml:"media"`
	Translate TranslateConfig `yaml:"translate"`
}

type TranslateConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Provider   string `yaml:"provider"`
	APIKey     string `yaml:"api_key"`
	BaseURL    string `yaml:"base_url"`
	Model      string `yaml:"model"`
	SourceLang string `yaml:"source_lang"`
	TargetLang string `yaml:"target_lang"`
}

type ServerConfig struct {
	Addr    string `yaml:"addr"`
	DataDir string `yaml:"data_dir"`
	Debug   bool   `yaml:"debug"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type SiteConfig struct {
	Name          string            `yaml:"name"`
	URL           string            `yaml:"url"`
	DefaultLang   string            `yaml:"default_lang"`
	Languages     []string          `yaml:"languages"`
	Theme         string            `yaml:"theme"`
	ThemesDir     string            `yaml:"themes_dir"`
	PrefixDefault bool              `yaml:"prefix_default_lang"`
	Description   string            `yaml:"description"`
	OGImage       string            `yaml:"og_image"`
	Names         map[string]string `yaml:"names"`
	Descriptions  map[string]string `yaml:"descriptions"`
	HomeTitles    map[string]string `yaml:"home_titles"`

	HomeProductsCategory string `yaml:"home_products_category"`
	HomeNewsCategory     string `yaml:"home_news_category"`
}

// SiteName 按语言取品牌名，缺失回退 Name。
func (s *SiteConfig) SiteName(lang string) string {
	if s.Names != nil {
		if v, ok := s.Names[lang]; ok && v != "" {
			return v
		}
	}
	return s.Name
}

// SiteDescription 按语言取 meta description，缺失回退 Description。
func (s *SiteConfig) SiteDescription(lang string) string {
	if s.Descriptions != nil {
		if v, ok := s.Descriptions[lang]; ok && v != "" {
			return v
		}
	}
	return s.Description
}

// HomeTitle 按语言取首页 title（含关键词），缺失回退 SiteName。
func (s *SiteConfig) HomeTitle(lang string) string {
	if s.HomeTitles != nil {
		if v, ok := s.HomeTitles[lang]; ok && v != "" {
			return v
		}
	}
	return s.SiteName(lang)
}

type MediaConfig struct {
	Driver string    `yaml:"driver"`
	S3     *S3Config `yaml:"s3"`
}

type S3Config struct {
	Endpoint      string `yaml:"endpoint"`
	Region        string `yaml:"region"`
	Bucket        string `yaml:"bucket"`
	AccessKey     string `yaml:"access_key"`
	SecretKey     string `yaml:"secret_key"`
	PublicBaseURL string `yaml:"public_base_url"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyEnv(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	set := func(key string, dst *string) {
		if v, ok := os.LookupEnv("DULIZHAN_" + key); ok && v != "" {
			*dst = v
		}
	}
	set("SERVER_ADDR", &cfg.Server.Addr)
	set("SERVER_DATA_DIR", &cfg.Server.DataDir)
	if v, ok := os.LookupEnv("DULIZHAN_SERVER_DEBUG"); ok && v == "true" {
		cfg.Server.Debug = true
	}
	set("DATABASE_DRIVER", &cfg.Database.Driver)
	set("DATABASE_DSN", &cfg.Database.DSN)
	set("SITE_URL", &cfg.Site.URL)
	set("SITE_NAME", &cfg.Site.Name)
	set("SITE_THEME", &cfg.Site.Theme)
	set("SITE_DEFAULT_LANG", &cfg.Site.DefaultLang)
	set("SITE_HOME_PRODUCTS_CATEGORY", &cfg.Site.HomeProductsCategory)
	set("SITE_HOME_NEWS_CATEGORY", &cfg.Site.HomeNewsCategory)
	set("MEDIA_DRIVER", &cfg.Media.Driver)
	if v, ok := os.LookupEnv("DULIZHAN_SITE_LANGUAGES"); ok && v != "" {
		cfg.Site.Languages = strings.Split(v, ",")
	}
	applyS3Env(cfg)
	if v, ok := os.LookupEnv("DULIZHAN_TRANSLATE_ENABLED"); ok && v == "true" {
		cfg.Translate.Enabled = true
	}
	set("TRANSLATE_API_KEY", &cfg.Translate.APIKey)
	set("TRANSLATE_BASE_URL", &cfg.Translate.BaseURL)
	set("TRANSLATE_MODEL", &cfg.Translate.Model)
	set("TRANSLATE_SOURCE_LANG", &cfg.Translate.SourceLang)
	set("TRANSLATE_TARGET_LANG", &cfg.Translate.TargetLang)
}

func applyS3Env(cfg *Config) {
	set := func(key string, dst *string) {
		if v, ok := os.LookupEnv("DULIZHAN_" + key); ok && v != "" {
			*dst = v
		}
	}
	_, hasEndpoint := os.LookupEnv("DULIZHAN_MEDIA_S3_ENDPOINT")
	_, hasRegion := os.LookupEnv("DULIZHAN_MEDIA_S3_REGION")
	_, hasBucket := os.LookupEnv("DULIZHAN_MEDIA_S3_BUCKET")
	_, hasAccessKey := os.LookupEnv("DULIZHAN_MEDIA_S3_ACCESS_KEY")
	_, hasSecretKey := os.LookupEnv("DULIZHAN_MEDIA_S3_SECRET_KEY")
	_, hasPublicBaseURL := os.LookupEnv("DULIZHAN_MEDIA_S3_PUBLIC_BASE_URL")
	if !(hasEndpoint || hasRegion || hasBucket || hasAccessKey || hasSecretKey || hasPublicBaseURL) {
		return
	}
	if cfg.Media.S3 == nil {
		cfg.Media.S3 = &S3Config{}
	}
	set("MEDIA_S3_ENDPOINT", &cfg.Media.S3.Endpoint)
	set("MEDIA_S3_REGION", &cfg.Media.S3.Region)
	set("MEDIA_S3_BUCKET", &cfg.Media.S3.Bucket)
	set("MEDIA_S3_ACCESS_KEY", &cfg.Media.S3.AccessKey)
	set("MEDIA_S3_SECRET_KEY", &cfg.Media.S3.SecretKey)
	set("MEDIA_S3_PUBLIC_BASE_URL", &cfg.Media.S3.PublicBaseURL)
}

func (c *Config) Validate() error {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Server.DataDir == "" {
		c.Server.DataDir = "./data"
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "sqlite"
	}
	if c.Site.URL == "" {
		return fmt.Errorf("site.url 不能为空")
	}
	if len(c.Site.Languages) == 0 {
		return fmt.Errorf("site.languages 至少配置一种语言")
	}
	found := false
	for _, l := range c.Site.Languages {
		if l == c.Site.DefaultLang {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("site.default_lang %q 不在 site.languages 中", c.Site.DefaultLang)
	}
	if c.Site.Theme == "" {
		c.Site.Theme = "default"
	}
	if c.Site.ThemesDir == "" {
		c.Site.ThemesDir = "./themes"
	}
	if c.Media.Driver == "s3" {
		if c.Media.S3 == nil || c.Media.S3.Bucket == "" {
			return fmt.Errorf("media.s3.bucket 必填（driver=s3 时）")
		}
	}
	if c.Translate.Enabled {
		if c.Translate.APIKey == "" {
			return fmt.Errorf("translate.api_key 必填（enabled=true 时）")
		}
		if c.Translate.BaseURL == "" {
			c.Translate.BaseURL = "https://api.openai.com/v1"
		}
		if c.Translate.Model == "" {
			return fmt.Errorf("translate.model 必填（enabled=true 时）")
		}
		if c.Translate.SourceLang == "" {
			c.Translate.SourceLang = c.Site.DefaultLang
		}
		if c.Translate.TargetLang == "" {
			return fmt.Errorf("translate.target_lang 必填（enabled=true 时）")
		}
		if c.Translate.SourceLang == c.Translate.TargetLang {
			return fmt.Errorf("translate.source_lang 与 target_lang 不能相同")
		}
		if !containsString(c.Site.Languages, c.Translate.SourceLang) {
			return fmt.Errorf("translate.source_lang %q 不在 site.languages 中", c.Translate.SourceLang)
		}
		if !containsString(c.Site.Languages, c.Translate.TargetLang) {
			return fmt.Errorf("translate.target_lang %q 不在 site.languages 中", c.Translate.TargetLang)
		}
	}
	return nil
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
