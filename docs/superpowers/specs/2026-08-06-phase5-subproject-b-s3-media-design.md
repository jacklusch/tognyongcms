# Dulizhan CMS — 阶段 5 子项目 B：S3 媒体实现 设计

- 日期：2026-08-06
- 状态：已确认（用户逐节审阅通过）
- 前置文档：`docs/superpowers/specs/2026-08-05-cms-design.md`（§7 媒体存储）、`2026-08-06-phase4-content-type-builder-design.md`
- 基线：阶段 4 已验收通过；`MediaStore` 接口 + `LocalStore` 已存在

## 1. 目标与范围

`media.driver: s3` 时用 aws-sdk-go-v2 实现 `MediaStore` 接口（Save/Delete/URL），兼容 AWS S3 / MinIO / 阿里 OSS。

**包含**：S3Store 实现、config media.s3 块、装配切换、mock client 单测。

**排除**：S3 缩略图、生命周期管理、预签名 URL、分片上传（YAGNI）。

## 2. config 扩展

### 2.1 MediaConfig（`internal/config/config.go`）

```go
type MediaConfig struct {
	Driver string   `yaml:"driver"`
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
```

env 覆盖：`DULIZHAN_MEDIA_S3_ENDPOINT`、`DULIZHAN_MEDIA_S3_REGION`、`DULIZHAN_MEDIA_S3_BUCKET`、`DULIZHAN_MEDIA_S3_ACCESS_KEY`、`DULIZHAN_MEDIA_S3_SECRET_KEY`、`DULIZHAN_MEDIA_S3_PUBLIC_BASE_URL`。

`Validate()`：`driver=="s3"` 时校验 S3 配置非空（bucket 必填）。

### 2.2 config.yaml 示例

```yaml
media:
  driver: "s3"
  s3:
    endpoint: ""
    region: "cn-hangzhou"
    bucket: "my-bucket"
    access_key: "xxx"
    secret_key: "yyy"
    public_base_url: ""
```

## 3. S3Store 实现（`internal/media/s3.go`）

```go
package media

type S3Store struct {
	client     *s3.Client
	bucket     string
	publicBase string
}

func NewS3Store(cfg S3Config) (*S3Store, error) {
	// aws.Config + credentials.NewStaticCredentialsProvider
	// endpoint 非空 → 用 custom endpoint（WithBaseEndpoint，支持 MinIO/OSS）
}

func (s *S3Store) Save(ctx context.Context, key string, r io.Reader, contentType string) (string, error) {
	// PutObject: Bucket=s.bucket, Key=key, Body=r, ContentType=contentType
	// 返回 s.URL(key)
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	// DeleteObject
}

func (s *S3Store) URL(key string) string {
	if s.publicBase != "" {
		return strings.TrimRight(s.publicBase, "/") + "/" + key
	}
	// 默认构造：https://<bucket>.s3.<region>.amazonaws.com/<key>
	// （endpoint 非空时用 endpoint 拼接：strings.TrimRight(endpoint,"/") + "/" + bucket + "/" + key）
}
```

**装配**（`internal/server/server.go` + `cmd/dulizhan/main.go`）：
```go
var med media.MediaStore
if cfg.Media.Driver == "s3" && cfg.Media.S3 != nil {
	med, err = media.NewS3Store(*cfg.Media.S3)
} else {
	med = media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
}
```
现有 adminapi/server 消费 `media.MediaStore` 接口，无需改动。

## 4. 依赖

`go get github.com/aws/aws-sdk-go-v2/service/s3@v1.71.0 github.com/aws/aws-sdk-go-v2/config@v1.28.0 github.com/aws/aws-sdk-go-v2/credentials@v1.17.0`

- **版本约束**：aws-sdk-go-v2 需兼容 Go 1.22（v1.71.0/config v1.28.0 等当前版本要求 go 1.20+，兼容）。**实现时**以 `go get` 解析到的实际兼容版本为准（go.mod 的 `go 1.22.2` 约束），锁定后验证 go.mod 中 gin/sqlite 未变。**勿运行 `go mod tidy`**。
- 若 `go get` 拉取失败，走 goproxy.cn 镜像（`$env:GOPROXY="https://goproxy.cn,direct"`）。

## 5. 测试

`internal/media/s3_test.go`（mock client）：
- 注入 fake S3 client（实现 PutObject/DeleteObject 接口），断言：
  - Save 调 PutObject 且 key/contentType 正确传递，返回 URL（public_base_url 优先）
  - Delete 调 DeleteObject
  - URL：public_base_url 非空 → 前缀拼接；空 → bucket/endpoint 拼接
- config 层：`Validate` s3 driver 缺 bucket → 错误

## 6. 验收

- `go test -count=1 ./internal/media/ ./internal/config/ -v` 全绿
- `go test -count=1 ./...` 全绿、build/vet/gofmt 干净
- 冒烟（可选）：配置 s3 driver 启动不报错（mock 或真实 MinIO）；前端媒体库上传走 S3（若环境有）

## 7. 工作流约定

- 不 git 提交；superpowers SDD 驱动
