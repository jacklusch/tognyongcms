# API 参考

管理 API 前缀 `/api`，返回 JSON `{code, message, data?}`。认证：`POST /api/auth/login` 拿 token → `Authorization: Bearer <token>`。

## 认证
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /api/auth/login | 公开 | body {username,password} → {token} |
| POST | /api/auth/logout | 登录 | 失效会话 |
| GET | /api/auth/me | 登录 | {user:{id,username,role}, permissions} |

## 内容类型
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | /api/content-types | content.read | 类型列表 |
| POST | /api/content-types | content_types.manage | 建类型 |
| PUT | /api/content-types/:id | content_types.manage | 改类型（name 不可变） |
| DELETE | /api/content-types/:name | content_types.manage | 级联删类型+内容 |

## 内容
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | /api/content | content.read.<type> | 列表（type/lang/status/page） |
| POST | /api/content | content.write | 建内容 |
| GET | /api/content/:id | content.read.<type> | 详情 |
| PUT | /api/content/:id | content.write | 更新 |
| DELETE | /api/content/:id | content.delete | 删除 |
| POST | /api/content/:id/publish | content.publish | 发布 |
| POST | /api/content/:id/unpublish | content.publish | 撤回 |
| GET | /api/content/:id/translations | content.read.<type> | 翻译列表 |
| POST | /api/content/:id/translate | content.write | 建翻译 |

## 分类
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | /api/categories | content.read | 分类树 + 扁平路径（见下） |
| POST | /api/categories | content_types.manage | 建分类（body 见下） |
| PUT | /api/categories/:id | content_types.manage | 改分类（含上级分类，防环） |
| DELETE | /api/categories/:id | content_types.manage | 删分类（有子分类或内容 → 403） |

### GET /api/categories 响应
`{items, all}`：
- `items` —— 分类树，根节点含 `children` 递归嵌套；每节点：`{id, parent_id, name, slug, description, content_count, children}`
- `all` —— 扁平路径，供前端级联选择器：`[{id, path}]`，`path` 为父链拼接（如 `产品/斩拌机`，顶级为 `产品`）

### POST/PUT body
`{name, slug?, description?, parent_id?}`
- `slug` 可省略——空则自动由名称生成（小写连字符），撞车自动加 `-2/-3` 后缀；显式填写的 slug 撞车则 422"分类 slug 已存在"
- `parent_id` 省略/0 表示顶级；非 0 须指向存在的分类
- PUT 校验防环：上级分类不能是自身或其子孙（否则 422）
- 中文名不填 slug 会 422（slugify 中文为空，需手动填 ASCII slug）

### 删除规则
先查子分类（`children` 非空）→ 403"该分类下仍有子分类，请先删除子分类"；再查内容数（>0）→ 403"该分类下仍有内容"。

## 媒体 / 设置 / 菜单 / 用户 / 角色 / 搜索 / 统计
| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /api/media/upload | media.upload | 上传 |
| GET | /api/media | media.upload | 列表 |
| DELETE | /api/media/:id | media.delete | 删除 |
| GET/PUT | /api/settings | settings.manage | 设置读写 |
| GET/POST | /api/menus | menus.manage | 菜单列表/新建 |
| PUT/DELETE | /api/menus/:id | menus.manage | 改/删菜单 |
| GET/POST | /api/users | users.manage | 用户列表/新建 |
| DELETE | /api/users/:id | users.manage | 删用户（删自己/内置 admin/最后 admin → 403） |
| PUT | /api/users/:id/role | users.manage | 改角色（改自己角色 → 403） |
| PUT | /api/users/:id/password | users.manage | 改密码（自己需旧密码 old_password） |
| GET | /api/roles | roles.manage | 角色列表 |
| POST | /api/roles | roles.manage | 建角色 |
| PUT/DELETE | /api/roles/:id | roles.manage | 改/删角色（内置 admin/editor/author 不可改名、不可删；有用户引用拒删） |
| GET | /api/roles/perms | roles.manage | 权限点列表 |
| GET | /api/search | content.read.<type> | 搜索（type/lang/q） |
| GET | /api/stats | content.read | 仪表盘统计 |
| GET | /api/meta | 登录 | 站点配置（语言/主题） |

## 错误
`{code, message}`：401 未登录 / 403 无权限 / 404 不存在 / 422 校验失败 / 400 参数错误 / 500 服务器错误。

## 类型级权限
权限点支持前缀：`content.read.article` 命中 `content.read.*`、`content.*`、`*`（admin）。路由中间件统一要求基础权限（如 `content.read`），handler 内再按类型校验 `content.read.<type>`——上表权限列为有效权限。角色经 `GET /api/roles/perms` 查看全部权限点。
