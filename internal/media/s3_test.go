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
	putKey     string
	putBody    string
	putType    string
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
	// 1. public_base_url 优先
	s1 := &S3Store{client: &fakeS3Client{}, bucket: "b", publicBase: "https://cdn.example.com", endpoint: "https://minio:9000", region: "cn-hangzhou"}
	if got := s1.URL("202608/x.png"); got != "https://cdn.example.com/202608/x.png" {
		t.Errorf("URL(publicBase) = %q", got)
	}
	// 2. endpoint 回退：endpoint/bucket/key
	s2 := &S3Store{client: &fakeS3Client{}, bucket: "bkt", endpoint: "https://minio:9000"}
	if got := s2.URL("202608/x.png"); got != "https://minio:9000/bkt/202608/x.png" {
		t.Errorf("URL(endpoint) = %q", got)
	}
	// endpoint 带尾斜杠
	s3 := &S3Store{client: &fakeS3Client{}, bucket: "bkt", endpoint: "https://minio:9000/"}
	if got := s3.URL("202608/x.png"); got != "https://minio:9000/bkt/202608/x.png" {
		t.Errorf("URL(endpoint trailing slash) = %q", got)
	}
	// 3. region 回退
	s4 := &S3Store{client: &fakeS3Client{}, bucket: "bkt", region: "cn-hangzhou"}
	if got := s4.URL("202608/x.png"); got != "https://bkt.s3.cn-hangzhou.amazonaws.com/202608/x.png" {
		t.Errorf("URL(region) = %q", got)
	}
	// region 缺省 us-east-1
	s5 := &S3Store{client: &fakeS3Client{}, bucket: "bkt"}
	if got := s5.URL("202608/x.png"); got != "https://bkt.s3.us-east-1.amazonaws.com/202608/x.png" {
		t.Errorf("URL(default region) = %q", got)
	}
}

func TestS3StoreKey(t *testing.T) {
	cases := []struct {
		name string
		s    *S3Store
		url  string
		want string
	}{
		{"AWS 虚拟主机形式", &S3Store{bucket: "bkt"}, "https://bkt.s3.us-east-1.amazonaws.com/202608/x.png", "202608/x.png"},
		{"AWS 缺省 region", &S3Store{bucket: "b"}, "https://b.s3.amazonaws.com/202608/x.png", "202608/x.png"},
		{"endpoint+bucket 剥 bucket", &S3Store{bucket: "bkt", endpoint: "https://minio:9000"}, "https://minio:9000/bkt/202608/x.png", "202608/x.png"},
		{"public_base 形式（路径为 key）", &S3Store{bucket: "bkt", publicBase: "https://cdn/x"}, "https://cdn/x/202608/y.png", "x/202608/y.png"},
		{"key 以 bucket 开头不误剥", &S3Store{bucket: "b"}, "https://b.s3.amazonaws.com/b.png", "b.png"},
		{"非法 URL 原样剥前导斜杠", &S3Store{bucket: "bkt"}, "/202608/x.png", "202608/x.png"},
	}
	for _, c := range cases {
		c.s.client = &fakeS3Client{}
		if got := c.s.Key(c.url); got != c.want {
			t.Errorf("%s: Key(%q) = %q, want %q", c.name, c.url, got, c.want)
		}
	}
	// 与 URL 互逆：Key(URL(key)) 还原（public_base 不带路径前缀的形态）
	key := "202608/y.png"
	for _, s := range []*S3Store{
		{bucket: "bkt"},
		{bucket: "bkt", region: "cn-hangzhou"},
		{bucket: "bkt", endpoint: "https://minio:9000"},
		{bucket: "bkt", publicBase: "https://cdn.example.com"},
	} {
		s.client = &fakeS3Client{}
		if got := s.Key(s.URL(key)); got != key {
			t.Errorf("URL/Key 不可逆 (%+v): %q", s, got)
		}
	}
}
