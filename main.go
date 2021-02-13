package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"

	"github.com/google/uuid"

	log "github.com/sirupsen/logrus"

	"github.com/lthummus/bucket-filler/remuxlog"
	"github.com/lthummus/bucket-filler/streamer"
	"github.com/lthummus/bucket-filler/videostorage"
)

type remuxHandler struct {
	inputBucketName  string
	outputBucketName string
	downloadDir      string

	deleteWhenDone bool

	inputStorage  videostorage.Storage
	outputStorage videostorage.Storage
	remuxLog      remuxlog.RemuxLog
}

func (rh *remuxHandler) log(ctx context.Context, start time.Time, inputKey string, outputKey string, error string) {
	duration := int(time.Since(start).Milliseconds())
	lc, _ := lambdacontext.FromContext(ctx)

	err := rh.remuxLog.LogRemux(inputKey, duration, lc.AwsRequestID, outputKey, error)
	if err != nil {
		log.WithFields(log.Fields{
			"inputKey":  inputKey,
			"outputKey": outputKey,
			"error":     error,
		}).WithError(err).Warn("could not record")
	}
}

func (rh *remuxHandler) handler(ctx context.Context, event events.S3Event) (string, error) {
	success := true
	start := time.Now()
	log.Info("starting event handling (v2)")
	sourceBucket := event.Records[0].S3.Bucket.Name
	sourceKey, _ := url.QueryUnescape(event.Records[0].S3.Object.Key)

	eventLogger := log.WithFields(log.Fields{
		"sourceBucket":      sourceBucket,
		"sourceKey":         sourceKey,
		"destinationBucket": rh.outputBucketName,
	})

	eventLogger.Info("decoded input")

	if sourceBucket != rh.inputBucketName {
		eventLogger.Warn("source bucket not correct. stopping.")
		return "skipped", nil
	}

	if !strings.HasSuffix(strings.ToLower(sourceKey), ".mp4") {
		eventLogger.Warn("file not an MP4. Skipping")
		rh.log(ctx, start, sourceKey, "", "not an MP4")
		return "not an mp4, skipped", nil
	}

	destKey := strings.TrimSuffix(sourceKey, filepath.Ext(sourceKey)) + ".flv"
	testFile := path.Join(rh.downloadDir, fmt.Sprintf("%s%s", uuid.NewString(), filepath.Ext(sourceKey)))
	err := rh.inputStorage.Download(sourceKey, testFile)
	if err != nil {
		eventLogger.WithError(err).Error("could not download")
		rh.log(ctx, start, sourceKey, destKey, "could not download")
		return "download failed", err
	}

	eventLogger = eventLogger.WithField("tempFile", testFile)

	eventLogger.Info("file downloaded")

	destFile := filepath.Join(rh.downloadDir, fmt.Sprintf("%s.flv", uuid.NewString()))

	eventLogger = eventLogger.WithFields(log.Fields{
		"destKey":  destKey,
		"destFile": destFile,
	})

	eventLogger.Info("starting remux")
	_ = streamer.StartFfmpegRemux("/opt/bin/ffmpeg", testFile, destFile)
	eventLogger.Info("finished remux")

	eventLogger.Info("starting remuxed upload")
	err = rh.outputStorage.Upload(destKey, destFile)
	if err != nil {
		rh.log(ctx, start, sourceKey, destKey, "could not upload file")
		success = false
		eventLogger.WithError(err).Warn("could not upload file")
	}
	eventLogger.Info("completed remux upload")

	err = os.Remove(destFile)
	if err != nil {
		eventLogger.WithError(err).WithField("destFile", destFile).Warn("could not delete dest file")
	}
	eventLogger.Info("removed destFile")
	err = os.Remove(testFile)
	if err != nil {
		eventLogger.WithError(err).WithField("testFile", testFile).Warn("could not delete temp file")
	}

	eventLogger.WithField("testFile", testFile).Info("temp file deleted")
	eventLogger.WithField("destKey", destKey).Info("all done")

	if success && rh.deleteWhenDone {
		var message string

		err := rh.inputStorage.Delete(sourceKey)
		if err != nil {
			eventLogger.WithError(err).Warn("could not delete")
			message = err.Error()
		}

		rh.log(ctx, start, sourceKey, destKey, message)
	}

	return fmt.Sprintf("%s %s", sourceBucket, sourceKey), nil
}

func main() {
	downloadDir := os.Getenv("DOWNLOAD_DIR")
	if downloadDir == "" {
		log.Fatal("DOWNLOAD_DIR not set")
	}

	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		log.WithField("DOWNLOAD_DIR", downloadDir).Fatal("download dir does not exist")
	}

	outputBucketName := os.Getenv("OUTPUT_BUCKET_NAME")
	if outputBucketName == "" {
		log.Fatal("OUTPUT_BUCKET_NAME not set")
	}

	inputBucketName := os.Getenv("INPUT_BUCKET_NAME")
	if inputBucketName == "" {
		log.Fatal("INPUT_BUCKET_NAME not set")
	}

	tableName := os.Getenv("REMUX_LOG_TABLE_NAME")
	if tableName == "" {
		log.Fatal("REMUX_LOG_TABLE_NAME not set")
	}

	arnTopic := os.Getenv("NOTIFICATION_ARN_TOPIC")
	if arnTopic == "" {
		log.Warn("NOTIFICATION_ARN_TOPIC not set. Will not send notifications")
	}

	deleteWhenDone := false
	if os.Getenv("DELETE_WHEN_DONE") == "true" {
		log.Info("will delete objects on completion")

		deleteWhenDone = true
	}

	handler := &remuxHandler{
		downloadDir:      downloadDir,
		inputBucketName:  inputBucketName,
		outputBucketName: outputBucketName,
		deleteWhenDone:   deleteWhenDone,
		outputStorage:    videostorage.New(outputBucketName),
		inputStorage:     videostorage.New(inputBucketName),
		remuxLog:         remuxlog.New(tableName),
	}
	lambda.Start(handler.handler)
}
