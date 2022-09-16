package main

import (
	"bytes"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/urfave/cli/v2"
)

const (
	contentType   = "text/plain"
	defaultRegion = "us-east-1"
	expireSeconds = 600
)

var (
	bucket string
	region string
	key    string
)

func main() {
	doMain(os.Args)
}

func doMain(args []string) {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "region",
				Destination: &region,
				Value:       defaultRegion,
			},
		},
		Commands: []*cli.Command{
			{
				Name:   "run",
				Usage:  "Run the test",
				Action: cmdRun,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "bucket",
						Destination: &bucket,
					},
					&cli.StringFlag{
						Name:        "key",
						Destination: &key,
					},
				},
			},
		},
	}

	err := app.Run(args)
	if err != nil {
		log.Fatal(err)
	}
}

func cmdRun(cliCtx *cli.Context) error {
	ctx := cliCtx.Context
	logLevel := aws.LogDebug
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:   aws.String(region),
			LogLevel: &logLevel,
		},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return err
	}
	client := s3.New(sess)

	// Initiate Multipart upload
	expiresAt := time.Now().Add(time.Duration(expireSeconds) * time.Second)
	createParams := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Expires:     &expiresAt,
	}
	requestOptions := func(req *request.Request) {
		// This will pre-sign the request for the given duration.
		exp := time.Until(expiresAt)
		req.ExpireTime = exp
	}
	rspCreate, err := client.CreateMultipartUploadWithContext(
		ctx, createParams, requestOptions,
	)
	if err != nil {
		return err
	}

	var partNum int64 = 1
	uploadParams := &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   rspCreate.UploadId,
		PartNumber: &partNum,
	}

	r := bytes.NewReader([]byte("dummy content"))
	uploadParams.Body = r

	completedParts := make([]*s3.CompletedPart, 0, 100)
	rspUpload, err := client.UploadPartWithContext(
		ctx,
		uploadParams,
		requestOptions,
	)
	if err != nil {
		return err
	}
	part := partNum
	completedParts = append(
		completedParts,
		&s3.CompletedPart{
			ETag:       rspUpload.ETag,
			PartNumber: &part,
		},
	)

	completeUploadParams := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: rspCreate.UploadId,
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	_, err = client.CompleteMultipartUploadWithContext(
		ctx,
		completeUploadParams,
		requestOptions,
	)
	if err != nil {
		return err
	}

	time.Sleep(600 * time.Second)

	return nil
}
