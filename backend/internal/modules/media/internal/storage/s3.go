package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 是 aws-sdk-go-v2 的 Store 實作，R2 與 s3compat 共用。
type S3 struct {
	bucket  string
	client  *s3.Client
	presign *s3.PresignClient
}

var _ Store = (*S3)(nil)

// New 依設定建構 S3；設定不完整時回傳錯誤，不退回 fake。不做網路呼叫。
func New(cfg Config) (*S3, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	creds := aws.Credentials{AccessKeyID: cfg.AccessKeyID, SecretAccessKey: cfg.SecretAccessKey, Source: "media-storage-config"}
	client := s3.New(s3.Options{
		Region:       cfg.Region,
		BaseEndpoint: aws.String(cfg.Endpoint),
		UsePathStyle: cfg.PathStyle,
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return creds, nil
		}),
		// 只在操作要求時才算 checksum：預簽網址除 Content-MD5 外不多綁 x-amz-checksum-*，R2 已實測；S3 相容替身由整合測試驗證。
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	return &S3{bucket: cfg.Bucket, client: client, presign: s3.NewPresignClient(client)}, nil
}

// md5DigestSize 是 Content-MD5 解碼後的位元組數。
const md5DigestSize = 16

func validatePresign(in PresignPutInput) error {
	switch {
	case in.Key == "":
		return errors.New("storage: empty key")
	case in.ContentLength <= 0:
		return errors.New("storage: content length must be positive")
	case in.ContentType == "":
		return errors.New("storage: empty content type")
	case in.ContentMD5 == "":
		return errors.New("storage: empty content md5")
	case in.Expires <= 0:
		return errors.New("storage: expiry must be positive")
	case in.Expires > MaxPresignExpiry:
		return errors.New("storage: expiry exceeds the SigV4 limit of 7 days")
	}
	if raw, err := base64.StdEncoding.DecodeString(in.ContentMD5); err != nil || len(raw) != md5DigestSize {
		return errors.New("storage: content md5 must be standard base64 of 16 bytes")
	}
	return nil
}

func presignHeaders(in PresignPutInput) map[string]string {
	return map[string]string{
		"Content-Type":  in.ContentType,
		"Content-MD5":   in.ContentMD5,
		"If-None-Match": "*",
	}
}

// PresignPut 實作 Store。
func (s *S3) PresignPut(ctx context.Context, in PresignPutInput) (PresignedPut, error) {
	if err := validatePresign(in); err != nil {
		return PresignedPut{}, err
	}
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(in.Key),
		ContentLength: aws.Int64(in.ContentLength),
		ContentType:   aws.String(in.ContentType),
		ContentMD5:    aws.String(in.ContentMD5),
		IfNoneMatch:   aws.String("*"),
	}, s3.WithPresignExpires(in.Expires))
	if err != nil {
		return PresignedPut{}, fmt.Errorf("storage: presign put: %w", err)
	}
	return PresignedPut{URL: req.URL, Headers: presignHeaders(in)}, nil
}

// Get 實作 Store。
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: get: %w", err)
	}
	return out.Body, nil
}

// Head 實作 Store。
func (s *S3) Head(ctx context.Context, key string) (ObjectInfo, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, fmt.Errorf("storage: head: %w", err)
	}
	return ObjectInfo{Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType)}, nil
}

// Put 實作 Store。
func (s *S3) Put(ctx context.Context, in PutInput) error {
	if in.Key == "" {
		return errors.New("storage: empty key")
	}
	body, ok := in.Body.(io.ReadSeeker)
	if ok {
		remaining, err := remainingLen(body)
		if err != nil {
			return fmt.Errorf("storage: measure put body: %w", err)
		}
		if remaining != in.Size {
			return ErrSizeMismatch
		}
	} else {
		data, err := io.ReadAll(io.LimitReader(in.Body, in.Size+1))
		if err != nil {
			return fmt.Errorf("storage: read put body: %w", err)
		}
		if int64(len(data)) != in.Size {
			return ErrSizeMismatch
		}
		body = bytes.NewReader(data)
	}
	var contentType *string
	if in.ContentType != "" {
		contentType = aws.String(in.ContentType)
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(in.Key),
		Body:          body,
		ContentLength: aws.Int64(in.Size),
		ContentType:   contentType,
	})
	if err != nil {
		return fmt.Errorf("storage: put: %w", err)
	}
	return nil
}

// Delete 實作 Store；S3 的 DeleteObject 對不存在的物件本來就回成功。
func (s *S3) Delete(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}); err != nil {
		return fmt.Errorf("storage: delete: %w", err)
	}
	return nil
}

func remainingLen(r io.ReadSeeker) (int64, error) {
	cur, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	if _, err := r.Seek(cur, io.SeekStart); err != nil {
		return 0, err
	}
	return end - cur, nil
}

// isNotFound 只認 key 不存在：NoSuchKey、NotFound，以及 HEAD 沒有 body 的 404；NoSuchBucket 等其他錯誤碼一律是錯誤。
func isNotFound(err error) bool {
	var nsk *s3types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *s3types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var ae interface{ ErrorCode() string }
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		case "", "UnknownError":
		default:
			return false
		}
	}
	var re interface{ HTTPStatusCode() int }
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}
