package blob

import (
	"bytes"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/supperdoggy/SmartHomeServer/music-services/spotdl-wapper/pkg/config"
	"go.uber.org/zap"
)

type BlobStorage interface {
	UploadFile(data []byte, filename string) (string, error)
}

type blobStorage struct {
	l      *zap.Logger
	client *s3.S3

	// buckets
	musicFilesBucket string
}

func NewBlobStorage(l *zap.Logger, cfg config.BlobStorageConfig) (BlobStorage, error) {
	s3Config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(cfg.AccessKeyID, cfg.SecretAccess, ""), // Specifies your credentials.
		// Endpoint:         aws.String("https://nyc3.digitaloceanspaces.com"),                       // Find your endpoint in the control panel, under Settings. Prepend "https://".
		Endpoint:         aws.String(cfg.Endpoint),
		S3ForcePathStyle: aws.Bool(false),        // // Configures to use subdomain/virtual calling format. Depending on your version, alternatively use o.UsePathStyle = false
		Region:           aws.String(cfg.Region), // Must be "us-east-1" when creating new Spaces. Otherwise, use the region in your endpoint, such as "nyc3".
	}

	s3Options := session.Options{
		Config: *s3Config,
	}

	// Step 3: The new session validates your request and directs it to your Space's specified endpoint using the AWS SDK.
	newSession, err := session.NewSessionWithOptions(s3Options)
	if err != nil {
		return nil, err
	}

	s3Client := s3.New(newSession)

	return &blobStorage{
		l: l,
		// s:      newSession,
		client:           s3Client,
		musicFilesBucket: cfg.Bucket,
	}, nil
}

// UploadFile uploads a file to the blob storage
func (b *blobStorage) UploadFile(data []byte, filename string) (string, error) {
	// reader
	reader := bytes.NewReader(data)

	_, err := b.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(b.musicFilesBucket),
		Key:    aws.String(filename),
		Body:   aws.ReadSeekCloser(reader),
	})
	if err != nil {
		b.l.Error("failed to upload file", zap.Error(err))
		return "", err
	}

	return b.musicFilesBucket + "/" + filename, nil
}
