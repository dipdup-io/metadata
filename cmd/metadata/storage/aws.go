package storage

import (
	"bytes"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

// AWS -
type AWS struct {
	Session *session.Session
	Bucket  *string
}

// NewAWS -
func NewAWS(id, secret, region, bucket string) *AWS {
	if id == "" || secret == "" || region == "" || bucket == "" {
		return nil
	}

	return &AWS{
		Session: session.Must(session.NewSession(&aws.Config{
			Region:      aws.String(region),
			Credentials: credentials.NewStaticCredentials(id, secret, ""),
			MaxRetries:  aws.Int(3),
		})),
		Bucket: aws.String(bucket),
	}
}

// Upload -
func (storage *AWS) Upload(body io.Reader, filename string) error {
	uploader := s3manager.NewUploader(storage.Session)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      storage.Bucket,
		Key:         aws.String(filename),
		Body:        body,
		ContentType: aws.String("image/png"),
	})
	return err
}

// Download -
func (storage *AWS) Download(filename string) (io.Reader, error) {
	downloader := s3manager.NewDownloader(storage.Session)

	buf := aws.NewWriteAtBuffer([]byte{})

	if _, err := downloader.Download(buf, &s3.GetObjectInput{
		Bucket: storage.Bucket,
		Key:    aws.String(filename),
	}); err != nil {
		return nil, errors.Errorf("failed to download file, %v", err)
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// Exists -
func (storage *AWS) Exists(filename string) bool {
	_, err := s3.New(storage.Session).HeadObject(&s3.HeadObjectInput{
		Bucket: storage.Bucket,
		Key:    aws.String(filename),
	})
	return err == nil
}
