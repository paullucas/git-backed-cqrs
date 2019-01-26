package main

import (
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strconv"
	"strings"
)

// CreateTodoList command
type CreateTodoList struct {
	ID   string
	Name string
}

// TodoListCreated event
type TodoListCreated struct {
	ID   string
	Name string
}

// Event interface
type Event interface {
	getID() string
	getName() string
}

// Event getter implementations
func (event TodoListCreated) getID() string   { return event.ID }
func (event TodoListCreated) getName() string { return event.Name }

// TodoList state
type TodoList struct{ Name string }

// GUID generates a UUID
func GUID() string {
	guid, _ := uuid.NewV4()
	return guid.String()
}

// Git command execution from a given path
func Git(path string, command string, args ...string) error {
	err := exec.Command("git", append([]string{"-C", path, command}, args...)...).Run()
	return err
}

// Commit all changes to Git
func Commit(path string, msg string) error {
	err := Git(path, "add", ".")
	if err != nil {
		return err
	}

	err = Git(path, "commit", "-m", msg)
	return err
}

// InitStorage creates a Git repository, as well as event-stream & projection files
func InitStorage(repo string) error {
	// Check for a repo, proceed if not found
	_, err := os.Stat(repo)
	if err != nil {

		// Create Git repo & directory
		err = Git(".", "init", repo)
		if err != nil {
			return err
		}

		// Create sub-directories
		err = os.Mkdir(fmt.Sprintf("%s/event-stream", repo), 0700)
		if err != nil {
			return err
		}
		err = os.Mkdir(fmt.Sprintf("%s/projections", repo), 0700)
		if err != nil {
			return err
		}

		// Create .gitignore files
		_, err = os.OpenFile(fmt.Sprintf("%s/projections/.gitignore", repo), os.O_CREATE, 0700)
		if err != nil {
			return err
		}
		_, err = os.OpenFile(fmt.Sprintf("%s/event-stream/.gitignore", repo), os.O_CREATE, 0700)
		if err != nil {
			return err
		}

		// Initialize projections
		err = ioutil.WriteFile(fmt.Sprintf("%s/projections/todoLists", repo), []byte("[]"), 0700)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(fmt.Sprintf("%s/projections/TodoListsCount", repo), []byte("0"), 0700)
		if err != nil {
			return err
		}

		// Stage & commit changes
		err = Commit(repo, "Initial commit")
		if err != nil {
			return err
		}
	}
	return err
}

// UpdateStream appends an event to an event stream
func UpdateStream(repo string, event Event) (string, error) {

	// Stream directory path
	streamDir := path.Join(fmt.Sprintf("%s/event-stream", repo), event.getID())

	// Read stream directory
	dirs, _ := ioutil.ReadDir(streamDir)

	// Create stream directory if not found
	if len(dirs) == 0 {
		err := os.Mkdir(streamDir, 0755)
		if err != nil {
			fmt.Printf("Mkdir err = %v\n", err)
			return "", err
		}
	}

	// Generate filename
	seqNum := len(dirs) + 1
	cmdStrs := strings.Split(reflect.TypeOf(event).String(), ".")
	eventString := cmdStrs[len(cmdStrs)-1]
	fileName := path.Join(streamDir, fmt.Sprintf("%06d", seqNum)+"_"+eventString)

	// Create file
	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0700)
	if err != nil {
		return fileName, err
	}
	defer f.Close()

	// Serialize event to JSON
	serializedBytes, err := json.Marshal(event)
	if err != nil {
		return fileName, err
	}

	// Append event to file
	_, err = f.Write(serializedBytes)
	if err != nil {
		return fileName, err
	}

	return fileName, err
}

// UpdateStreamIndex keeps track of the most recent event
func UpdateStreamIndex(repo string, eventName string) error {
	// Open stream index
	f, err := os.OpenFile(fmt.Sprintf("%s/event-stream/index", repo), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	defer f.Close()

	// Append event name
	_, err = f.WriteString(eventName + "\n")
	return err
}

// ReadTodoListsProjection parses the todoLists projection
func ReadTodoListsProjection(repo string) ([]TodoList, error) {
	var todoLists []TodoList

	f, err := ioutil.ReadFile(fmt.Sprintf("%s/projections/todoLists", repo))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(f, &todoLists)
	return todoLists, err
}

// ReadTodoListsCountProjection parses the TodoListsCount projection
func ReadTodoListsCountProjection(repo string) (int, error) {
	var count int

	f, err := ioutil.ReadFile(fmt.Sprintf("%s/projections/TodoListsCount", repo))
	if err != nil {
		return count, err
	}

	count, err = strconv.Atoi(string(f))
	return count, err
}

// UpdateListOfTodos updates the todoLists projection
func UpdateListOfTodos(repo string, event Event) error {
	todos, err := ReadTodoListsProjection(repo)
	if err != nil {
		return err
	}

	todo := TodoList{event.getName()}
	updatedTodos, err := json.Marshal(append(todos, todo))
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(fmt.Sprintf("%s/projections/todoLists", repo), updatedTodos, 0700)
	return err
}

// UpdateCountOfTodoLists updates (increments) the TodoListsCount projection
func UpdateCountOfTodoLists(repo string, event Event) error {
	count, err := ReadTodoListsCountProjection(repo)
	if err != nil {
		return err
	}

	updatedCount := []byte(strconv.Itoa(count + 1))
	err = ioutil.WriteFile(fmt.Sprintf("%s/projections/TodoListsCount", repo), updatedCount, 0700)
	return err
}

// Handle an event, update read models
func Handle(repo string, event Event) error {
	// switch reflect.TypeOf(event).String() {
	switch event.(type) {

	case TodoListCreated:
		err := UpdateListOfTodos(repo, event)
		if err != nil {
			return err
		}

		err = UpdateCountOfTodoLists(repo, event)
		if err != nil {
			return err
		}
	}

	return nil
}

// StoreEvent stores an event on an event stream
func StoreEvent(repo string, event Event) error {
	name, err := UpdateStream(repo, event)
	if err != nil {
		return err
	}

	err = UpdateStreamIndex(repo, name)
	if err != nil {
		return err
	}

	err = Handle(repo, event)
	if err != nil {
		return err
	}

	err = Commit(repo, event.getID())
	return err
}

// Validate CreateTodoList command
func (command *CreateTodoList) Validate(repo string) error {
	// Don't accept empty name values
	if command.Name == "" {
		return fmt.Errorf("Command validation failed")
	}

	return nil
}

var (
	// Logger
	info = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)

	// HTML templates
	pageTemplates = template.Must(template.ParseGlob("templates/*.html"))
)

// ShowTodoListsHandler handles HTTP requests for TodoLists state queries
func ShowTodoListsHandler(repo string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		todoLists, err := ReadTodoListsProjection(repo)
		if err != nil {
			info.Println(fmt.Printf("ShowTodoListsHandler: ReadTodoListsProjection err = %v\n", err))
			http.Error(w, "Internal Server Error", 500)
			return
		}

		err = pageTemplates.ExecuteTemplate(w, "todoLists.html", todoLists)
		if err != nil {
			info.Println(fmt.Printf("ShowTodoListsHandler: ExecuteTemplate err = %v\n", err))
			http.Error(w, "Internal Server Error", 500)
			return
		}
	}
}

// CreateTodoListFormHandler handles HTTP requests for the createTodoList form
func CreateTodoListFormHandler(w http.ResponseWriter, r *http.Request) {
	err := pageTemplates.ExecuteTemplate(w, "createTodoList.html", nil)
	if err != nil {
		info.Println(fmt.Printf("CreateTodoListFormHandler: ExecuteTemplate err = %v\n", err))
		http.Error(w, "Internal Server Error", 500)
		return
	}
}

// CreateTodoListHandler handles HTTP requests for the createTodoList command
func CreateTodoListHandler(repo string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		command := &CreateTodoList{GUID(), r.FormValue("name")}

		// Validate command
		err := command.Validate(repo)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		// Store event
		event := TodoListCreated{command.ID, command.Name}
		err = StoreEvent(repo, event)
		if err != nil {
			info.Println(fmt.Printf("CreateTodoListHandler: StoreEvent err = %v\n", err))
			http.Error(w, "Internal Server Error", 500)
			return
		}

		// Command as response
		res, err := json.Marshal(command)
		if err != nil {
			info.Println(fmt.Printf("CreateTodoListHandler: JSON encoding err = %v\n", err))
			http.Error(w, "Internal Server Error", 500)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write(res)
	}
}

func main() {
	const repo = "storage"

	// Initialize repo
	err := InitStorage(repo)
	if err != nil {
		panic(err)
	}

	// Routes
	http.HandleFunc("/", ShowTodoListsHandler(repo))
	http.HandleFunc("/createTodoList", CreateTodoListHandler(repo))
	http.HandleFunc("/createTodoListForm", CreateTodoListFormHandler)

	// Server
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
