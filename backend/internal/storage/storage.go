package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zllyxr/live_claw/backend/internal/config"
)

const (
	PublicBucket   = "claw-public"
	PrivateBucket  = "claw-private"
	ReleasesBucket = "claw-releases"
)

type Service struct {
	client       *minio.Client
	publicSigner *minio.Client
}

type ObjectInfo struct {
	Size        int64
	ContentType string
	SHA256      string
}

func New(cfg config.Config) (*Service, error) {
	if strings.TrimSpace(cfg.MinIOAccessKey) == "" ||
		strings.TrimSpace(cfg.MinIOSecretKey) == "" {
		return nil, errors.New("minio access key and secret key are required")
	}
	region := strings.TrimSpace(cfg.MinIORegion)
	if region == "" {
		region = "us-east-1"
	}
	client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOUseTLS,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	publicSigner, err := minio.New(cfg.MinIOPublicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
		Secure: cfg.MinIOPublicUseTLS,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create public minio signer: %w", err)
	}
	return &Service{client: client, publicSigner: publicSigner}, nil
}

func (s *Service) EnsureBuckets(ctx context.Context) error {
	for _, bucket := range []string{PublicBucket, PrivateBucket, ReleasesBucket} {
		exists, err := s.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("check minio bucket %s: %w", bucket, err)
		}
		if !exists {
			if err = s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				response := minio.ToErrorResponse(err)
				if response.Code != "BucketAlreadyOwnedByYou" && response.Code != "BucketAlreadyExists" {
					return fmt.Errorf("create minio bucket %s: %w", bucket, err)
				}
			}
		}
	}
	publicReadPolicy := `{
		"Version":"2012-10-17",
		"Statement":[{
			"Effect":"Allow",
			"Principal":{"AWS":["*"]},
			"Action":["s3:GetObject"],
			"Resource":["arn:aws:s3:::` + PublicBucket + `/*"]
		}]
	}`
	if err := s.client.SetBucketPolicy(ctx, PublicBucket, publicReadPolicy); err != nil {
		return fmt.Errorf("set public media bucket policy: %w", err)
	}
	releaseCORS := cors.NewConfig([]cors.Rule{{
		AllowedOrigin: []string{"*"},
		AllowedMethod: []string{http.MethodGet, http.MethodHead, http.MethodPut},
		AllowedHeader: []string{"*"},
		ExposeHeader:  []string{"ETag"},
		MaxAgeSeconds: 3600,
	}})
	if err := s.client.SetBucketCors(ctx, ReleasesBucket, releaseCORS); err != nil {
		response := minio.ToErrorResponse(err)
		if response.StatusCode != http.StatusNotImplemented && response.Code != "NotImplemented" {
			return fmt.Errorf("set app release bucket cors: %w", err)
		}
	}
	return nil
}

func (s *Service) PresignedGet(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	if !validBucket(bucket) || strings.TrimSpace(objectKey) == "" {
		return "", errors.New("invalid storage object")
	}
	if expiry < time.Minute || expiry > 24*time.Hour {
		return "", errors.New("invalid signed url expiry")
	}
	signed, err := s.publicSigner.PresignedGetObject(ctx, bucket, objectKey, expiry, url.Values{})
	if err != nil {
		return "", fmt.Errorf("sign minio download: %w", err)
	}
	return signed.String(), nil
}

func (s *Service) PresignedPut(
	ctx context.Context,
	bucket, objectKey, contentType string,
	expiry time.Duration,
) (string, error) {
	if !validBucket(bucket) || strings.TrimSpace(objectKey) == "" || strings.TrimSpace(contentType) == "" {
		return "", errors.New("invalid storage upload")
	}
	if expiry < time.Minute || expiry > time.Hour {
		return "", errors.New("invalid signed url expiry")
	}
	signed, err := s.publicSigner.PresignHeader(ctx, "PUT", bucket, objectKey, expiry, url.Values{}, http.Header{
		"Content-Type": []string{contentType},
	})
	if err != nil {
		return "", fmt.Errorf("sign minio upload: %w", err)
	}
	return signed.String(), nil
}

func (s *Service) PutObject(
	ctx context.Context,
	bucket, objectKey string,
	body io.Reader,
	size int64,
	contentType string,
) error {
	if !validBucket(bucket) || strings.TrimSpace(objectKey) == "" || size < 0 || strings.TrimSpace(contentType) == "" {
		return errors.New("invalid storage object")
	}
	_, err := s.client.PutObject(ctx, bucket, objectKey, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("put minio object: %w", err)
	}
	return nil
}

func (s *Service) RemoveObject(ctx context.Context, bucket, objectKey string) error {
	if !validBucket(bucket) || strings.TrimSpace(objectKey) == "" {
		return errors.New("invalid storage object")
	}
	if err := s.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove minio object: %w", err)
	}
	return nil
}

func (s *Service) InspectObject(ctx context.Context, bucket, objectKey string) (ObjectInfo, error) {
	if !validBucket(bucket) || strings.TrimSpace(objectKey) == "" {
		return ObjectInfo{}, errors.New("invalid storage object")
	}
	object, err := s.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("open minio object: %w", err)
	}
	defer object.Close()
	stat, err := object.Stat()
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat minio object: %w", err)
	}
	digest := sha256.New()
	if _, err = io.Copy(digest, object); err != nil {
		return ObjectInfo{}, fmt.Errorf("hash minio object: %w", err)
	}
	return ObjectInfo{
		Size: stat.Size, ContentType: stat.ContentType,
		SHA256: hex.EncodeToString(digest.Sum(nil)),
	}, nil
}

func validBucket(bucket string) bool {
	return bucket == PublicBucket || bucket == PrivateBucket || bucket == ReleasesBucket
}
