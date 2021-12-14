package storage

import (
	"bytes"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/dipdup-net/metadata/cmd/metadata/config"
	"github.com/pkg/errors"
)

// AWS -
type AWS struct {
	Session *session.Session
	Bucket  *string
}

// NewAWS -
func NewAWS(settings config.AWS) *AWS {
	if settings.AccessKey == "" || settings.BucketName == "" || settings.Region == "" || settings.Secret == "" {
		return nil
	}

	return &AWS{
		Session: session.Must(session.NewSession(&aws.Config{
			Endpoint:    &settings.Endpoint,
			Region:      aws.String(settings.Region),
			Credentials: credentials.NewStaticCredentials(settings.AccessKey, settings.Secret, ""),
			MaxRetries:  aws.Int(3),
		})),
		Bucket: aws.String(settings.BucketName),
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
