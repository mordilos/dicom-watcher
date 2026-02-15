package watcher

import (
	"ikh/dicom-watcher/internal/api"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ikh/dicom-watcher/internal/config"
	"ikh/dicom-watcher/internal/models"
)

type Watcher struct {
	Config       *config.Config
	Studies      map[string]*models.Study
	LastEvent    time.Time
	Timeout      time.Duration
	PollInterval time.Duration
	BatchSize    int
	FileMetadata map[string]time.Time
	Mutex        sync.Mutex
}

func NewWatcher(config *config.Config) (*Watcher, error) {
	return &Watcher{
		Config:       config,
		Studies:      make(map[string]*models.Study),
		LastEvent:    time.Now(),
		Timeout:      time.Duration(config.Timeout) * time.Second,
		PollInterval: time.Duration(config.PollInterval) * time.Second,
		BatchSize:    config.BatchSize,
		FileMetadata: make(map[string]time.Time),
	}, nil
}

func (w *Watcher) Start() {
	go func() {
		for {
			time.Sleep(w.PollInterval)
			w.CheckDirectory()
			if time.Since(w.LastEvent) > w.Timeout {
				w.CheckReadyStudies()
			}
		}
	}()
}

func (w *Watcher) CheckDirectory() {
	// Walk the directory and process the files in batches
	log.Printf("Checking directory...")
	err := filepath.Walk(w.Config.DirectoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			if lastModified, ok := w.FileMetadata[path]; !ok || lastModified.Before(info.ModTime()) {
				w.ProcessFile(path, info.ModTime())
				w.FileMetadata[path] = info.ModTime()
				w.LastEvent = time.Now()
			}
		}
		return nil
	})
	if err != nil {
		log.Println("error:", err)
	}
}

func (w *Watcher) ProcessFile(filePath string, lastModified time.Time) {
	// Check if the file has a .dcm extension
	if filepath.Ext(filePath) != ".dcm" {
		return
	}

	// Extract study and series IDs from the file path
	studyID, seriesID, dicomID := extractIDs(filePath)

	// Lock the mutex before accessing the Studies map
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	// Update the study structure
	if _, ok := w.Studies[studyID]; !ok {
		w.Studies[studyID] = &models.Study{
			ID:     studyID,
			Series: make(map[string]*models.Series),
			Ready:  false,
		}
	}

	if _, ok := w.Studies[studyID].Series[seriesID]; !ok {
		w.Studies[studyID].Series[seriesID] = &models.Series{
			ID:         seriesID,
			DicomFiles: make(map[string]*models.DicomFile),
		}
	}

	w.Studies[studyID].Series[seriesID].DicomFiles[dicomID] = &models.DicomFile{
		ID:           dicomID,
		FilePath:     filePath,
		LastModified: lastModified,
	}
}

func (w *Watcher) CheckReadyStudies() {

	// Lock the mutex before accessing the Studies map
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	for _, study := range w.Studies {
		if !study.Ready {
			study.Ready = true
			// Notify the API that the study is ready
			go api.NotifyStudyReady(w.Config.ApiUrl, study.ID)
		}
	}
}

func extractIDs(filePath string) (studyID, seriesID, dicomID string) {
	// Extract study, series, and DICOM file IDs from the file path
	// Example: /directory/study123/series456/dicom789.dcm
	parts := strings.Split(filePath, "/")
	studyID = parts[len(parts)-3]
	seriesID = parts[len(parts)-2]
	dicomID = filepath.Base(parts[len(parts)-1])
	return studyID, seriesID, dicomID
}
