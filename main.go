package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Task struct {
	Id        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	tasks   = make(map[int]Task)
	nextID  = 1
	baseURL = "http://localhost:8080"
	mu      sync.Mutex
)

func startServer() {
	http.HandleFunc("/tasks", tasksHandler)
	http.HandleFunc("/tasks/", taskHandler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Ошибка запуска сервера: ", err)
	}
	fmt.Println("Сервер запущен!")
}

func tasksHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTasksHandler(w, r)
	case http.MethodPost:
		createTaskHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func taskHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/tasks/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
	}
	switch r.Method {
	case http.MethodGet:
		getTaskHandler(w, r, id)
	case http.MethodPut:
		doneTaskHandler(w, r, id)
	case http.MethodDelete:
		deleteTaskHandler(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func sendJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	list := []Task{}
	for _, task := range tasks {
		list = append(list, task)
	}
	sendJson(w, list)
}

func createTaskHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Error parsing json", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "Title is required", http.StatusBadRequest)
	}
	mu.Lock()
	defer mu.Unlock()
	task := Task{
		Id:        nextID,
		Title:     req.Title,
		Done:      false,
		CreatedAt: time.Now(),
	}
	tasks[nextID] = task
	nextID++
	w.WriteHeader(http.StatusCreated)
	sendJson(w, task)
}

func getTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	mu.Lock()
	defer mu.Unlock()
	task, ok := tasks[id]
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	sendJson(w, task)
}

func doneTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	mu.Lock()
	defer mu.Unlock()
	task, ok := tasks[id]
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	task.Done = true
	tasks[id] = task
	sendJson(w, task)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	mu.Lock()
	defer mu.Unlock()
	_, ok := tasks[id]
	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	delete(tasks, id)
	w.WriteHeader(http.StatusNoContent)
	sendJson(w, tasks)
}

// CLI
func addTask() {
	title := os.Args[2]
	jsonData := fmt.Sprintf(`{"title":"%s"}`, title)
	resp, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(jsonData))
	if err != nil {
		fmt.Println("Ошибка отправки запроса на сервер!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		fmt.Println("Ошибка добавления задачи!", err)
		return
	}
	var task Task
	json.NewDecoder(resp.Body).Decode(&task)
	fmt.Printf("Зачада добавлена! Номер = %d , текст \"%s\"", task.Id, task.Title)
}

func getAllTasks() {
	resp, err := http.Get(baseURL + "/tasks")
	if err != nil {
		fmt.Println("Ошибка отправки запроса на сервер!", err)
		return
	}
	defer resp.Body.Close()
	var list []Task
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) == 0 {
		fmt.Println("Нет задач!")
		return
	}
	for _, task := range list {
		status := "не выполнена"
		if task.Done {
			status = "выполнена"
		}
		fmt.Printf("Номер задачи: %d,\t текст задачи: %s,\t статус выполнения: %s.\n", task.Id, task.Title, status)
	}
}

func getTask() {
	idTask := os.Args[2]
	resp, err := http.Get(baseURL + "/tasks/" + idTask)
	if err != nil {
		fmt.Println("Ошибка отправки запроса!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("Задача с номером %s не найдена!", idTask)
		return
	}
	var task Task
	json.NewDecoder(resp.Body).Decode(&task)
	status := "не выполнена"
	if task.Done {
		status = "выполнена"
	}
	fmt.Printf("Номер задачи: %d,\t текст задачи: %s,\t статус выполнения: %s.\n", task.Id, task.Title, status)
}

func doneTask() {
	idTask := os.Args[2]
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/tasks/"+idTask, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Ошибка отправки запроса!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("Задача с номером %s не найдена!", idTask)
		return
	}
	fmt.Printf("Задача %s отмечена как выполненная!", idTask)
}

func deleteTask() {
	idTask := os.Args[2]
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/tasks/"+idTask, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Ошибка отправки запроса!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		fmt.Printf("Задача с номером %s не найдена!", idTask)
		return
	}
	fmt.Printf("Задача %s удалена!", idTask)
}

func main() {
	if len(os.Args) < 2 {
		startServer()
		return
	}
	command := os.Args[1]
	switch command {
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("Не указан текст задачи")
			return
		}
		addTask()
	case "list":
		getAllTasks()
	case "get":
		if len(os.Args) < 3 {
			fmt.Println("Не указан номер задачи")
			return
		}
		getTask()
	case "done":
		if len(os.Args) < 3 {
			fmt.Println("Не указан номер задачи")
			return
		}
		doneTask()
	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Не указан номер задачи")
			return
		}
		deleteTask()
	case "server":
		startServer()
	default:
		fmt.Printf("Неизвестная команда!\nВот список команд:\n server - запуск сервера;\n add \"задача\" - добавление новой задачи;\n get <id> - открыть задачу по номеру;\n list - просмотреть все задачи;\n done <id>- отметить задачу как выполненную;\n delete <id> - удалить задачу по номеру\n")
	}
}
