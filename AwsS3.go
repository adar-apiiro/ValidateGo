package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func main() {
	// Initialize a session using shared config (AWS credentials and region)
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	}))

	// Upload a file to S3
	err := uploadFile(sess, "myBucketName", "path/to/myFile.txt")
	if err != nil {
		log.Fatalf("Unable to upload file: %v", err)
	}
	fmt.Println("File uploaded successfully!")

	// List all objects in the S3 bucket
	err = listObjects(sess, "myBucketName")
	if err != nil {
		log.Fatalf("Unable to list objects: %v", err)
	}
}

// uploadFile uploads a file to an S3 bucket
func uploadFile(sess *session.Session, bucketName, filePath string) error {
	// Open the file to upload
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file %q: %v", filePath, err)
	}
	defer file.Close()

	// Create an S3 uploader
	uploader := s3manager.NewUploader(sess)

	// Upload the file to the specified S3 bucket
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(filePath), // S3 object name
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %v", err)
	}

	return nil
}

// listObjects lists all objects in a specified S3 bucket
func listObjects(sess *session.Session, bucketName string) error {
	// Create an S3 service client
	svc := s3.New(sess)

	// List objects in the specified bucket
	resp, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return fmt.Errorf("unable to list items in bucket %q: %v", bucketName, err)
	}

	// Print the object details
	fmt.Println("Objects in S3 bucket:")
	for _, item := range resp.Contents {
		fmt.Printf("Name: %s, Last modified: %v, Size: %d bytes\n", *item.Key, *item.LastModified, *item.Size)
	}

	return nil
}
