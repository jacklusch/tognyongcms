package media

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3svc "github.com/aws/aws-sdk-go-v2/service/s3"

	"dulizhan/internal/config"
)

// s3Client S3 客户端最小接口（便于测试注入 fake）。
type s3Client interface {
	PutObject(ctx context.Context, params *s3svc.PutObjectInput, optFns ...func(*s3svc.Options)) (*s3svc.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3svc.DeleteObjectInput, optFns ...func(*s3svc.Options)) (*s3svc.DeleteObjectOutput, error)
}

type S3Store struct {
	client     s3Client
	bucket     string
	region     string
	endpoint   string
	publicBase string
}

// NewS3Store 构造 S3Store。endpoint 非空时用自定义端点（MinIO/OSS）。
func NewS3Store(cfg config.S3Config) (*S3Store, error) {
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
	return &S3Store{client: client, bucket: cfg.Bucket, region: cfg.Region, endpoint: cfg.Endpoint, publicBase: cfg.PublicBaseURL}, nil
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
	if s.endpoint != "" {
		return strings.TrimRight(s.endpoint, "/") + "/" + s.bucket + "/" + key
	}
	region := s.region
	if region == "" {
		region = "us-east-1"
	}
	return "https://" + s.bucket + ".s3." + region + ".amazonaws.com/" + key
}

// Key 从 URL 还原存储键。URL 形如 https://<bucket>.s3.amazonaws.com/<key>
// 或 <public_base>/<key> 或 <endpoint>/<bucket>/<key>；提取路径部分并去掉
// endpoint+bucket 形式中的 bucket 前缀。
func (s *S3Store) Key(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return strings.TrimPrefix(urlStr, "/")
	}
	// endpoint+bucket 形式：路径以 /<bucket>/ 开头，剥掉 bucket 前缀
	if strings.HasPrefix(u.Path, "/"+s.bucket+"/") {
		return strings.TrimPrefix(u.Path, "/"+s.bucket+"/")
	}
	return strings.TrimPrefix(u.Path, "/")
}
