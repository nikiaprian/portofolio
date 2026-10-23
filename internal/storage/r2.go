package storage

import (
	"context"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type R2Store struct {
	client    *s3.Client
	bucket    string
	publicURL string
	prefix    string
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicBaseURL   string
	Prefix          string
}

func NewR2Store(ctx context.Context, cfg R2Config) (*R2Store, error) {
	if strings.TrimSpace(cfg.AccountID) == "" ||
		strings.TrimSpace(cfg.AccessKeyID) == "" ||
		strings.TrimSpace(cfg.SecretAccessKey) == "" ||
		strings.TrimSpace(cfg.Bucket) == "" ||
		strings.TrimSpace(cfg.PublicBaseURL) == "" {
		return nil, errors.New("missing required R2 config")
	}

	endpoint := "https://" + strings.TrimSpace(cfg.AccountID) + ".r2.cloudflarestorage.com"

	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.SecretAccessKey),
			"",
		)),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...any) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{URL: endpoint, SigningRegion: region}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			},
		)),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		// R2 is S3-compatible but expects path-style addressing.
		o.UsePathStyle = true
	})

	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix == "" {
		prefix = "uploads"
	}

	return &R2Store{
		client:    client,
		bucket:    strings.TrimSpace(cfg.Bucket),
		publicURL: strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/"),
		prefix:    prefix,
	}, nil
}

func (s *R2Store) Put(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	key = strings.TrimLeft(key, "/")
	if s.prefix != "" {
		key = strings.Trim(s.prefix, "/") + "/" + key
	}

	in := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}

	if _, err := s.client.PutObject(ctx, in); err != nil {
		return "", err
	}

	return s.publicURL + "/" + key, nil
}

// DeleteURL removes the object if objectURL is under this store's public base URL.
func (s *R2Store) DeleteURL(ctx context.Context, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	if i := strings.Index(rawURL, "?"); i >= 0 {
		rawURL = rawURL[:i]
	}
	if i := strings.Index(rawURL, "#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	obj, err := url.Parse(rawURL)
	if err != nil || obj.Path == "" {
		return nil
	}
	base, err := url.Parse(s.publicURL)
	if err != nil {
		return nil
	}
	objPath := strings.TrimSuffix(obj.Path, "/")
	basePath := strings.TrimSuffix(base.Path, "/")
	if base.Host != "" && obj.Host != "" && !strings.EqualFold(base.Host, obj.Host) {
		return nil
	}
	if basePath != "" && !strings.HasPrefix(objPath, basePath) {
		return nil
	}
	rel := strings.TrimPrefix(objPath, basePath)
	rel = strings.TrimLeft(rel, "/")
	if rel == "" {
		return nil
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(rel),
	})
	if err != nil && !isS3NoSuchKey(err) {
		return err
	}
	return nil
}

func isS3NoSuchKey(err error) bool {
	if err == nil {
		return false
	}
	// R2/S3 may return NotFound or a NoSuchKey code in the message.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nosuchkey") || strings.Contains(msg, "not found") || strings.Contains(msg, "statuscode: 404")
}
