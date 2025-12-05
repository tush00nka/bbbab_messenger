package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"time"
	"tush00nka/bbbab_messenger/internal/config"
	"tush00nka/bbbab_messenger/internal/model"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type S3Service struct {
	config   *config.Config
	uploader *manager.Uploader
	s3Client *s3.Client
}

func NewS3Service(cfg *config.Config) (*S3Service, error) {
	// Используем BaseEndpoint для кастомного endpoint
	s3Opts := []func(*s3.Options){}

	if cfg.S3Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true // Обязательно для MinIO
		})
	}

	// Создаем кастомный провайдер credentials
	credsProvider := credentials.NewStaticCredentialsProvider(
		cfg.S3AccessKeyID,
		cfg.S3SecretAccessKey,
		"",
	)

	// Создаем конфигурацию
	awsCfg := aws.Config{
		Region:      cfg.S3Region,
		Credentials: credsProvider,
	}
	// awsCfg, err := config.LoadDefaultConfig(context.Background(),
	// 	config.WithRegion(cfg.S3Region),
	// 	config.WithCredentialsProvider(credsProvider),
	// )
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to load AWS config: %w", err)
	// }

	// Создаем S3 клиент
	s3Client := s3.NewFromConfig(awsCfg, s3Opts...)

	service := &S3Service{
		config:   cfg,
		uploader: manager.NewUploader(s3Client),
		s3Client: s3Client,
	}

	log.Printf("🔧 S3 service initialized with endpoint: %s", cfg.S3Endpoint)
	return service, nil
}

func (s *S3Service) UploadFile(ctx context.Context, file io.Reader, filename, contentType, userID, chatID string) (*model.FileMetadata, error) {
	fileID := uuid.New().String()
	s3Key := path.Join("chats", chatID, fileID, filename)

	log.Printf("📤 Uploading file: %s to %s/%s", filename, s.config.S3BucketName, s3Key)

	result, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.config.S3BucketName),
		Key:         aws.String(s3Key),
		Body:        file,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	log.Printf("✅ File uploaded successfully: %s", result.Location)

	return &model.FileMetadata{
		ID:          fileID,
		Filename:    filename,
		ContentType: contentType,
		S3Key:       s3Key,
		S3Bucket:    s.config.S3BucketName,
		UploadedBy:  userID,
		ChatID:      chatID,
		CreatedAt:   time.Now(),
	}, nil
}

func (s *S3Service) GeneratePresignedURL(ctx context.Context, fileMetadata *model.FileMetadata, expires time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.s3Client)

	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(fileMetadata.S3Bucket),
		Key:    aws.String(fileMetadata.S3Key),
	}, s3.WithPresignExpires(expires))

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

func (s *S3Service) HealthCheck(ctx context.Context) error {
	// Простая проверка - пытаемся листовать bucket'ы
	_, err := s.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("storage health check failed: %w", err)
	}
	return nil
}
