package main

import (
	"log"
	"net/http"
	"test_ex_zip/internal"
	"test_ex_zip/internal/handler"
	"test_ex_zip/internal/service"
)

func main() {
	cfg, err := internal.LoadConfig("configs/conf.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	manager := service.NewTaskManager(cfg)
	taskHandler := handler.NewTaskHandler(manager)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /tasks", taskHandler.CreateTask)
	mux.HandleFunc("POST /tasks/{id}", taskHandler.AddFile)
	mux.HandleFunc("GET /status/{id}", taskHandler.GetStatus)
	mux.HandleFunc("GET /download/{id}", taskHandler.DownloadArchive)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	log.Printf("Server started on %s", cfg.Addr)
	log.Fatal(http.ListenAndServe(cfg.Addr, mux))
}

// пользователь отправляет запрос со ссылками, их необходимо скачать, запаковать в zip и вернуть пользователю
// по каждой задаче пользователь должен уметь получить её статус
//
