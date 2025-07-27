package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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

	if v, err := g.SetView("help", 0, 0, maxX-1, 6, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Wrap = true
		fmt.Fprintln(v, "c: Создать задачу | a: Добавить файл | s: Статус | d: Скачать")
		fmt.Fprintln(v, "Tab: Переключение | Ctrl+C: Выход")
	}

	tasksHeight := maxY/2 - 3
	if v, err := g.SetView("tasks", 0, 7, maxX/2, 7+tasksHeight, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Highlight = true
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		s.updateTasksView(v)
	}

	if v, err := g.SetView("output", maxX/2+1, 7, maxX-1, 7+tasksHeight, 0); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Autoscroll = true
		v.Wrap = true
		s.updateOutputView(v)
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

	baseTitles := map[string]string{
		"help":   "Инструкции",
		"tasks":  "Задачи",
		"output": "Вывод",
		"input":  "Команда",
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
		if err := s.g.SetKeybinding(view, 'a', gocui.ModNone, s.addFilePrompt); err != nil {
			return err
		}
		if err := s.g.SetKeybinding(view, 's', gocui.ModNone, s.showStatus); err != nil {
			return err
		}
		if err := s.g.SetKeybinding(view, 'd', gocui.ModNone, s.downloadArchive); err != nil {
			return err
		}
	}

	if err := s.g.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, s.executeCommand); err != nil {
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
	}
	s.updateTasksView(v)
	return nil
}

func (s *GUIState) cursorUp(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		return nil
	}
	if s.SelectedTask > 0 {
		s.SelectedTask--
	}
	s.updateTasksView(v)
	return nil
}

func (s *GUIState) nextView(g *gocui.Gui, v *gocui.View) error {
	views := []string{"input", "tasks", "output"}
	currentView := v.Name()

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

func (s *GUIState) updateTasksView(v *gocui.View) {
	v.Clear()
	for i, task := range s.Tasks {
		if i == s.SelectedTask {
			fmt.Fprintf(v, "> %s [%s]\n", task.ID, task.Status)
		} else {
			fmt.Fprintf(v, "  %s [%s]\n", task.ID, task.Status)
		}
	}
}

func (s *GUIState) updateOutputView(v *gocui.View) {
	v.Clear()
	for _, line := range s.OutputLines {
		fmt.Fprintln(v, line)
	}
}

func (s *GUIState) quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (s *GUIState) executeCommand(g *gocui.Gui, v *gocui.View) error {
	command := strings.TrimSpace(v.Buffer())
	v.Clear()
	v.SetCursor(0, 0)
	s.InputBuffer = ""

	switch {
	case strings.HasPrefix(command, "c"):
		s.createTask(g, v)
	case strings.HasPrefix(command, "a"):
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
	case strings.HasPrefix(command, "s"):
		s.showStatus(g, v)
	case strings.HasPrefix(command, "d"):
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
			s.updateOutputView(v)
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
		v, err := g.View("tasks")
		if err == nil {
			s.updateTasksView(v)
		}
		return nil
	})

	return nil
}

func (s *GUIState) addFile(g *gocui.Gui, v *gocui.View, taskID, url string) error {
	data := map[string]string{"url": url}
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
				s.updateTasksView(v)
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
	s.showStatusForTask(taskID)
	return nil
}

func (s *GUIState) showStatusForTask(taskID string) {
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

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		s.addOutput(string(body))
	} else {
		s.addOutput(prettyJSON.String())
	}
}

func (s *GUIState) downloadArchive(g *gocui.Gui, v *gocui.View) error {
	if len(s.Tasks) == 0 {
		s.addOutput("No tasks available")
		return nil
	}
	taskID := s.Tasks[s.SelectedTask].ID
	s.downloadArchiveForTask(taskID)
	return nil
}

func (s *GUIState) downloadArchiveForTask(taskID string) {
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

func (s *GUIState) addFilePrompt(g *gocui.Gui, v *gocui.View) error {
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
