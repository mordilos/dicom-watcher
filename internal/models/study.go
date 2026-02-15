package models

import "time"

type Study struct {
	ID     string
	Series map[string]*Series
	Ready  bool
}

type Series struct {
	ID         string
	DicomFiles map[string]*DicomFile
}

type DicomFile struct {
	ID           string
	FilePath     string
	LastModified time.Time
}
