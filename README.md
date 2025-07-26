# Test_Ex_zip
A lightweight HTTP service that downloads files from public URLs, packages them into ZIP archives, and manages concurrent processing tasks with strict resource limits.

### API Endpoints
- `POST /tasks` - Create new task
- `POST /tasks/{id}` - Add file URL to task
- `GET /status/{id}` - Get task status
- `GET /download/{id}` - Download ZIP archive

#### Concurrency Safety:
  - errgroup for parallel file downloads
  - RWMutex for task access protection
  - Buffered channel for task limiting

#### Requirements Compliance:
  - Max 3 files per task
  - Max 3 concurrent tasks
  - File type filtering (.pdf, .jpeg, .jpg)
  - Detailed error reporting

#### Idiomatic Go:
  - Clean layer separation
  - Error handling
  - Efficient routing

#### Easy Deployment:
  - Standard library only
  - Minimal dependencies
  - Easy environment setup

#### Design Principles
 - `Worker Pool Pattern`: Limits concurrent tasks using buffered channels
 - `Error Group`: Manages parallel file downloads with synchronization
 - `Resource Efficiency`: Stream processing avoids large memory allocations
 - `Graceful Degradation`: Processes available files when some downloads fail


### Run service: 
```bash
go run cmd/server/main.go
```


### Postman Testing
1. Import Test_Ex_zip.postman_collection.json
2. Test scenarios:
    - Task creation and file addition
    - Invalid URL handling
    - Concurrent task limiting
    - Archive download verification