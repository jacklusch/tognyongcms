# 规格：分类子分类功能（2026-08-12）

## 1. 背景与目标

现有分类系统为单层平铺：`categories` 表（name/slug/description），内容 `payload.category` 存分类 id，前台 `/category/:slug` 归档，管理端平铺表格 CRUD。

本规格为分类增加**子分类（树形层级）**能力。典型场景：产品分类下建 斩拌机 / 香肠机 / 拌馅机 子分类。

目标：
- 支持任意层级分类树（parent 自引用）
- 后台分类管理支持对子分类的增删改查
- 前台正确渲染子分类（两级路径 URL、父分类聚合子孙内容、子分类导航入口）

## 2. 需求决策（已与用户确认）

| 决策点 | 结论 |
|---|---|
| 子分类前台 URL | 两级路径 `/category/products/chopper`（任意层级递归拼接） |
| 层级深度 | 任意层级（parent_id 自引用），slug 全局唯一 |
| 归档聚合 | 父分类页**递归聚合所有子孙分类的内容**；子分类页同理（递推一致） |
| 管理端交互 | 树形表格 + 行内"添加子分类"；新建/编辑对话框含"上级分类"级联选择 |
| 删除父分类 | 有子分类则禁止删除（有内容也禁止，与现状一致） |
| 内容分类选择器 | 级联选择器，可选中任意层级节点 |
| 前台导航 | 仍由菜单系统控制；seed 补子分类菜单项，用户可手动配置 |
| slug 生成 | 后台新建不填 slug 自动 slugify（重名加后缀），可手动改 |

## 3. 数据模型与 store 层

### 3.1 categories 表加列

`internal/store/sqlite/migrate.go` 的 schema 后追加守卫语句（存量库升级幂等）：

```sql
ALTER TABLE categories ADD COLUMN parent_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_categories_parent ON categories (parent_id);
```

- `parent_id = 0` 表示顶级分类（用 0 而非 NULL，避免 NULL 比较，跨库兼容）
- 不允许悬空 parent_id（创建/更新时校验 parent 存在）
- 防环：parent_id 不能等于自身 id，且不能是自身子孙
- **slug 全局唯一约束不变**

### 3.2 store.Category 模型

`internal/store/model.go`：

```go
type Category struct {
    ID          int64     `json:"id"`
    ParentID    int64     `json:"parent_id"`
    Name        string    `json:"name"`
    Slug        string    `json:"slug"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### 3.3 CategoryRepo 接口扩展

`internal/store/store.go` 在现有接口上新增：

```go
type CategoryRepo interface {
    // 既有
    List(ctx) ([]Category, error)
    GetByID(ctx, id) (Category, error)
    GetBySlug(ctx, slug) (Category, error)
    Create(ctx, c *Category) error
    Update(ctx, c *Category) error
    Delete(ctx, id) error
    CountContent(ctx, id) (int, error)
    // 新增
    ListChildren(ctx, parentID) ([]Category, error) // 直接子分类
    Descendants(ctx, id) ([]Category, error)        // 全部子孙（不含自身），用于聚合与防环
}
```

- `GetBySlug` 保持全局唯一解析（可命中任意层级）
- `List` 返回顺序按 `ORDER BY id`
- `Descendants` 用递归 CTE（SQLite 支持 `WITH RECURSIVE`）

### 3.4 防环校验

创建/更新时若 `parent_id != 0`：
1. parent 记录必须存在（否则 `ErrValidation`）
2. `parent_id != id`
3. parent 不在 `Descendants(id)` 中（用递归查询判据：`Descendants(id)` 是否含 parent_id）

## 4. adminapi 层

### 4.1 categoryReq 扩展

```go
type categoryReq struct {
    Name        string `json:"name"`
    Slug        string `json:"slug"`
    Description string `json:"description"`
    ParentID    int64  `json:"parent_id"` // 0=顶级
}
```

### 4.2 GET /api/categories —— 树形返回

响应：

```json
{
  "items": [{
    "id": 1, "parent_id": 0, "name": "产品", "slug": "products",
    "description": "", "content_count": 3,
    "children": [{
      "id": 5, "parent_id": 1, "name": "斩拌机", "slug": "chopper",
      "description": "", "content_count": 1, "children": []
    }]
  }],
  "all": [{ "id": 1, "path": "产品" }, { "id": 5, "path": "产品/斩拌机" }]
}
```

- 单次 `List` 全部记录，内存组树（O(n)），避免 N+1
- `content_count` 保留在每节点（该分类直接内容数）
- `all` 为扁平数组含层级路径，供内容编辑器级联选择器直接使用（避免级联控件单独再拉一次）

树组装逻辑独立为纯函数 `buildCategoryTree(cats []store.Category) (roots, all)`，可单测。

### 4.3 CRUD 变更

- **Create**：`parent_id` 校验存在；无 slug 自动 slugify（名称转小写连字符），撞车加后缀（`-2`, `-3`…）
- **Update**：同上 + 防环
- **Delete**：`ListChildren(parent_id=id)` 非空 → `ErrForbidden`"该分类下仍有子分类，请先删除子分类"；`CountContent>0` → `ErrForbidden`（现状保留）

权限路由不变（读 `content.read`，写 `content_types.manage`）。

## 5. 前台渲染

### 5.1 路由

`internal/server/frontend.go` 的 `render` 路径分支：`/category/` 后允许任意段数路径：

```go
// segs[0]=="category"，segs[1:] 为分类路径各段
case len(segs) >= 2 && segs[0] == "category":
    s.renderCategory(c, th, lang, segs[1:], data)
```

### 5.2 路径解析

`renderCategory` 逐级解析：第一段用 `GetBySlug` 找顶级，后续段在 `ListChildren(prevID)` 中匹配 slug；任一级找不到 → 404。最终得目标 Category 节点。

### 5.3 递归聚合归档

目标分类 `Descendants(id)` + 自身 → id 列表 → **IN 查询**替代现有单 id LIKE：

```go
// sqlite contentRepo 新增/替换：
ListByCategories(ctx, typeName, lang string, ids []int64, offset, limit int) ([]store.Content, error)
CountByCategories(ctx, typeName, lang string, ids []int64) (int, error)
```

现有 `ListByCategory`/`CountByCategory`（单 id LIKE）删除，`internal/server/frontend.go` 与 seed 调用点同步改。

### 5.4 子分类导航

`renderCategory` 注入 `data.SubCategories`（目标节点直接子分类列表）。`list.html` 顶部渲染子分类导航链接（`/category/products/chopper`），子分类页同样显示自身子分类。

### 5.5 详情页

`renderSingle` 的 `EntryCategory` 保留分类名；新增 `EntryCategoryURL` 指向该分类完整路径 URL（`/category/products/chopper`）。分类名取自 `GetByID` 解析，URL 由父链逐级拼 slug。

### 5.6 SEO

`BuildList` 用目标分类名，逻辑不变。

## 6. 管理端 Vue

### 6.1 CategoriesView.vue —— 树形表格

- `el-table` 树形：`row-key="id"` + `:tree-props="{ children: 'children' }"`，数据直接用 adminapi 返回的 `items` 树
- 每行操作：**添加子分类**（预填 parent_id）+ 编辑 + 删除
- 新建/编辑对话框加"上级分类"`el-cascader`（用扁平 `all`，`value=id`/`label=path`，可清空=顶级）
- 删除前检查 `row.children?.length`（前端拦截，后端 403 双保险）
- 有子分类时删除按钮置灰或点击提示

### 6.2 API 层（category.ts）

- `CategoryItem` 加 `parent_id`、`children: CategoryItem[]`
- `listCategories` 返回 `{ items: CategoryItem[], all: {id, path}[] }`
- `createCategory`/`updateCategory` body 加 `parent_id`

### 6.3 CategoryPicker.vue（内容编辑器）

改级联选择器（用 `all` 扁平树转级联，或直接复用 `all` 展开为级联 options），选中任一节点。

## 7. seed 演示数据

- 产品分类下加子分类：斩拌机（chopper）、香肠机（sausage-machine）、拌馅机（mixer）
- 每个子分类 1 篇双语演示文章（zh/en 同组 + category 指向子分类 id）
- `main` 菜单（zh/en）补子分类菜单项（指向 `/category/products/chopper` 等）
- `DULIZHAN_SEED_NO_DOWNLOAD` 机制保持，幂等规则不变（已存在跳过）

## 8. 测试计划

### 8.1 sqlite（store/sqlite/sqlite_test.go）

- `ListChildren` 按 parent 过滤
- `Descendants` 递归返回全部子孙（含多级）
- 防环：parent 为自身/子孙时报错（该逻辑在 service/adminapi 层，sqlite 只测数据读写）
- `ListByCategories` IN 查询多 id、空 id 列表

### 8.2 adminapi（categories_test.go）

- 树形返回结构正确（children 嵌套、all 带 path）
- CRUD parent_id：创建子分类、更新父分类、更新改 parent 成功
- 防环：parent=自身 / parent=子孙 → 422
- 删除有子分类 → 403；删除无子分类有内容 → 403（现状）
- slug 自动生成 + 撞车加后缀

### 8.3 server（frontend_test.go）

- `/category/products/chopper` 200 且只含子分类内容
- `/category/products` 聚合子孙（含子分类内容）
- 未知路径 `/category/a/b/c` → 404
- 详情页 `EntryCategoryURL` 正确

### 8.4 admin（vitest）

- categories-view.test.ts 适配树形（children 渲染、添加子分类）
- category-picker.test.ts 适配级联

## 9. 迁移与兼容

- 存量库：`ALTER TABLE ... ADD COLUMN parent_id` 守卫自动加列；既有顶级分类 parent_id=0，行为不变
- 现有 `ListByCategory`（单 id LIKE）删除，调用点（frontend.go、seed）全部同步到 `ListByCategories`/`CountByCategories`
- adminapi `HandleCategories` 响应结构变更（items 树 + all），前端同步适配

## 10. 范围外（YAGNI）

- 分类拖拽排序、手动排序
- 父分类页面包屑（仅子分类导航入口）
- 分类树懒加载（一次全量足够）
- 子分类 slug 局部唯一
