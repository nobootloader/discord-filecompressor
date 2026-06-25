package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel"
)

// S3 Presigner encapsulates the Amazon Simple Storage Service (Amazon S3) presign actions
// used in the examples.
// It contains PresignClient, a client that is used to presign requests to Amazon S3.
// Presigned requests contain temporary credentials and can be made from any HTTP client.
type S3 struct {
	PresignClient *s3.PresignClient
	S3Client      *s3.Client
}

func New(presingclient *s3.PresignClient, s3client *s3.Client) S3 {
	return S3{
		PresignClient: presingclient,
		S3Client:      s3client,
	}
}

const tracerID = "compression-repository-s3"

// GetObject makes a presigned request that can be used to get an object from a bucket.
// The presigned request is valid for the specified number of seconds.
func (presigner S3) GetObject(
	ctx context.Context, bucketName string, objectKey string, lifetimeSecs int64) (*v4.PresignedHTTPRequest, error) {

	_, span := otel.Tracer(tracerID).Start(ctx, "Repository/GetObject")
	defer span.End()
	request, err := presigner.PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(lifetimeSecs * int64(time.Second))
	})
	if err != nil {
		log.Printf("Couldn't get a presigned request to get %v:%v. Here's why: %v\n",
			bucketName, objectKey, err)
	}
	return request, err
}

// PutObject makes a presigned request that can be used to put an object in a bucket.
// The presigned request is valid for the specified number of seconds.
func (p S3) PutObject(ctx context.Context, bucketname string, objectKey string, lifetimeSecs int64) (*v4.PresignedHTTPRequest, error) {
	_, span := otel.Tracer(tracerID).Start(ctx, "Repository/PutObject")
	defer span.End()

	request, err := p.PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketname),
		Key:         aws.String(objectKey),
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		log.Printf("Couldn't get a presigned request to put %v:%v. Here's why: %v\n",
			bucketname, objectKey, err)
	}
	return request, err
}

// DeleteObject makes a presigned request that can be used to delete an object from a bucket.
func (presigner S3) DeleteObject(ctx context.Context, bucketName string, objectKey string) (*v4.PresignedHTTPRequest, error) {
	_, span := otel.Tracer(tracerID).Start(ctx, "Repository/DeleteObject")
	defer span.End()
	request, err := presigner.PresignClient.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Couldn't get a presigned request to delete object %v. Here's why: %v\n", objectKey, err)
	}
	return request, err
}

func (p S3) DownloadObject(ctx context.Context, bucketName string, objectKey string, filename string) (string, error) {
	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(bucketName),
		Key:                        aws.String(objectKey),
		ResponseContentType:        aws.String("video/mp4"),
		ResponseContentDisposition: aws.String("attachment; filename=\"" + filename + "\""),
	})

	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			log.Printf("Can't get object %s from bucket %s. No such key exists.\n", objectKey, bucketName)
			err = noKey
		} else {
			log.Printf("Couldn't get object %v:%v. Here's why: %v\n", bucketName, objectKey, err)
		}
		return "", err
	}

	defer result.Body.Close()

	tempFilePath := "/tmp/" + filename
	file, err := os.Create(tempFilePath)
	if err != nil {
		log.Printf("Couldn't create file %v. Here's why: %v\n", objectKey, err)
		return "", err
	}
	defer file.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("Couldn't read object body from %v. Here's why: %v\n", objectKey, err)
	}
	_, err = file.Write(body)
	if err != nil {
		log.Printf("failed to write file")
		return "", err
	}
	return tempFilePath, nil
}

func (p S3) DownloadPartialObject(ctx context.Context, bucketName string, objectKey string, filename string, byteLimit int64) (string, error) {
	rangeHeader := fmt.Sprintf("bytes=0-%d", byteLimit-1)
	result, err := p.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Range:  aws.String(rangeHeader),
	})
	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			log.Printf("Can't get object %s from bucket %s. No such key exists.\n", objectKey, bucketName)
			err = noKey
		} else {
			log.Printf("Couldn't get object %v:%v. Here's why: %v\n", bucketName, objectKey, err)
		}
		return "", err
	}

	defer result.Body.Close()

	filePath := fmt.Sprintf("%s_partial", filename)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Couldn't create file %v. Here's why: %v\n", objectKey, err)
		return "", err
	}
	defer file.Close()
	partialData, err := io.ReadAll(result.Body)
	if err != nil {
		log.Printf("Couldn't read object body from %v. Here's why: %v\n", objectKey, err)
		return "", err
	}
	_, err = file.Write(partialData)
	if err != nil {
		log.Printf("failed to write file")
		return "", err
	}
	return filePath, nil
}

func (p S3) UploadObject(ctx context.Context, bucketName string, objectKey string, filename string) error {
	_, span := otel.Tracer(tracerID).Start(ctx, "Repository/UploadObject")
	defer span.End()
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	result, err := p.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:             aws.String(bucketName),
		Key:                aws.String(objectKey),
		Body:               f,
		ContentType:        aws.String("video/mp4"),
		ContentDisposition: aws.String("attachment; filename=\"" + filename + "\""),
	})

	if err != nil {
		var noKey *types.NoSuchKey
		if errors.As(err, &noKey) {
			log.Printf("Can't get object %s from bucket %s. No such key exists.\n", objectKey, bucketName)
			err = noKey
		} else {
			log.Printf("Couldn't get object %v:%v. Here's why: %v\n", bucketName, objectKey, err)
		}
		return err
	}
	log.Println(result)
	return nil
}
