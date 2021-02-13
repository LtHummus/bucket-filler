package videostorage

import (
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws/session"
)

type videoStorage struct {
	bucket     string
	s3         *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
}

var _ Storage = &videoStorage{}

func New(bucket string) *videoStorage {
	manager := s3.New(session.Must(session.NewSession()))
	return &videoStorage{
		bucket:     bucket,
		s3:         manager,
		uploader:   s3manager.NewUploaderWithClient(manager),
		downloader: s3manager.NewDownloaderWithClient(manager),
	}
}

func (vs *videoStorage) Delete(key string) error {
	_, err := vs.s3.DeleteObject(&s3.DeleteObjectInput{
		Key:    &key,
		Bucket: &vs.bucket,
	})

	return err
}

func (vs *videoStorage) Download(key, dest string) error {
	localLogger := log.WithFields(log.Fields{
		"key":  key,
		"dest": dest,
	})
	localLogger.Info("starting download")
	destDir := filepath.Dir(dest)
	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		log.WithError(err).Warn("could not create directories")
		return err
	}
	destFile, err := os.Create(dest)
	if err != nil {
		log.WithError(err).Warn("could not create file")
		return err
	}

	bytes, err := vs.downloader.Download(destFile, &s3.GetObjectInput{
		Bucket: &vs.bucket,
		Key:    &key,
	})
	if err != nil {
		localLogger.WithError(err).Warn("could not download file")
		return err
	}

	localLogger.WithField("bytes", bytes).Info("download complete")
	return nil
}

func (vs *videoStorage) Upload(key string, sourceFile string) error {
	file, err := os.Open(sourceFile)
	if err != nil {
		log.WithFields(log.Fields{
			"key":        key,
			"sourceFile": sourceFile,
		}).Warn("could not open source file")
		return err
	}

	defer file.Close()
	_, err = vs.uploader.Upload(&s3manager.UploadInput{
		Bucket: &vs.bucket,
		Key:    &key,
		Body:   file,
	})

	if err != nil {
		log.WithFields(log.Fields{
			"key":        key,
			"sourceFile": sourceFile,
		}).Warn("could not upload file")
		return err
	}

	return nil
}
