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
	Config        *config.Config
	TenantStudies map[string]map[string]*models.Study
	LastEvent     time.Time
	Timeout       time.Duration
	PollInterval  time.Duration
	BatchSize     int
	FileMetadata  map[string]time.Time
	Mutex         sync.Mutex
	StudyTimers   map[string]*time.Timer
}

func NewWatcher(config *config.Config) (*Watcher, error) {
	return &Watcher{
		Config:        config,
		TenantStudies: make(map[string]map[string]*models.Study),
		LastEvent:     time.Now(),
		Timeout:       time.Duration(config.Timeout) * time.Second,
		PollInterval:  time.Duration(config.PollInterval) * time.Second,
		BatchSize:     config.BatchSize,
		FileMetadata:  make(map[string]time.Time),
		StudyTimers:   make(map[string]*time.Timer),
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

	go func() {
		for err := range errChan {
			log.Println("error:", err)
		}
	}()

	numWorkers := runtime.NumCPU() * 2
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

				// LOCK: Protect FileMetadata and LastEvent
				w.Mutex.Lock()
				lastModified, ok := w.FileMetadata[filePath]
				needsProcessing := !ok || lastModified.Before(info.ModTime())

				if needsProcessing {
					// We process while holding the lock because ProcessFile
					// also manipulates shared maps
					w.processFileLocked(filePath, info.ModTime())
					w.FileMetadata[filePath] = info.ModTime()
					w.LastEvent = time.Now()
				}
				w.Mutex.Unlock()
			}
		}()
	}

	err := filepath.Walk(w.Config.DirectoryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
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

	// 4. Cleanup
	close(fileChan) // Signal workers to stop
	wg.Wait()       // Wait for workers to finish
	close(errChan)  // Now safe to close error channel
}

func (w *Watcher) processFileLocked(filePath string, lastModified time.Time) {
	// Check if the file has a .dcm extension
	ext := filepath.Ext(filePath)
	if ext != ".dcm" && !strings.HasSuffix(filePath, ".dcm.gz") {
		return
	}

	tenantID, studyID, seriesID, dicomID := extractIDs(filePath)
	model := "medclip"

	if _, ok := w.TenantStudies[tenantID]; !ok {
		w.TenantStudies[tenantID] = make(map[string]*models.Study)
	}

	if _, ok := w.TenantStudies[tenantID][studyID]; !ok {
		w.TenantStudies[tenantID][studyID] = &models.Study{
			ID:     studyID,
			Series: make(map[string]*models.Series),
			Ready:  false,
		}

		// Capture vars for closure
		tID, sID := tenantID, studyID
		w.StudyTimers[studyID] = time.AfterFunc(w.Timeout, func() {
			w.CheckStudyReady(tID, sID, model)
		})
	} else {
		if timer, ok := w.StudyTimers[studyID]; ok {
			timer.Reset(w.Timeout)
		}
	}

	if _, ok := w.TenantStudies[tenantID][studyID].Series[seriesID]; !ok {
		w.TenantStudies[tenantID][studyID].Series[seriesID] = &models.Series{
			ID:         seriesID,
			DicomFiles: make(map[string]*models.DicomFile),
		}
	}

	w.TenantStudies[tenantID][studyID].Series[seriesID].DicomFiles[dicomID] = &models.DicomFile{
		ID:           dicomID,
		FilePath:     filePath,
		LastModified: lastModified,
	}
}

func (w *Watcher) CheckStudyReady(tenantID, studyID, model string) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if study, ok := w.TenantStudies[tenantID][studyID]; ok && !study.Ready {
		study.Ready = true
		log.Printf("tenant: %s - study: %s stabilized...", tenantID, studyID)

		// Clean up timer map to prevent memory leak
		delete(w.StudyTimers, studyID)

		// Trigger API call in goroutine so we don't hold the lock during network I/O
		go api.NotifyStudyReady(w.Config.ApiUrl, tenantID, studyID, model)
	}
}

func extractIDs(filePath string) (tenantID, studyID, seriesID, dicomID string) {
	// Use filepath.ToSlash to handle Windows/Linux path differences
	parts := strings.Split(filepath.ToSlash(filePath), "/")

	if len(parts) < 4 {
		return "unknown", "unknown", "unknown", filepath.Base(filePath)
	}

	tenantID = parts[len(parts)-4]
	studyID = parts[len(parts)-3]
	seriesID = parts[len(parts)-2]
	dicomID = filepath.Base(parts[len(parts)-1])
	return tenantID, studyID, seriesID, dicomID
}
