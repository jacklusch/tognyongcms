package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"dulizhan/internal/auth"
	"dulizhan/internal/config"
	"dulizhan/internal/content"
	"dulizhan/internal/i18n"
	"dulizhan/internal/media"
	"dulizhan/internal/schema"
	"dulizhan/internal/seed"
	"dulizhan/internal/server"
	"dulizhan/internal/store/sqlite"
	"dulizhan/internal/theme"
	"dulizhan/internal/translate"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	args := flag.Args()
	if len(args) > 0 && args[0] == "seed" {
		// seed 模式：config.yaml 可选，缺失时用默认值
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			cfg = &config.Config{}
			cfg.Database.Driver = "sqlite"
			cfg.Database.DSN = "dulizhan.db"
			cfg.Site.Languages = []string{"zh", "en"}
			cfg.Site.DefaultLang = "zh"
		}
		st, err := sqlite.Open(cfg.Database.DSN)
		if err != nil {
			log.Fatalf("打开数据库失败: %v", err)
		}
		authSvc := auth.New(st, 7*24*time.Hour)
		svc := content.New(st, schema.NewRegistry(), cfg.Site.Languages)
		// seed 分支：构造本地媒体（演示图片下载用），driver 无关
		seedMed := media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
		ctx := context.Background()
		if err := seed.EnsureAuth(ctx, st, authSvc); err != nil {
			log.Fatalf("初始化角色失败: %v", err)
		}
		if err := seed.Run(ctx, st, svc, seedMed); err != nil {
			log.Fatalf("初始化数据失败: %v", err)
		}
		fmt.Println("种子数据初始化完成")
		return
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	level := slog.LevelInfo
	if cfg.Server.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	st, err := sqlite.Open(cfg.Database.DSN)
	if err != nil {
		slog.Error("打开数据库失败", "error", err)
		os.Exit(1)
	}
	reg, err := i18n.New(cfg.Site.Languages, cfg.Site.DefaultLang, cfg.Site.PrefixDefault)
	if err != nil {
		slog.Error("语言配置错误", "error", err)
		os.Exit(1)
	}
	authSvc := auth.New(st, 7*24*time.Hour)
	svc := content.New(st, schema.NewRegistry(), cfg.Site.Languages)
	if cfg.Translate.Enabled {
		tr := translate.New(cfg.Translate.APIKey, cfg.Translate.BaseURL, cfg.Translate.Model)
		svc.SetTranslator(tr, &cfg.Translate)
	}
	var med media.MediaStore
	if cfg.Media.Driver == "s3" && cfg.Media.S3 != nil {
		med, err = media.NewS3Store(*cfg.Media.S3)
		if err != nil {
			slog.Error("初始化 S3 媒体失败", "error", err)
			os.Exit(1)
		}
	} else {
		med = media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
	}

	loader := theme.NewLoader(cfg.Site.ThemesDir, reg, cfg.Server.Debug)
	srv, err := server.New(cfg, st, svc, reg, loader, authSvc, med)
	if err != nil {
		slog.Error("初始化服务器失败", "error", err)
		os.Exit(1)
	}
	slog.Info("Dulizhan CMS 启动", "addr", cfg.Server.Addr, "site", cfg.Site.Name, "theme", cfg.Site.Theme)
	if err := srv.Run(); err != nil {
		slog.Error("服务器运行失败", "error", err)
		os.Exit(1)
	}
}
