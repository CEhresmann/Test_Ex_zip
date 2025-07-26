package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"test_ex_zip/internal"
	"test_ex_zip/internal/service"
	"time"
)

type TaskHandler struct {
	manager *service.TaskManager
}

func NewTaskHandler(manager *service.TaskManager) *TaskHandler {
	return &TaskHandler{manager: manager}
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	task, err := h.manager.CreateTask()
	if err != nil {
		if errors.Is(err, service.ErrServerBusy) {
			respondError(w, http.StatusTooManyRequests, "server busy")
		} else {
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	respondJSON(w, http.StatusCreated, map[string]string{"id": task.ID})
}

func (h *TaskHandler) AddFile(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var request struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if err := h.manager.AddFile(taskID, request.URL); err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			respondError(w, http.StatusNotFound, "task not found")
		case errors.Is(err, service.ErrMaxFiles):
			respondError(w, http.StatusBadRequest, "max files reached")
		case errors.Is(err, service.ErrInvalidFileType):
			respondError(w, http.StatusBadRequest, "invalid file type")
		default:
			respondError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	task, err := h.manager.GetTask(taskID)
	if err != nil {
		respondError(w, http.StatusNotFound, "task not found")
		return
	}

	task.Mu.Lock()
	response := struct {
		Status    internal.TaskStatus `json:"status"`
		Files     []internal.File     `json:"files"`
		Archive   string              `json:"archive,omitempty"`
		CreatedAt time.Time           `json:"created_at"`
	}{
		Status:    task.Status,
		Files:     make([]internal.File, len(task.Files)),
		CreatedAt: task.CreatedAt,
	}
	copy(response.Files, task.Files)

	if task.Status == internal.StatusCompleted {
		response.Archive = "/download/" + task.ID
	}
	task.Mu.Unlock()

	respondJSON(w, http.StatusOK, response)
}

func (h *TaskHandler) DownloadArchive(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	task, err := h.manager.GetTask(taskID)
	if err != nil || task.Status != internal.StatusCompleted {
		respondError(w, http.StatusNotFound, "archive not available")
		return
	}
	http.ServeFile(w, r, task.ArchivePath)
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
