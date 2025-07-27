package main

import (
	"flag"
	"log"
	"net/http"
	"test_ex_zip/cli"
	"test_ex_zip/internal"
	"test_ex_zip/internal/handler"
	"test_ex_zip/internal/service"
)

func main() {
	guiMode := flag.Bool("gui", false, "Enable graphical user interface")
	flag.Parse()

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

	go func() {
		log.Printf("Server started on %s", cfg.Addr)
		log.Fatal(http.ListenAndServe(cfg.Addr, mux))
	}()

	if *guiMode {
		if err := cli.StartGUI(); err != nil {
			log.Fatalf("GUI error: %v", err)
		}
	} else {
		select {}
	}
}
