package model

import "time"

type FileMetadata struct {
	ID               string    `json:"id"`
	Filename         string    `json:"filename"`
	Size             int64     `json:"size"`
	ContentType      string    `json:"content_type"`
	S3Key            string    `json:"s3_key"`
	S3Bucket         string    `json:"s3_bucket"`
	UploadedByUserID uint      `json:"uploaded_by_user_id"`
	ChatID           uint      `json:"chat_id"`
	CreatedAt        time.Time `json:"created_at"`
}
