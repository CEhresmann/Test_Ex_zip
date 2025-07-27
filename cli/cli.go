package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/awesome-gocui/gocui"
)

type GUIState struct {
	g            *gocui.Gui
	Tasks        []TaskInfo
	SelectedTask int
	InputBuffer  string
	OutputLines  []string
}

type TaskInfo struct {
	ID      string
	Status  string
	Files   []string
	Archive string
}

type FileInfo struct {
	URL    string `json:"url"`
	Status string `json:"status"`
}

type TaskStatus struct {
	ID        string     `json:"id"`
	Status    string     `json:"status"`
	Files     []FileInfo `json:"files"`
	Archive   string     `json:"archive"`
	CreatedAt string     `json:"created_at"`
}

func StartGUI() error {
	g, err := gocui.NewGui(gocui.OutputNormal, false)
	if err != nil {
		return err
	}
	defer g.Close()

	state := &GUIState{
		g:           g,
		OutputLines: []string{"Welcome :)", "Press 'c' to create new task", "Press 'a' to add file to task", "Press 's' to show task status", "Press 'd' to download archive"},
	}

	g.SetManagerFunc(state.layout)

	if err := state.keybindings(); err != nil {
		return err
	}

	if err = g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		return err
	}

	return nil
}

func (s *GUIState) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	baseTitles := map[string]string{
		"help":   "Инструкции",
		"tasks":  "Задачи",
		"output": "Вывод",
		"input":  "Команда",
	}

	if v, err := g.SetView("help", 0, 0, maxX-1, 6, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Wrap = true
		fmt.Fprintln(v, "c: Создать задачу | a: Добавить файл | s: Статус | d: Скачать")
		fmt.Fprintln(v, "Tab: Переключение | Стрелки: Выбор задачи | Ctrl+C: Выход")

	}

	tasksHeight := maxY/2 - 3
	if v, err := g.SetView("tasks", 0, 7, maxX/2, 7+tasksHeight, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack

		v.Editable = false
		v.Wrap = false

		v.SetCursor(0, s.SelectedTask)

		s.updateTView(v, g)
	}

	if v, err := g.SetView("output", maxX/2+1, 7, maxX-1, 7+tasksHeight, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Autoscroll = true
		v.Wrap = true
		s.updateOView(v)
	}

	if v, err := g.SetView("input", 0, maxY-4, maxX-1, maxY-1, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Editable = true
		v.Wrap = true
		fmt.Fprint(v, s.InputBuffer)
		if _, err := g.SetCurrentView("input"); err != nil {
			return err
		}
	}

	currentView := g.CurrentView()
	for name, baseTitle := range baseTitles {
		v, err := g.View(name)
		if err != nil {
			continue
		}
		if currentView != nil && currentView.Name() == name {
			v.Title = ">>> " + baseTitle
		} else {
			v.Title = "    " + baseTitle
		}
	}

	return nil
}

func (s *GUIState) keybindings() error {
	if err := s.g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, s.quit); err != nil {
		return err
	}
	if err := s.g.SetKeybinding("", gocui.KeyTab, gocui.ModNone, s.nextView); err != nil {
		return err
	}

	if err := s.g.SetKeybinding("tasks", gocui.KeyArrowDown, gocui.ModNone, s.cursorDown); err != nil {
		return err
	}
	if err := s.g.SetKeybinding("tasks", gocui.KeyArrowUp, gocui.ModNone, s.cursorUp); err != nil {
		return err
	}
	if err := s.g.SetKeybinding("tasks", 'j', gocui.ModNone, s.cursorDown); err != nil {
		return err
	}
	if err := s.g.SetKeybinding("tasks", 'k', gocui.ModNone, s.cursorUp); err != nil {
		return err
	}

	views := []string{"help", "tasks", "output", "input"}
	for _, view := range views {
		if err := s.g.SetKeybinding(view, 'c', gocui.ModNone, s.createTask); err != nil {
			return err
		}
		if err := s.g.SetKeybinding(view, 'a', gocui.ModNone, s.promt); err != nil {
			return err
		}
		if err := s.g.SetKeybinding(view, 's', gocui.ModNone, s.showStatus); err != nil {
			return err
		}
		if err := s.g.SetKeybinding(view, 'd', gocui.ModNone, s.downloadArchive); err != nil {
			return err
		}
	}

	if err := s.g.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, s.runCommand); err != nil {
		return err
	}

	return nil
}

func (s *GUIState) cursorDown(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		return nil
	}
	if s.SelectedTask < len(s.Tasks)-1 {
		s.SelectedTask++
	} else {
		s.SelectedTask = 0
	}

	if tasksView, err := g.View("tasks"); err == nil {
		s.updateTView(tasksView, g)
	}
	return nil
}

func (s *GUIState) cursorUp(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		return nil
	}
	if s.SelectedTask > 0 {
		s.SelectedTask--
	} else {
		s.SelectedTask = len(s.Tasks) - 1
	}

	if tasksView, err := g.View("tasks"); err == nil {
		s.updateTView(tasksView, g)
	}
	return nil
}

func (s *GUIState) nextView(g *gocui.Gui, v *gocui.View) error {
	views := []string{"tasks", "output", "input"}
	currentView := v.Name()
	if currentView == "" {
		_, err := g.SetCurrentView(views[0])
		return err
	}
	nextIndex := 0
	for i, name := range views {
		if name == currentView {
			nextIndex = (i + 1) % len(views)
			break
		}
	}

	_, err := g.SetCurrentView(views[nextIndex])
	return err
}

func (s *GUIState) updateTView(v *gocui.View, g *gocui.Gui) {
	v.Clear()
	_, maxY := g.Size()
	tasksHeight := maxY/2 - 3
	for _, task := range s.Tasks {
		fmt.Fprintf(v, "%s [%s]\n", task.ID, task.Status)
	}

	if len(s.Tasks) > 0 {
		if s.SelectedTask >= len(s.Tasks) {
			s.SelectedTask = len(s.Tasks) - 1
		}
		if s.SelectedTask < 0 {
			s.SelectedTask = 0
		}

		v.SetCursor(0, s.SelectedTask)

		v.SetOrigin(0, 0)
		if s.SelectedTask >= tasksHeight-1 {
			v.SetOrigin(0, s.SelectedTask-tasksHeight+2)
		}
	}
}

func (s *GUIState) updateOView(v *gocui.View) {
	v.Clear()
	for _, line := range s.OutputLines {
		fmt.Fprintln(v, line)
	}
}

func (s *GUIState) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (s *GUIState) runCommand(g *gocui.Gui, v *gocui.View) error {
	command := strings.TrimSpace(v.Buffer())
	v.Clear()
	v.SetCursor(0, 0)
	s.InputBuffer = ""

	switch {
	case command == "c":
		s.createTask(g, v)
	case strings.HasPrefix(command, "a "):
		url := strings.TrimSpace(strings.TrimPrefix(command, "a"))
		if url == "" {
			s.addOutput("Usage: a <url>")
			return nil
		}
		if len(s.Tasks) == 0 {
			s.addOutput("No tasks available")
			return nil
		}
		taskID := s.Tasks[s.SelectedTask].ID
		s.addFile(g, v, taskID, url)
	case command == "s":
		s.showStatus(g, v)
	case command == "d":
		s.downloadArchive(g, v)
	default:
		s.addOutput("Unknown command: " + command)
	}
	return nil
}

func (s *GUIState) addOutput(line string) {
	s.OutputLines = append(s.OutputLines, line)
	s.g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("output"); err == nil {
			s.updateOView(v)
		}
		return nil
	})
}

func (s *GUIState) createTask(g *gocui.Gui, v *gocui.View) error {
	resp, err := http.Post("http://localhost:8080/tasks", "application/json", nil)
	if err != nil {
		s.addOutput("Error creating task: " + err.Error())
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		s.addOutput(fmt.Sprintf("Error: server returned %d", resp.StatusCode))
		return nil
	}

	var result struct{ ID string }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.addOutput("Error decoding response: " + err.Error())
		return nil
	}

	s.Tasks = append(s.Tasks, TaskInfo{
		ID:     result.ID,
		Status: "pending",
	})
	s.SelectedTask = len(s.Tasks) - 1
	s.addOutput("Created task: " + result.ID)

	g.Update(func(g *gocui.Gui) error {
		if v, err := g.View("tasks"); err == nil {
			s.updateTView(v, g)
		}
		return nil
	})

	return nil
}

func transGDLink(original string) string {
	if strings.Contains(original, "drive.google.com/file/d/") {
		re := regexp.MustCompile(`/file/d/([^/]+)`)
		matches := re.FindStringSubmatch(original)
		if len(matches) > 1 {
			fileID := matches[1]
			return "https://drive.google.com/uc?id=" + fileID + "&export=download"
		}
	}
	return original
}

func isValidExt(uri string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		return false
	}

	ext := strings.ToLower(filepath.Ext(u.Path))
	return ext == ".pdf" || ext == ".jpg"
}

func (s *GUIState) addFile(g *gocui.Gui, v *gocui.View, taskID, url string) error {
	transformedURL := transGDLink(url)
	if transformedURL != url {
		s.addOutput("Transformed URL: " + transformedURL)
	}

	if !isValidExt(transformedURL) {
		s.addOutput("Error: Only PDF and JPEG files are allowed")
		return nil
	}

	data := map[string]string{"url": transformedURL}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(
		"http://localhost:8080/tasks/"+taskID,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		s.addOutput("Error adding file: " + err.Error())
		return nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNoContent:
		s.addOutput("Added file to task " + taskID)
		for i := range s.Tasks {
			if s.Tasks[i].ID == taskID {
				s.Tasks[i].Files = append(s.Tasks[i].Files, url)
				if len(s.Tasks[i].Files) == 3 {
					s.Tasks[i].Status = "processing"
				}
				break
			}
		}
		g.Update(func(g *gocui.Gui) error {
			v, err := g.View("tasks")
			if err == nil {
				s.updateTView(v, g)
			}
			return nil
		})
	case http.StatusNotFound:
		s.addOutput("Task not found: " + taskID)
	case http.StatusBadRequest:
		s.addOutput("Invalid request: " + resp.Status)
	default:
		s.addOutput(fmt.Sprintf("Error: server returned %d", resp.StatusCode))
	}

	return nil
}

func (s *GUIState) showStatus(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		s.addOutput("No tasks available")
		return nil
	}
	taskID := s.Tasks[s.SelectedTask].ID
	s.showStatusLittle(taskID)
	return nil
}

func (s *GUIState) showStatusLittle(taskID string) {
	resp, err := http.Get("http://localhost:8080/status/" + taskID)
	if err != nil {
		s.addOutput("Error getting status: " + err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.addOutput(fmt.Sprintf("Error: server returned %d", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		s.addOutput("Error reading response: " + err.Error())
		return
	}

	var status TaskStatus
	if err := json.Unmarshal(body, &status); err != nil {
		s.addOutput("Error parsing status: " + err.Error())
		return
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Task ID: %s\n", taskID))
	output.WriteString(fmt.Sprintf("Status: %s\n", status.Status))
	output.WriteString(fmt.Sprintf("Created at: %s\n", status.CreatedAt))

	output.WriteString("Files:\n")
	for i, file := range status.Files {
		output.WriteString(fmt.Sprintf("  %d. URL: %s\n", i+1, file.URL))
		output.WriteString(fmt.Sprintf("     Status: %s\n", file.Status))
	}

	if status.Archive != "" {
		output.WriteString(fmt.Sprintf("Archive: %s\n", status.Archive))
	} else {
		output.WriteString("Archive: not ready\n")
	}

	s.addOutput(output.String())
}

func (s *GUIState) downloadArchive(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		s.addOutput("No tasks available")
		return nil
	}
	taskID := s.Tasks[s.SelectedTask].ID
	s.downloadArchiveLittle(taskID)
	return nil
}

func (s *GUIState) downloadArchiveLittle(taskID string) {
	resp, err := http.Get("http://localhost:8080/download/" + taskID)
	if err != nil {
		s.addOutput("Error downloading archive: " + err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.addOutput(fmt.Sprintf("Download failed: %d %s", resp.StatusCode, resp.Status))
		return
	}

	filename := taskID + ".zip"
	file, err := os.Create(filename)
	if err != nil {
		s.addOutput("Error creating file: " + err.Error())
		return
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		s.addOutput("Error saving archive: " + err.Error())
		return
	}

	s.addOutput("Archive saved as " + filename)
}

func (s *GUIState) promt(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		s.addOutput("No tasks available")
		return nil
	}

	s.InputBuffer = "a "
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View("input")
		if err == nil {
			v.Clear()
			fmt.Fprint(v, s.InputBuffer)
		}
		_, err = g.SetCurrentView("input")
		return err
	})
	return nil
}
