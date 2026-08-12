package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dulizhan/internal/auth"
	"dulizhan/internal/content"
	"dulizhan/internal/media"
	"dulizhan/internal/schema"
	"dulizhan/internal/store"
)

// Run 创建内置内容类型、分类与演示内容（幂等：已存在则跳过）。
func Run(ctx context.Context, st store.Store, svc *content.Service, med media.MediaStore) error {
	article := schema.ContentType{
		Name:  "article",
		Label: "文章",
		Fields: []schema.Field{
			{Name: "title", Label: "标题", Type: schema.TypeText, Required: true, Indexed: true, Translatable: true},
			{Name: "slug", Label: "别名", Type: schema.TypeSlug, Translatable: true},
			{Name: "excerpt", Label: "摘要", Type: schema.TypeTextarea, Translatable: true},
			{Name: "content", Label: "正文", Type: schema.TypeRichText, Required: true, Translatable: true},
			{Name: "cover", Label: "封面图", Type: schema.TypeImage},
			{Name: "published_on", Label: "发布日期", Type: schema.TypeDate},
			{Name: "category", Label: "分类", Type: schema.TypeRelation, RelationType: "category"},
		},
	}
	if _, err := svc.GetType(ctx, "article"); err != nil {
		if err := svc.CreateType(ctx, &article); err != nil {
			return fmt.Errorf("创建内容类型 article: %w", err)
		}
	}

	page := schema.ContentType{
		Name:  "page",
		Label: "页面",
		Fields: []schema.Field{
			{Name: "title", Label: "标题", Type: schema.TypeText, Required: true, Indexed: true, Translatable: true},
			{Name: "slug", Label: "别名", Type: schema.TypeSlug, Translatable: true},
			{Name: "content", Label: "正文", Type: schema.TypeRichText, Required: true, Translatable: true},
		},
	}
	if _, err := svc.GetType(ctx, "page"); err != nil {
		if err := svc.CreateType(ctx, &page); err != nil {
			return fmt.Errorf("创建内容类型 page: %w", err)
		}
	}

	// 先建 zh，再把 en 建为同组翻译（同 content_id），保证多语言切换可用。
	if _, err := svc.GetPublishedBySlugLang(ctx, "article", "hello-zh", "zh"); err != nil {
		e, err := svc.Create(ctx, "article", "zh", map[string]any{
			"title": "你好，世界", "slug": "hello-zh",
			"excerpt": "这是第一篇中文示例文章", "content": "<p>欢迎使用 Dulizhan CMS。</p>",
		}, 0)
		if err != nil {
			return fmt.Errorf("创建示例文章 hello-zh: %w", err)
		}
		if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
			return err
		}
		// en 为 zh 的同组翻译（同 content_id 翻译组）
		te, err := svc.CreateTranslation(ctx, "article", "en", e.Content.ContentID, map[string]any{
			"title": "Hello World", "slug": "hello-en",
			"excerpt": "The first sample article", "content": "<p>Welcome to Dulizhan CMS.</p>",
		}, content.Actor{UserID: 0, IsModerator: true})
		if err != nil {
			return fmt.Errorf("创建示例文章 hello-en 翻译: %w", err)
		}
		if err := svc.SetStatus(ctx, te.Content.ID, "published"); err != nil {
			return err
		}
	}
	// hello-zh/en 归入 news 分类（分类创建后设置）
	if err := seedDemoData(ctx, st, svc, med); err != nil {
		return err
	}
	return nil
}

// seedDemoData 建分类、双语演示文章与 main 菜单（幂等）。
func seedDemoData(ctx context.Context, st store.Store, svc *content.Service, med media.MediaStore) error {
	demoCategories := []struct{ Name, Slug string }{
		{"新闻", "news"}, {"关于", "about"}, {"产品", "products"},
	}
	catIDs := map[string]int64{}
	for _, c := range demoCategories {
		if existing, err := st.CategoryRepo().GetBySlug(ctx, c.Slug); err == nil {
			catIDs[c.Slug] = existing.ID
			continue
		}
		cat := &store.Category{Name: c.Name, Slug: c.Slug}
		if err := st.CategoryRepo().Create(ctx, cat); err != nil {
			return fmt.Errorf("创建分类 %s: %w", c.Slug, err)
		}
		catIDs[c.Slug] = cat.ID
	}

	// 演示文章（每分类 2-3 篇，zh 先建 + en 同组翻译）
	type demoArticle struct {
		Slug    string
		ZhTitle string
		ZhBody  string
		EnTitle string
		EnBody  string
	}
	demoContent := map[string][]demoArticle{
		"news": {
			{"news-1", "公司发布新一代产品", "<p>我们很高兴宣布新一代产品正式发布，带来更强大的性能与更友好的体验。</p>", "Company Launches Next-Gen Product", "<p>We are excited to announce the launch of our next-generation product with improved performance and UX.</p>"},
			{"news-2", "与行业伙伴达成战略合作", "<p>本次合作将整合双方优势，为用户提供更完整的解决方案。</p>", "Strategic Partnership Announced", "<p>This partnership combines our strengths to deliver a more complete solution.</p>"},
		},
		"about": {
			{"about-1", "关于我们", "<p>我们是一支专注创新的团队，致力于用技术创造价值。</p>", "About Us", "<p>We are an innovative team dedicated to creating value through technology.</p>"},
		},
		"products": {
			{"products-1", "旗舰产品一览", "<p>我们的旗舰产品系列涵盖多种场景，满足不同需求。</p>", "Flagship Products Overview", "<p>Our flagship product line covers multiple scenarios.</p>"},
			{"products-2", "新品评测", "<p>第三方评测对新产品给出了高度评价。</p>", "New Product Review", "<p>Third-party reviewers gave high marks to our new product.</p>"},
		},
	}
	for catSlug, arts := range demoContent {
		for _, a := range arts {
			if _, err := svc.GetPublishedBySlugLang(ctx, "article", a.Slug, "zh"); err == nil {
				continue // 已存在
			}
			cover := downloadCover(ctx, med, "https://picsum.photos/seed/"+a.Slug+"/800/500")
			e, err := svc.Create(ctx, "article", "zh", map[string]any{
				"title": a.ZhTitle, "slug": a.Slug,
				"excerpt": firstSentence(a.ZhBody), "content": a.ZhBody,
				"category": fmt.Sprintf("%d", catIDs[catSlug]),
				"cover":    cover,
			}, 0)
			if err != nil {
				return fmt.Errorf("创建演示文章 %s: %w", a.Slug, err)
			}
			if err := svc.SetStatus(ctx, e.Content.ID, "published"); err != nil {
				return err
			}
			te, err := svc.CreateTranslation(ctx, "article", "en", e.Content.ContentID, map[string]any{
				"title": a.EnTitle, "slug": a.Slug + "-en",
				"excerpt": firstSentence(a.EnBody), "content": a.EnBody,
				"category": fmt.Sprintf("%d", catIDs[catSlug]),
				"cover":    cover,
			}, content.Actor{UserID: 0, IsModerator: true})
			if err != nil {
				return fmt.Errorf("创建演示翻译 %s-en: %w", a.Slug, err)
			}
			if err := svc.SetStatus(ctx, te.Content.ID, "published"); err != nil {
				return err
			}
		}
	}

	// hello-zh/en 归入 news 分类（若未设分类）
	if e, err := svc.GetPublishedBySlugLang(ctx, "article", "hello-zh", "zh"); err == nil {
		if e.Fields["category"] == nil {
			// 更新 payload 加分类——用 svc.Update 重存
			fields := e.Fields
			fields["category"] = fmt.Sprintf("%d", catIDs["news"])
			if _, err := svc.Update(ctx, e.Content.ID, fields); err != nil {
				return err
			}
		}
	}
	if e, err := svc.GetPublishedBySlugLang(ctx, "article", "hello-en", "en"); err == nil {
		if e.Fields["category"] == nil {
			fields := e.Fields
			fields["category"] = fmt.Sprintf("%d", catIDs["news"])
			if _, err := svc.Update(ctx, e.Content.ID, fields); err != nil {
				return err
			}
		}
	}

	// main 菜单（zh/en 幂等）
	if err := seedMainMenu(ctx, st, "zh"); err != nil {
		return err
	}
	if err := seedMainMenu(ctx, st, "en"); err != nil {
		return err
	}
	return nil
}

// seedMainMenu 建 zh/en 的 main 菜单（幂等）。
func seedMainMenu(ctx context.Context, st store.Store, lang string) error {
	if menus, err := st.MenuRepo().ListByLang(ctx, lang); err == nil {
		for _, m := range menus {
			if m.Name == "main" {
				return nil // 已有
			}
		}
	}
	labels := map[string]struct{ Zh, En string }{
		"news":     {"新闻", "News"},
		"about":    {"关于", "About"},
		"products": {"产品", "Products"},
	}
	home := "首页"
	if lang == "en" {
		home = "Home"
	}
	items := []store.MenuItem{{Label: home, Type: "home", URL: ""}}
	for _, slug := range []string{"news", "about", "products"} {
		label := labels[slug].Zh
		if lang == "en" {
			label = labels[slug].En
		}
		items = append(items, store.MenuItem{Label: label, Type: "custom", URL: "/category/" + slug})
	}
	itemsJSON, _ := json.Marshal(items)
	return st.MenuRepo().Create(ctx, &store.Menu{Name: "main", Lang: lang, Items: string(itemsJSON)})
}

// firstSentence 取正文第一句作为摘要（剥 HTML 标签的简化版）。
func firstSentence(s string) string {
	if i := strings.Index(s, "。"); i > 0 {
		s = s[:i+1]
	}
	s = strings.TrimPrefix(s, "<p>")
	s = strings.TrimSuffix(s, "</p>")
	return s
}

// EnsureAuth 幂等创建内置三角色与默认管理员（admin/admin123）。
func EnsureAuth(ctx context.Context, st store.Store, authSvc *auth.Service) error {
	roles := map[string][]string{
		"admin":  {"*"}, // admin 拥有通配权限，绕过所有检查
		"editor": {"content.*", "content_types.manage", "media.upload", "media.delete", "menus.manage", "settings.manage"},
		"author": {"content.read", "content.write", "media.upload"},
	}
	// 固定创建顺序，保证 admin 始终 id 最小（首个创建）
	roleOrder := []string{"admin", "editor", "author"}
	roleIDs := map[string]int64{}
	for _, name := range roleOrder {
		perms := roles[name]
		if r, err := st.RoleRepo().GetByName(ctx, name); err == nil {
			roleIDs[name] = r.ID
			continue
		}
		r, err := st.RoleRepo().Create(ctx, name)
		if err != nil {
			return err
		}
		if err := st.RoleRepo().Update(ctx, r.ID, name, mustJSON(perms)); err != nil {
			return err
		}
		roleIDs[name] = r.ID
	}
	if _, err := st.UserRepo().GetByUsername(ctx, "admin"); err != nil {
		if _, err := authSvc.CreateUser(ctx, "admin", "admin123", roleIDs["admin"]); err != nil {
			return err
		}
	}
	return nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
