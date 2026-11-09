# 首页产品卡片链接到所属分类归档页 设计文档

- 日期：2026-09-10
- 状态：已确认（用户选择方案 A）
- 关联：`2026-08-13-home-category-sections-design.md`（首页产品/新闻分区数据源）

## 1. 目标

首页「产品中心」分区每张产品卡片的「查看详情」链接，由当前的产品详情页（`/article/<slug>`）改为该产品**所属分类的归档列表页**（`/category/products/<slug...>`）。分类层级由服务端解析，模板只负责渲染。

行为示例：
- `chopper-1`（属 products/chopper）→ `/category/products/chopper`
- `sausage-machine-1`（属 products/sausage-machine）→ `/category/products/sausage-machine`
- 顶级产品（属 products）→ `/category/products`
- 无 `category` 字段或解析失败 → 回退详情页 URL（保持现状）

**仅改首页产品卡片**；新闻卡片、列表页卡片不改（函数可复用，后续需要时再接入）。

## 2. 设计决策（头脑风暴确认）

| 决策点 | 结论 |
|---|---|
| 跳转目标 | 按产品所属分类跳转（用户选择） |
| 实现方式 | 方案 A：新增模板函数 `entryCategoryURL`，服务端解析分类链 |
| 注入方式 | `theme.Loader` 持有解析器，`Load` 时下发给 `Theme`；`server.New` 注入，避免每请求改动共享对象 |
| 解析失败回退 | 回退到详情页 URL（`url` 语义），不产生空链接 |
| 卡片文案 | 保持 `view_more`（不改语言包，避免影响列表页同 key 文案） |

## 3. 改动清单

### 3.1 `internal/theme`（新增注入点 + 模板函数）

`theme.go`：
- `Theme` 加字段 `entryCatURL func(lang string, e content.Entry) string`
- `Loader` 加同名字段与 setter `SetEntryCategoryURL(fn)`；`Load` 时把 loader 的值赋给新 `Theme`。

`funcs.go` — `buildFuncs` 注册：
```go
"entryCategoryURL": func(lang string, e content.Entry) string {
    if t.entryCatURL != nil {
        if u := t.entryCatURL(lang, e); u != "" {
            return u
        }
    }
    return t.urlPath(lang, "/"+e.TypeName+"/"+e.Content.Slug) // 回退详情页
},
```

### 3.2 `internal/server`（解析实现 + 注入）

`frontend.go`：
- 从 `renderSingle` 抽出可复用方法：
```go
// entryCategoryURL 解析 entry 的 category 字段为分类归档 URL；无分类/失败返回 ""。
func (s *Server) entryCategoryURL(lang string, e content.Entry) string
```
  内部复用 `categoryChainInfo` + `categoryURL`（与详情页面包屑同一套逻辑）。
- `renderSingle` 改调用该方法（行为不变，去除重复代码）。

`server.go` 的 `New`：构造 `s` 后调用 `loader.SetEntryCategoryURL(s.entryCategoryURL)`。

### 3.3 `themes/default/templates/index.html`

产品卡片（第 22 行）：
```html
<a class="card" href="{{entryCategoryURL $.Lang .}}">
```
新闻卡片不动。

## 4. 兼容性

- 未注入解析器时 `entryCategoryURL` 回退详情页 URL，等价现状。
- `renderSingle` 重构后行为不变（复用同一解析逻辑）。
- `theme.Data`、`content.Entry` 契约不变；`Fields` 不被污染。
- 现有测试不受影响；新增用例覆盖首页卡片链接。

## 5. 验证

- `go test -count=1 ./...` 全绿；`go build ./...`、`go vet ./...`、`gofmt -l .` 干净
- 新增服务端测试：构造 products/chopper 分类与产品，断言首页 HTML 含 `href="/en/category/products/chopper"`
- 新增 theme 测试：`entryCategoryURL` 有解析器时返回分类 URL、无解析器/无分类时回退详情页
- 冒烟：中英双语首页产品卡片分别指向 `/en/category/...` 与 `/zh/category/...`，点击进入对应分类列表页
