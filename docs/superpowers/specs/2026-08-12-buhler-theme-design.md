# Bühler 风格前台主题设计（themes/default 改造）

- 日期：2026-08-12
- 状态：已确认（用户逐节审阅通过）
- 范围：仅 `themes/default/`，后台代码与逻辑不变

## 1. 目标

在不改动任何后台代码/逻辑的前提下，参考 https://www.buhlergroup.com/global/en/homepage.html 的风格与布局，将默认主题改造成"经典 Bühler 工业风"：白底黑字、Bühler 红强调、深色导航与页脚、全宽 Hero 大图、产品卡片网格、深色单栏页脚。

## 2. 关键约束

- **只改 `themes/default/`**：`theme.yaml` 的 `templates` / `partials` / `type_map` / `locales` key 保持原样，后台渲染流程零改动。
- **数据契约不变**：模板只消费 `theme.Data` 现有字段（`.Site/.Lang/.Langs/.Items/.Entry/.Menus/.SubCategories/.EntryCategory/.EntryCategoryURL` 等）。
- **验证命令**：`go test -count=1 ./...`、`go vet ./...`、`gofmt -l .` 必须干净；冒烟覆盖 首页 / 列表 / 详情 / 分类归档 四类页面。

## 3. 设计决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 实现范围 | 直接改造 `themes/default`（不新建主题） |
| 整体风格 | A 经典 Bühler 风：深色导航条 + 全宽 Hero + 红标题 + 产品卡片网格 + 深色页脚 |
| 强调色 | Bühler 红 `#E31E24` |
| 首页产品入口 | 内容驱动：`.Items`（article 类型已发布内容，含 cover）渲染卡片，产品名/图/链接全部来自后台内容 |
| 页脚 | 深色单栏简洁：footer 菜单 + 版权行 |
| 语言切换器 | 要。导航栏顶栏右侧，用 `.Langs` + `.Meta.HrefLangs` 渲染 |
| 导航下拉 | 简化深色下拉面板（两级 `.Children`，纯链接） |
| 首页 Hero 大图 | 主题内置静态图（`static/img/hero.svg` 手写深色工业纹理；CSS 渐变作回退），用户可自行替换同名文件 |

## 4. 文件清单

```
themes/default/
├── theme.yaml              # 不变
├── templates/
│   ├── index.html          # 重写：深色顶栏+导航 / Hero / 产品中心 / 新闻动态 / 页脚
│   ├── list.html           # 重写：页头 / 子分类 chip / 卡片网格 / 分页
│   ├── single.html         # 重写：封面 / 分类 chip / 标题 / 时间 / 正文
│   └── partials/
│       ├── nav.html        # 重写：深色导航 + 语言切换 + 两级下拉
│       └── footer.html     # 重写：深色单栏页脚
├── static/
│   ├── css/main.css        # 重写：Bühler 风全量样式（含响应式）
│   ├── js/main.js          # 重写：移动端菜单 / 下拉点击切换 / 返回顶部
│   └── img/hero.svg        # 新增：Hero 背景（手写 SVG 深色工业纹理；CSS 渐变作回退）
└── locales/
    ├── zh.yaml             # 增加文案 key（见 §7）
    └── en.yaml             # 对应英文
```

## 5. 模板与数据映射

### 5.1 首页 `index.html`（数据：`.Site` / `.Items` / `.Menus` / `.Langs`）

```
深色顶栏：站点名/描述（左）+ 语言切换 .Langs（右，链到 .Meta.HrefLangs 对应 URL）
深色导航：logo .Site.Name（左）+ 主导航 firstMenu（右，有 Children 渲染深色下拉面板）
Hero 大图：static/img/hero.svg 全宽背景 + 深色遮罩 + 大标题（locales 文案，大写）
          + 副标题（.Site.Description）+ CTA 按钮（typeURL "article"）
产品中心：.Items 循环前 6 条 → 卡片（cover 图 + Title + excerpt + url 链接）
新闻动态：.Items 循环前 3 条 → 横排新闻卡（Title + PublishedAt）
深色页脚：menu "footer" 链接 + 版权行（.Site.Name）
```

> 说明：模板取 `.Items` 前 N 条用 Go 模板内置 `slice` 函数（`slice .Items 0 6`），`themes/default` 现有模板函数集不含它，但 `text/template` 内置函数自带，无需后台改动。
```

### 5.2 列表页 `list.html`（数据：`.TypeName` / `.Items` / `.SubCategories` / `.Total` / `.Page` / `.PerPage`）

```
页头：类型名标题 + 红色下划线
子分类：.SubCategories 渲染 chip（Name → URL）
内容网格：.Items 循环 → 卡片（cover + Title + excerpt + url）
分页：.Total/.Page/.PerPage + pages/pagerURL（TypeName 为 category 时隐藏，沿用现有约定）
```

### 5.3 详情页 `single.html`（数据：`.Entry` / `.EntryCategory` / `.EntryCategoryURL`）

```
封面图：.Entry.Fields.cover（media 函数）
分类 chip：.EntryCategory（链到 .EntryCategoryURL）
标题：.Entry.Content.Title
时间：.Entry.Content.PublishedAt
正文：rich 字段用 raw 输出（沿用现有 single 约定）
```

### 5.4 `nav.html` / `footer.html`

- `nav.html`：深色导航条。语言切换在顶栏或导航右侧：遍历 `.Langs`，非当前语言渲染链接。链接来源：`.Meta.HrefLangs`（每个语言对应当前页 URL，跳过 `x-default`）；无 `.Meta.HrefLangs` 时回退 `home`。
- `footer.html`：深色背景，`menu "footer"`（用 `menu $ "footer"`）渲染链接，底部版权行。

## 6. 视觉规范

```
配色：
  --red    #E31E24   强调（Hero CTA、标题下划线、卡片 hover、链接、页脚 hover）
  --ink    #111      正文
  --dark   #1a1a1a   导航/页脚背景
  --gray   #f5f5f5   分区背景 / 图片占位
  --muted  #666      次要文字
  --line   #e8e8e8   分隔线 / 卡片边框

字体：Helvetica Neue / PingFang SC / Microsoft YaHei 无衬线
  Hero 标题：900 weight、clamp(40px,6vw,64px)、大写、深色遮罩上的白色
  区块标题：32px、800 weight、大写、左对齐 + 64×4px 红色下划线短条
  卡片标题：18px、700；正文 14px --muted

布局：
  内容容器：max-width 1200px 居中，左右 24px padding
  首页分区：产品中心（白底）→ 新闻动态（浅灰底）
  页脚：深色，链接居中单栏 + 版权行

响应式（≤900px）：
  网格降为单列；导航隐藏、显示汉堡按钮（js/main.js 切换）
  保持触屏可用：下拉 hover → 点击切换
```

## 7. 文案（locales）

`zh.yaml` 新增 key：

```yaml
products: 产品中心
news: 新闻动态
view_more: 查看详情
learn_more: 了解更多
home: 首页
about: 关于我们
contact: 联系我们
copyright: © %s · 保留所有权利
```

`en.yaml` 对应英文。原有 key（`latest`/`articles`/`no_content`/`home`）保留以兼容 fallback。

## 8. 前端行为（static/js/main.js）

- 移动端汉堡按钮切换导航展开/收起。
- 桌面端下拉 CSS `:hover` 触发；触屏环境点击切换。
- 页脚"返回顶部"链接平滑滚动。
- 全部为渐进增强：无 JS 时 CSS 菜单仍可用。

## 9. 明确不做（YAGNI）

1. 不改 `theme.yaml` 模板 key / `type_map`，后台渲染零改动。
2. 不新增后台字段 / 接口 / 数据模型。
3. 不做滚动动画、懒加载、图片灯箱等非 SEO 增值项，保持服务端渲染。
4. 不写死产品名——产品名/图/链接全部来自后台 article 内容。
5. 不做多语言 Mega Menu 缩略图（主题无此类数据）。

## 10. 测试与验证

- `go test -count=1 ./...` 全绿（`themes/default/theme_test.go` 与 `internal/theme` 测试不依赖模板结构，应不变）。
- `go vet ./...`、`gofmt -l .` 干净。
- 冒烟（确认 8080 无残留进程）：`server.debug: true` 启动，检查：
  - `/` 首页：Hero、产品卡片、新闻、深色页脚渲染正常
  - `/article` 列表：页头、卡片网格、分页
  - `/article/{slug}` 详情：封面、标题、正文
  - `/category/{slug}` 分类归档：子分类 chip + 卡片 + 无分页
  - 中/英两种语言切换链接正确
  - 响应式 900px 断点：导航折叠 + 汉堡菜单

## 11. 风险与缓解

| 风险 | 缓解 |
|---|---|
| Hero 静态图风格与内容不匹配 | 使用中性的深色工业 SVG 纹理；替换仅需覆盖 `static/img/hero.svg` |
| 语言切换 URL 需与当前页一致 | 用 `.Meta.HrefLangs`（seo 已为每页生成各语言 URL，含 x-default），过滤 x-default 后渲染 |
| 列表页 `TypeName=category` 时 pagination 逻辑 | 沿用现有 `ne $.TypeName "category"` 约定，不新增逻辑 |
| 卡片网格条目不足 | 用 `slice` 限制前 N 条；无图时 `media` 返回空串 → 用 `--gray` 占位 |
