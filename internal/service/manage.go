package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"test_ex_zip/internal"
	"time"
)

var (
	ErrTaskNotFound    = errors.New("task not found")
	ErrMaxFiles        = errors.New("max files reached")
	ErrInvalidFileType = errors.New("invalid file type")
	ErrServerBusy      = errors.New("server busy")
)

type TaskManager struct {
	tasks      map[string]*internal.Task
	tasksMu    sync.RWMutex
	activeJobs chan struct{}
	cfg        *internal.Config
}

func NewTaskManager(cfg *internal.Config) *TaskManager {
	return &TaskManager{
		tasks:      make(map[string]*internal.Task),
		activeJobs: make(chan struct{}, cfg.MaxTasks),
		cfg:        cfg,
	}
}

func (m *TaskManager) CreateTask() (*internal.Task, error) {
	select {
	case m.activeJobs <- struct{}{}:
	default:
		return nil, ErrServerBusy
	}

	task := &internal.Task{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Status:    internal.StatusPending,
		CreatedAt: time.Now(),
	}

	m.tasksMu.Lock()
	m.tasks[task.ID] = task
	m.tasksMu.Unlock()

	return task, nil
}

func (m *TaskManager) AddFile(taskID, url string) error {
	m.tasksMu.RLock()
	task, exists := m.tasks[taskID]
	m.tasksMu.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	task.Mu.Lock()
	defer task.Mu.Unlock()

	if len(task.Files) >= m.cfg.MaxFiles {
		return ErrMaxFiles
	}

	ext := filepath.Ext(url)
	valid := false
	for _, allowed := range m.cfg.AllowedExts {
		if ext == allowed {
			valid = true
			break
		}
	}

	if !valid {
		return ErrInvalidFileType
	}

	task.Files = append(task.Files, internal.File{
		URL:    url,
		Status: "queued",
	})

	if len(task.Files) == m.cfg.MaxFiles {
		go m.processTask(task)
	}

	return nil
}

func (m *TaskManager) processTask(task *internal.Task) {
	task.Mu.Lock()
	task.Status = internal.StatusProcessing
	task.Mu.Unlock()

	defer func() {
		<-m.activeJobs
	}()
	prt, err := m.cfg.MakeTimePr()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), prt)
	defer cancel()

	urls := make([]string, len(task.Files))
	for i, file := range task.Files {
		urls[i] = file.URL
	}

	dwn, err := m.cfg.MakeTimeDwn()
	downloadedPaths, errors := DownloadFiles(ctx, urls, m.cfg.TempDir, dwn)

	task.Mu.Lock()
	for i := range task.Files {
		if i < len(errors) && errors[i] != nil {
			task.Files[i].Status = "failed"
			task.Files[i].Error = errors[i].Error()
		} else if i < len(downloadedPaths) && downloadedPaths[i] != "" {
			task.Files[i].Status = "downloaded"
			task.Files[i].Path = downloadedPaths[i]
		}
	}
	task.Mu.Unlock()

	archivePath := filepath.Join(m.cfg.ArchiveDir, task.ID+".zip")
	filePaths := make([]string, len(task.Files))
	fileNames := make([]string, len(task.Files))
	for i, file := range task.Files {
		filePaths[i] = file.Path
		fileNames[i] = filepath.Base(file.URL)
	}

	if err := CreateArchive(filePaths, fileNames, archivePath); err == nil {
		task.Mu.Lock()
		task.Status = internal.StatusCompleted
		task.ArchivePath = archivePath
		task.CompletedAt = time.Now()
		task.Mu.Unlock()
	}
}

func (m *TaskManager) GetTask(taskID string) (*internal.Task, error) {
	m.tasksMu.RLock()
	defer m.tasksMu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}
