package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/ses"
)

func AwsSdk() error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	var config Config
	configBuf := &aws.WriteAtBuffer{}
	downloader := s3manager.NewDownloader(sess)
	_, err := downloader.Download(configBuf, &s3.GetObjectInput{
		Bucket: aws.String(configBucket),
		Key:    aws.String(configKey),
	})
	if err != nil {
		return fmt.Errorf("Error downloading configuration: %v", err)
	}
	err = yaml.Unmarshal(configBuf.Bytes(), &config)
	if err != nil {
		return fmt.Errorf("Error unmarshaling configuration: %v", err)
	}
  
	nvdFeed := nvd.NewNVDFeed(config.Feeds["NVD"].URL)
	err = nvdFeed.Download()
	if err != nil {
		return fmt.Errorf("Error downloading NVD feed: %v", err)
	}
	vulnerabilities, err := nvdFeed.Vulnerabilities(config.Filter)
	if err != nil {
		return fmt.Errorf("Error processing NVD vulnerabilities: %v", err)
	}
	sort.Sort(sort.Reverse(feeds.ByScore(vulnerabilities)))

	// Report only new vulnerabilities.
	var vReport Vulnerabilities
	dynamoSvc := dynamodb.New(sess)
	for _, v := range vulnerabilities {
		result, err := dynamoSvc.GetItem(&dynamodb.GetItemInput{
			TableName: aws.String(config.Resources.CacheTable),
			Key: map[string]*dynamodb.AttributeValue{
				"cve": {
					S: aws.String(v.CVE),
				},
			},
		})
		if err != nil {
			log.Println("Error retrieving vulnerability from cache:", err.Error())
		}
		if len(result.Item) < 1 {
			// Only if the key is not found.
			v.Meta["severity"] = severity(v.Score)
			vReport.Vulnerabilities = append(vReport.Vulnerabilities, v)
			continue
		}
	}
	vReport.Count = len(vReport.Vulnerabilities)

	if vReport.Count < 1 {
		return nil
	}

	sesSvc := ses.New(sess)
	vReportJSON, err := json.Marshal(vReport)
	if err != nil {
		return fmt.Errorf("Error marshaling JSON report: %v", err)
	}

	_, err = sesSvc.SendTemplatedEmail(&ses.SendTemplatedEmailInput{
		Source:     aws.String(config.Resources.SourceEmail),
		ReturnPath: aws.String(config.Resources.SourceEmail),
		Destination: &ses.Destination{
			ToAddresses: aws.StringSlice([]string{config.Resources.DestinationEmail}),
		},
		Template:     aws.String(config.Resources.Template),
		TemplateData: aws.String(string(vReportJSON)),
	})
	if err != nil {
		return fmt.Errorf("Error sending report email: %v", err)
	}

	// Mark all reported vulnerabilities as processed.
	for _, v := range vReport.Vulnerabilities {
		item, err := dynamodbattribute.MarshalMap(v)
		if err != nil {
			log.Println("Error marshaling vulnerability for cache:", err.Error())
			continue
		}
		_, err = dynamoSvc.PutItem(&dynamodb.PutItemInput{
			Item:      item,
			TableName: aws.String(config.Resources.CacheTable),
		})
		if err != nil {
			log.Println("Error adding vulnerability to cache:", err.Error())
			continue
		}
	}

	return nil
}
