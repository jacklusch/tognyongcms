# 阶段 5 子项目 B：S3 媒体实现 计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** `media.driver: s3` 时用 aws-sdk-go-v2 实现 `MediaStore` 接口（Save/Delete/URL），兼容 AWS S3 / MinIO / 阿里 OSS。

**架构：** config 加 `media.s3` 块；`internal/media/s3.go` 实现 `S3Store`；`server/main` 装配按 driver 选择 LocalStore 或 S3Store；mock client 单测。

**技术栈：** Go、aws-sdk-go-v2（service/s3 v1.71.0 + config + credentials）。

**前置基线：** 阶段 4 已验收通过；`MediaStore` 接口 + `LocalStore` 已存在。规格：`docs/superpowers/specs/2026-08-06-phase5-subproject-b-s3-media-design.md`。

**环境约束：**
- **不 git 提交**；**勿运行 `go mod tidy`**
- 网络走 goproxy.cn（已配置）；aws-sdk-go-v2 版本锁定 v1.71.0（Go 1.20+，兼容 go 1.22.2）
- 错误消息用中文；依赖锁定勿动（gin@v1.10.0/sqlite@v1.34.2）

**现状关键点：**
- `internal/media/media.go`：`MediaStore` 接口（Save/Delete/URL）、`LocalStore`、`sanitizeKey`
- `internal/config/config.go`：`MediaConfig{Driver}`（仅 driver，无 S3 块）
- `internal/server/server.go`：装配 `media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")`
- `cmd/dulizhan/main.go`：同装配

---

### 任务 1：config media.s3 块 + Validate

**文件：**
- 修改：`internal/config/config.go`（MediaConfig + S3Config + env + Validate）
- 修改：`internal/config/config_test.go`（补测试）

- [ ] **步骤 1：编写失败测试**（追加到 `internal/config/config_test.go`）

```go
func TestValidateS3Config(t *testing.T) {
	cfg := &Config{}
	cfg.Site.URL = "https://example.com"
	cfg.Site.DefaultLang = "zh"
	cfg.Site.Languages = []string{"zh"}
	cfg.Media.Driver = "s3"
	cfg.Media.S3 = &S3Config{Region: "cn-hangzhou"} // 缺 bucket
	if err := cfg.Validate(); err == nil {
		t.Error("s3 driver 缺 bucket 应报错")
	}
	cfg.Media.S3 = &S3Config{Region: "cn-hangzhou", Bucket: "b"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("s3 driver 配 bucket 应通过: %v", err)
	}
}

func TestLoadS3Config(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `
site:
  url: "https://example.com"
  default_lang: "zh"
  languages: ["zh"]
media:
  driver: "s3"
  s3:
    region: "cn-hangzhou"
    bucket: "my-bucket"
    access_key: "ak"
    secret_key: "sk"
`
	if err := os.WriteFile(p, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Media.Driver != "s3" || cfg.Media.S3 == nil || cfg.Media.S3.Bucket != "my-bucket" {
		t.Errorf("S3 配置加载错误: %+v", cfg.Media)
	}
}
```

- [ ] **步骤 2：运行确认失败**

运行：`go test -count=1 ./internal/config/ -run 'TestValidateS3Config|TestLoadS3Config' -v`
预期：FAIL（`S3Config` 未定义）

- [ ] **步骤 3：config 扩展**（`internal/config/config.go`）

```go
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
```

`applyEnv` 追加：
```go
	if cfg.Media.S3 == nil {
		cfg.Media.S3 = &S3Config{}
	}
	set("MEDIA_S3_ENDPOINT", &cfg.Media.S3.Endpoint)
	set("MEDIA_S3_REGION", &cfg.Media.S3.Region)
	set("MEDIA_S3_BUCKET", &cfg.Media.S3.Bucket)
	set("MEDIA_S3_ACCESS_KEY", &cfg.Media.S3.AccessKey)
	set("MEDIA_S3_SECRET_KEY", &cfg.Media.S3.SecretKey)
	set("MEDIA_S3_PUBLIC_BASE_URL", &cfg.Media.S3.PublicBaseURL)
```
> 注意：`applyEnv` 无条件建 S3Config 会导致 yaml 无 s3 块时也非 nil——**更好**：仅当 `DULIZHAN_MEDIA_S3_*` 任一存在时才建。实现时用 `if v, ok := os.LookupEnv(...)` 判断。

`Validate()` 追加：
```go
	if c.Media.Driver == "s3" {
		if c.Media.S3 == nil || c.Media.S3.Bucket == "" {
			return fmt.Errorf("media.s3.bucket 必填（driver=s3 时）")
		}
	}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test -count=1 ./internal/config/ -v`
预期：全部 PASS

- [ ] **步骤 5：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 2：S3Store 实现 + mock 测试

**文件：**
- 创建：`internal/media/s3.go`
- 创建：`internal/media/s3_test.go`
- 修改：`go.mod`/`go.sum`（go get aws-sdk-go-v2）

- [ ] **步骤 1：添加依赖**

```bash
$env:GOPROXY="https://goproxy.cn,direct"
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.71.0 github.com/aws/aws-sdk-go-v2/config@v1.28.0 github.com/aws/aws-sdk-go-v2/credentials@v1.17.0
# 确认 go.mod 中 gin/sqlite 版本未变
Select-String -Path go.mod -Pattern "gin-gonic|modernc"
```

- [ ] **步骤 2：编写失败测试**（`internal/media/s3_test.go`）

```go
package media

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3svc "github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeS3Client 实现 S3Store 依赖的 PutObject/DeleteObject 方法（最小接口）。
type fakeS3Client struct {
	putKey    string
	putBody   string
	putType   string
	deletedKey string
}

func (f *fakeS3Client) PutObject(ctx context.Context, params *s3svc.PutObjectInput, optFns ...func(*s3svc.Options)) (*s3svc.PutObjectOutput, error) {
	f.putKey = aws.ToString(params.Key)
	f.putType = aws.ToString(params.ContentType)
	b, _ := io.ReadAll(params.Body)
	f.putBody = string(b)
	return &s3svc.PutObjectOutput{}, nil
}

func (f *fakeS3Client) DeleteObject(ctx context.Context, params *s3svc.DeleteObjectInput, optFns ...func(*s3svc.Options)) (*s3svc.DeleteObjectOutput, error) {
	f.deletedKey = aws.ToString(params.Key)
	return &s3svc.DeleteObjectOutput{}, nil
}

func TestS3StoreSaveDelete(t *testing.T) {
	fake := &fakeS3Client{}
	s := &S3Store{client: fake, bucket: "bkt", publicBase: ""}
	ctx := context.Background()

	url, err := s.Save(ctx, "202608/abc.png", bytes.NewReader([]byte("hello")), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if fake.putKey != "202608/abc.png" || fake.putBody != "hello" || fake.putType != "image/png" {
		t.Errorf("PutObject 参数 = key:%q body:%q type:%q", fake.putKey, fake.putBody, fake.putType)
	}
	if url == "" {
		t.Error("URL 为空")
	}
	if err := s.Delete(ctx, "202608/abc.png"); err != nil {
		t.Fatal(err)
	}
	if fake.deletedKey != "202608/abc.png" {
		t.Errorf("DeleteObject key = %q", fake.deletedKey)
	}
}

func TestS3StoreURL(t *testing.T) {
	// public_base_url 优先
	s1 := &S3Store{client: &fakeS3Client{}, bucket: "b", publicBase: "https://cdn.example.com"}
	if got := s1.URL("202608/x.png"); got != "https://cdn.example.com/202608/x.png" {
		t.Errorf("URL(publicBase) = %q", got)
	}
	// 无 public_base_url 用 bucket 拼接
	s2 := &S3Store{client: &fakeS3Client{}, bucket: "b", publicBase: ""}
	u2 := s2.URL("202608/x.png")
	if u2 != "https://b.s3.amazonaws.com/202608/x.png" && u2 != "https://b.s3.us-east-1.amazonaws.com/202608/x.png" {
		t.Errorf("URL(bucket) = %q", u2)
	}
}
```
> `S3Store.client` 字段类型：用接口（`PutObject`/`DeleteObject` 方法集），而非 `*s3svc.Client`——便于 fake。**实现时**定义 `s3Client interface { PutObject(...); DeleteObject(...) }`。

- [ ] **步骤 3：运行确认失败**

运行：`go test -count=1 ./internal/media/ -run 'TestS3StoreSaveDelete|TestS3StoreURL' -v`
预期：FAIL（S3Store 未定义）

- [ ] **步骤 4：实现 S3Store**（`internal/media/s3.go`）

```go
package media

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3svc "github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3Client S3 客户端最小接口（便于测试注入 fake）。
type s3Client interface {
	PutObject(ctx context.Context, params *s3svc.PutObjectInput, optFns ...func(*s3svc.Options)) (*s3svc.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3svc.DeleteObjectInput, optFns ...func(*s3svc.Options)) (*s3svc.DeleteObjectOutput, error)
}

type S3Store struct {
	client     s3Client
	bucket     string
	publicBase string
}

// NewS3Store 构造 S3Store。endpoint 非空时用自定义端点（MinIO/OSS）。
func NewS3Store(cfg S3Config) (*S3Store, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	}
	acfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置: %w", err)
	}
	var client *s3svc.Client
	if cfg.Endpoint != "" {
		client = s3svc.NewFromConfig(acfg, func(o *s3svc.Options) {
			o.BaseEndpoint = aws.String(strings.TrimRight(cfg.Endpoint, "/"))
		})
	} else {
		client = s3svc.NewFromConfig(acfg)
	}
	return &S3Store{client: client, bucket: cfg.Bucket, publicBase: cfg.PublicBaseURL}, nil
}

func (s *S3Store) Save(ctx context.Context, key string, r io.Reader, contentType string) (string, error) {
	k, ok := sanitizeKey(key)
	if !ok {
		return "", fmt.Errorf("非法存储键 %q", key)
	}
	_, err := s.client.PutObject(ctx, &s3svc.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(k),
		Body:        r,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	return s.URL(k), nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	k, ok := sanitizeKey(key)
	if !ok {
		return fmt.Errorf("非法存储键 %q", key)
	}
	_, err := s.client.DeleteObject(ctx, &s3svc.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(k),
	})
	return err
}

func (s *S3Store) URL(key string) string {
	if s.publicBase != "" {
		return strings.TrimRight(s.publicBase, "/") + "/" + key
	}
	return "https://" + s.bucket + ".s3.amazonaws.com/" + key
}
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test -count=1 ./internal/media/ -v`
预期：全部 PASS

- [ ] **步骤 6：全量回归**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿

---

### 任务 3：装配切换（server + main）+ 冒烟

**文件：**
- 修改：`internal/server/server.go`（装配按 driver）
- 修改：`cmd/dulizhan/main.go`（装配按 driver）

- [ ] **步骤 1：server 装配**（`internal/server/server.go`）

`New` 中媒体构造改为：
```go
	var med media.MediaStore
	if cfg.Media.Driver == "s3" && cfg.Media.S3 != nil {
		med, err = media.NewS3Store(*cfg.Media.S3)
		if err != nil {
			return nil, fmt.Errorf("初始化 S3 媒体: %w", err)
		}
	} else {
		med = media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
	}
```
> `New` 当前签名无 media 参数（medStore 是参数）。**确认**：`server.New(cfg, st, svc, reg, loader, authSvc, medStore)`——medStore 是入参！所以**装配在 main.go**，server 只是接收。修正：本任务只改 **main.go**（构造时按 driver 选择），server.go 不动。见步骤 2。

- [ ] **步骤 2：main.go 装配**（`cmd/dulizhan/main.go`）

```go
	var med media.MediaStore
	if cfg.Media.Driver == "s3" && cfg.Media.S3 != nil {
		med, err = media.NewS3Store(*cfg.Media.S3)
		if err != nil {
			log.Fatalf("初始化 S3 媒体失败: %v", err)
		}
	} else {
		med = media.NewLocalStore(filepath.Join(cfg.Server.DataDir, "media"), "/media")
	}
	// server.New(cfg, st, svc, reg, loader, authSvc, med) 使用 med
```

- [ ] **步骤 3：验证**

运行：`go test -count=1 ./...`；`go build ./...`；`go vet ./...`
预期：全绿（config 默认 local，无行为变化）

- [ ] **步骤 4：冒烟（可选）**

- 配置 `media.driver: s3` + 假 endpoint → 启动不报错（真实上传需真实 S3/MinIO，无环境则跳过）
- 用 mock 单测已覆盖 Save/Delete/URL 行为

---

## 自检记录

**1. 规格覆盖度：**
- 规格 §2 config → 任务 1
- 规格 §3 S3Store → 任务 2
- 规格 §4 依赖 → 任务 2 步骤 1
- 规格 §5 测试 → 任务 2（mock）
- 规格 §6 验收 → 任务 3

**2. 占位符扫描：** 无 TBD/TODO。每步含代码。`s3Client` 接口定义明确（PutObject/DeleteObject），无占位。

**3. 类型一致性：**
- `S3Config` 任务 1 定义，任务 2 NewS3Store/任务 3 装配使用
- `s3Client` 接口任务 2 定义，S3Store.client 字段与 fake 一致
- `S3Store{client, bucket, publicBase}` 字段与 URL/Save/Delete 一致
- config 装配修正：server.New 的 medStore 是入参，装配在 main.go（步骤 2 说明）
