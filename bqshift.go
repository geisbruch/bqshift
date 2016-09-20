package main

import (
	"fmt"
	"github.com/uswitch/bqshift/bigquery"
	"github.com/uswitch/bqshift/redshift"
	"github.com/uswitch/bqshift/storage"
)

type shifter struct {
	redshift *redshift.Client
	config   *Configuration
}

func (s *shifter) Run(table string, config *bigquery.Configuration) error {
	storageClient, err := storage.NewClient(config, s.config.S3)
	if err != nil {
		return err
	}

	bq, err := bigquery.NewClient()
	exists, err := bq.DatasetExists(config.ProjectID, config.DatasetName)
	if err != nil {
		return fmt.Errorf("error checking dataset: %s", err.Error())
	}

	if !exists {
		return fmt.Errorf("dataset doesn't exist: %s", config.DatasetName)
	}

	if err != nil {
		return fmt.Errorf("error creating bigquery client: %s", err.Error())
	}

	fmt.Println("unloading to s3")
	result, err := s.redshift.Unload(table)
	if err != nil {
		return fmt.Errorf("error unloading: %s", err.Error())
	}
	fmt.Println("unloaded to", result.Bucket, ", transferring to cloud storage")
	stored, err := storageClient.TransferToCloudStorage(result)
	if err != nil {
		return fmt.Errorf("error transferring to cloud storage: %s", err.Error())
	}

	fmt.Println("transferred to cloud storage. creating bigquery table.")
	schema, err := s.redshift.ExtractSchema(table)
	if err != nil {
		return fmt.Errorf("error extracting source schema: %s", err.Error())
	}
	ref := bigquery.TableReference(config.ProjectID, config.DatasetName, table)
	err = bq.CreateTable(ref, schema)
	if err != nil {
		return fmt.Errorf("couldn't create bigquery destination table: %s", err.Error())
	}
	err = bq.LoadTable(ref, stored.BucketName, stored.Prefix)
	if err != nil {
		return fmt.Errorf("error loading data into table: %s", err.Error())
	}

	return nil
}

func NewShifter(config *Configuration) (*shifter, error) {
	client, err := redshift.NewClient(config.Redshift, config.S3)
	if err != nil {
		return nil, err
	}

	return &shifter{client, config}, nil
}
