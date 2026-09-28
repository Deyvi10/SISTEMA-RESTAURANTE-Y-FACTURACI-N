package imagenes

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrNoExiste = errors.New("imagenes: no existe")

// Store guarda objetos. En la nube es S3; en local, VersityGW (ADR-0011).
type Store interface {
	Put(ctx context.Context, key string, data []byte, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Exists(ctx context.Context, key string) (bool, error)
}

type S3 struct {
	c      *minio.Client
	bucket string
}

func NewS3(endpoint, access, secret, bucket string, ssl bool) (*S3, error) {
	c, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(access, secret, ""), Secure: ssl, BucketLookup: minio.BucketLookupPath})
	if err != nil {
		return nil, err
	}
	return &S3{c: c, bucket: bucket}, nil
}

func (s *S3) Put(ctx context.Context, key string, data []byte, ct string) error {
	_, err := s.c.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: ct, CacheControl: "public, max-age=31536000, immutable"})
	return err
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.c.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNoExiste
		}
		return nil, err
	}
	return obj, nil
}

func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.c.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil && minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return false, nil
	}
	return err == nil, err
}

// Memory es un Store en memoria para pruebas.
type Memory struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func (m *Memory) Put(_ context.Context, key string, data []byte, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objs == nil {
		m.objs = map[string][]byte{}
	}
	m.objs[key] = bytes.Clone(data)
	return nil
}

func (m *Memory) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objs[key]
	if !ok {
		return nil, ErrNoExiste
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *Memory) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objs[key]
	return ok, nil
}
