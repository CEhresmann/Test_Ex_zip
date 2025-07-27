package internal

import (
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

type File struct {
	URL    string `json:"url"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Path   string `json:"-"`
}

type Task struct {
	ID          string     `json:"id"`
	Status      TaskStatus `json:"status"`
	Files       []File     `json:"files"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	ArchivePath string     `json:"archive_path,omitempty"`
	Mu          sync.Mutex `json:"-"`
}
