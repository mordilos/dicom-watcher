package models

import "time"

type Tenant struct {
	ID    string
	Study map[string]*Study
}

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
