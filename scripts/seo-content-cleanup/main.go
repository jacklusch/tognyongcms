// 一次性脚本：删除测试内容（按 slug 匹配），供 SEO 审计后清理。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"dulizhan/internal/store/sqlite"
)

// 待删除的 slug（含 en 翻译组一并删）
var testSlugs = map[string]bool{
	"tiktok": true, "youtube": true, "news003": true, "news": true,
	"news-004": true, "news-005": true, "news-006": true, "news-007": true,
	"news-008": true, "news-009": true, "news-010": true, "run-test-1": true,
	"post-21": true, "chopper-mixer-test-data": true,
	// 用户确认的额外测试页（dry-run 核对发现）
	"news-01": true, "news-02": true, "news-09": true, "required-ok-1": true,
}

func main() {
	dry := flag.Bool("dry-run", false, "只打印命中，不删除")
	flag.Parse()
	if len(flag.Args()) < 1 {
		fmt.Fprintln(os.Stderr, "用法: go run ./scripts/seo-content-cleanup [-dry-run] <sqlite-dsn>")
		os.Exit(2)
	}
	ctx := context.Background()
	st, err := sqlite.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "打开库:", err)
		os.Exit(1)
	}
	types, err := st.ContentTypeRepo().List(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "列出类型:", err)
		os.Exit(1)
	}
	deleted := 0
	for _, ct := range types {
		for _, lang := range []string{"zh", "en"} {
			rows, err := st.ContentRepo().ListByTypeLangStatus(ctx, ct.Name, lang, "", 0, 100000)
			if err != nil {
				fmt.Fprintln(os.Stderr, "列出内容:", err)
				continue
			}
			for _, row := range rows {
				if !testSlugs[row.Slug] {
					continue
				}
				if *dry {
					fmt.Printf("命中待删: %s/%s lang=%s id=%d\n", ct.Name, row.Slug, row.Lang, row.ID)
					continue
				}
				group, err := st.ContentRepo().ListByContentID(ctx, row.ContentID)
				if err != nil {
					fmt.Fprintln(os.Stderr, "查内容组:", err)
					continue
				}
				for _, g := range group {
					if err := st.ContentRepo().Delete(ctx, g.ID); err != nil {
						fmt.Fprintf(os.Stderr, "删除 %s/%s (id=%d): %v\n", ct.Name, g.Slug, g.ID, err)
						continue
					}
					fmt.Printf("已删除 %s/%s (lang=%s, id=%d)\n", ct.Name, g.Slug, g.Lang, g.ID)
					deleted++
				}
			}
		}
	}
	fmt.Printf("共删除 %d 条\n", deleted)
}
