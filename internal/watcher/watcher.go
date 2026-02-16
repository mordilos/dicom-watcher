package watcher

import (
	"ikh/dicom-watcher/internal/api"
	"log"
	"os"
	"path/filepath"
	"runtime"
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
	StudyTimers  map[string]*time.Timer
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
		StudyTimers:  make(map[string]*time.Timer),
	}, nil
}

func (w *Watcher) Start() {
	go func() {
		for {
			time.Sleep(w.PollInterval)
			w.CheckDirectory()
		}
	}()
}

func (w *Watcher) CheckDirectory() {
	log.Printf("Checking directory...")

	// Create a channel to receive file paths
	fileChan := make(chan string, w.BatchSize)

	// Create a channel to receive errors
	errChan := make(chan error, w.BatchSize)

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start a goroutine to walk the directory and send file paths to the channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := filepath.Walk(w.Config.DirectoryPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				errChan <- err
				return err
			}
			if !info.IsDir() {
				fileChan <- path
			}
			return nil
		})
		if err != nil {
			errChan <- err
		}
		close(fileChan)
		close(errChan)
	}()

	// Start a pool of goroutines to process files from the channel
	numWorkers := runtime.NumCPU() * 2 // Adjust the number of workers based on the number of CPU cores
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range fileChan {
				info, err := os.Stat(filePath)
				if err != nil {
					errChan <- err
					continue
				}
				if lastModified, ok := w.FileMetadata[filePath]; !ok || lastModified.Before(info.ModTime()) {
					w.ProcessFile(filePath, info.ModTime())
					w.FileMetadata[filePath] = info.ModTime()
					w.LastEvent = time.Now()
				}
			}
		}()
	}

	// Start a goroutine to handle errors from the error channel
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range errChan {
			log.Println("error:", err)
		}
	}()

	// Wait for all goroutines to finish
	wg.Wait()
}

func (w *Watcher) ProcessFile(filePath string, lastModified time.Time) {
	log.Println("Processing file:", filePath)

	// Check if the file has a .dcm extension
	if filepath.Ext(filePath) != ".dcm" && filepath.Ext(filePath) != ".dcm.gz" {
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
		// Start a timer for the study
		w.StudyTimers[studyID] = time.AfterFunc(w.Timeout, func() {
			w.CheckStudyReady(studyID)
		})
	} else {
		// Reset the timer for the study
		if timer, ok := w.StudyTimers[studyID]; ok {
			timer.Reset(w.Timeout)
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

func (w *Watcher) CheckStudyReady(studyID string) {
	// Lock the mutex before accessing the Studies map
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if study, ok := w.Studies[studyID]; ok && !study.Ready {
		study.Ready = true

		log.Println("study ready:", studyID)
		// Notify the API that the study is ready
		go api.NotifyStudyReady(w.Config.ApiUrl, study.ID)
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
