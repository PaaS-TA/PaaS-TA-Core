package reporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/onsi/ginkgo/config"
	ginkgotypes "github.com/onsi/ginkgo/types"
)

type S3Reporter struct {
	logger     lager.Logger
	bucketName string
	uploader   *s3manager.Uploader
}

func NewS3Reporter(
	logger lager.Logger,
	bucketName string,
	uploader *s3manager.Uploader,
) S3Reporter {
	return S3Reporter{
		logger:     logger.Session("s3-reporter"),
		bucketName: bucketName,
		uploader:   uploader,
	}
}

type Data struct {
	Timestamp   int64
	Measurement ginkgotypes.SpecMeasurement
}

var startTime string

func (r *S3Reporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *ginkgotypes.SuiteSummary) {
	startTime = time.Now().Format(time.RFC3339)
}

func (r *S3Reporter) BeforeSuiteDidRun(setupSummary *ginkgotypes.SetupSummary) {
}

func (r *S3Reporter) AfterSuiteDidRun(setupSummary *ginkgotypes.SetupSummary) {
}

func (r *S3Reporter) SpecWillRun(specSummary *ginkgotypes.SpecSummary) {
}

func (r *S3Reporter) SpecDidComplete(specSummary *ginkgotypes.SpecSummary) {
	if specSummary.Passed() && specSummary.IsMeasurement {
		for _, measurement := range specSummary.Measurements {
			if measurement.Info == nil {
				panic(fmt.Sprintf("%#v", specSummary))
			}

			info, ok := measurement.Info.(ReporterInfo)
			if !ok {
				r.logger.Error("failed-type-assertion-on-measurement-info", errors.New("type-assertion-failed"))
				continue
			}

			if info.MetricName == "" {
				r.logger.Error("failed-blank-metric-name", errors.New("blank-metric-name"))
				continue
			}

			now := time.Now()
			data := Data{
				Timestamp:   now.Unix(),
				Measurement: *measurement,
			}

			dataJSON, err := json.Marshal(data)
			if err != nil {
				r.logger.Error("failed-marshaling-data", err)
				continue
			}

			measurementData := string(dataJSON)
			key := fmt.Sprintf("%s/%s-%s", startTime, info.MetricName, now.Format(time.RFC3339))

			_, err = r.uploader.Upload(&s3manager.UploadInput{
				Bucket: aws.String(r.bucketName),
				Key:    aws.String(key),
				Body:   bytes.NewReader([]byte(measurementData)),
			})

			if err != nil {
				r.logger.Error("failed-uploading-metric-to-s3", err)
				continue
			}

			r.logger.Debug("successfully-uploaded-metric-to-s3", lager.Data{
				"bucket-name": r.bucketName,
				"key":         key,
				"content":     measurementData,
			})
		}
	}
}

func (r *S3Reporter) SpecSuiteDidEnd(summary *ginkgotypes.SuiteSummary) {
}
