package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/surajkadam7/iot-backend/internal/config"
)

// S3 implements Store (and Presigner) against AWS S3 or an S3-compatible endpoint.
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func NewS3(ctx context.Context, cfg config.Config) (*S3, error) {
	if strings.TrimSpace(cfg.S3Bucket) == "" {
		return nil, fmt.Errorf("S3_BUCKET is required for the s3 archive backend")
	}
	loadOpts := []func(*awsconfig.LoadOptions) error{}
	if cfg.AWSRegion != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(cfg.AWSRegion))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	var s3Opts []func(*s3.Options)
	if cfg.S3Endpoint != "" {
		endpoint := cfg.S3Endpoint
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}
	client := s3.NewFromConfig(awsCfg, s3Opts...)
	return &S3{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.S3Bucket,
	}, nil
}

func (s *S3) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	key, err := normalizeKey(key)
	if err != nil {
		return err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) List(ctx context.Context, prefix string) ([]string, error) {
	prefix = strings.TrimPrefix(strings.TrimSpace(prefix), "/")
	var out []string
	var token *string
	for {
		resp, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range resp.Contents {
			if obj.Key == nil || *obj.Key == "" {
				continue
			}
			out = append(out, *obj.Key)
		}
		if !aws.ToBool(resp.IsTruncated) {
			break
		}
		token = resp.NextContinuationToken
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

func (s *S3) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	key, err := normalizeKey(key)
	if err != nil {
		return "", err
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

var _ Store = (*S3)(nil)
var _ Presigner = (*S3)(nil)
