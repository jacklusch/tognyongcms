# 首页分区按分类取数据 设计文档

- 日期：2026-08-13
- 状态：已确认（用户逐节审阅通过）
- 关联：`2026-08-12-buhler-theme-design.md`（Bühler 主题，本需求为其首页数据源扩展）

## 1. 目标

首页"产品中心"与"新闻动态"两个分区不再显示 article 全量内容，改为分别取管理后台**分类管理**中"产品""新闻"分类（及其子分类）的内容。导航"首页/新闻/关于/产品"已通过菜单管理指向对应分类页（需求已满足，不改）。**样式零改动**。

## 2. 设计决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 首页分类识别 | **配置化 slug**：config 加 `site.home_products_category` / `site.home_news_category`，默认空（不启用过滤，保持向后兼容） |
| 允许后台改动 | 是（用户确认最小后台改动） |
| 分类内容聚合 | 复用现有 `CategoryRepo.Descendants`（递归子分类）+ `content.ListPublishedByCategories`，与分类归档页聚合规则一致（父分类含子孙分类内容） |
| 样式 | index.html 只换数据源（`.Items` → `.Products`/`.News`），CSS/JS/结构不动 |
| 回退 | 分类未配置/不存在/无内容时回退 `.Items`（行为与现状一致） |

## 3. 改动清单

### 3.1 后台

**`internal/config/config.go`** — `SiteConfig` 加两个可选字段：
```go
HomeProductsCategory string `yaml:"home_products_category"`
HomeNewsCategory     string `yaml:"home_news_category"`
```
env 覆盖：`DULIZHAN_SITE_HOME_PRODUCTS_CATEGORY` / `DULIZHAN_SITE_HOME_NEWS_CATEGORY`。

**`internal/theme/theme.go`** — `Data` 加两个字段：
```go
Products []content.Entry // 首页产品中心（产品分类聚合，含子分类）
News     []content.Entry // 首页新闻动态
```

**`internal/server/frontend.go`** — `renderHome` 增加分类聚合：
```
slug = cfg.Site.HomeProductsCategory（非空时）
  → CategoryRepo.GetBySlug → Descendants 聚合 ids → ListPublishedByCategories(article, lang, ids, 1, 6) → data.Products
slug = cfg.Site.HomeNewsCategory（非空时）
  → 同上（取 3 条）→ data.News
```
分类不存在/查询失败时静默留空（回退机制兜底）。

### 3.2 主题

**`themes/default/templates/index.html`** — 数据源替换（样式零改动）：
- 产品中心：`slice .Products 0 6`（`.Products` 为空时回退 `slice .Items 0 6`）
- 新闻动态：`slice .News 0 3`（`.News` 为空时回退 `slice .Items 0 3`）

### 3.3 配置示例

`config.example.yaml` 加注释示例：
```yaml
site:
  home_products_category: "products"   # 首页产品中心取该分类（含子分类）的内容
  home_news_category: "news"           # 首页新闻动态取该分类（含子分类）的内容
```

## 4. 兼容性

- 未配置两个 slug 时 `renderHome` 行为与现在完全一致（`.Items` 全量 + 模板回退）
- 现有 `theme.Data` 使用方（list/single/404）不受影响
- `go test ./...` 全绿（config/theme/server 测试需新增用例覆盖）

## 5. 验证

- `go test -count=1 ./...` 全绿；`go vet ./...`、`gofmt -l .` 干净
- 配置后冒烟：首页产品中心只显示产品分类（含子分类）内容、新闻动态只显示新闻分类内容；未配置时回退全量
- 中英双语分别验证
