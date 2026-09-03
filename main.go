package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Task struct {
	Id        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	baseURL = "http://localhost:8080"
)

// Функции для БД

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "./tasks.db")
	if err != nil {
		fmt.Println("Ошибка при создании базы данных!")
		return
	}
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	done INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(query)
	if err != nil {
		fmt.Println("Ошибка при создании таблицы базы данных!", err)
		return
	}
	fmt.Println("База данных подключена!")
}

// http сервер
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
	rows, err := db.Query("SELECT id, title, done, created_at FROM tasks ORDER BY id")
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	list := []Task{}
	for rows.Next() {
		var task Task
		err := rows.Scan(&task.Id, &task.Title, &task.Done, &task.CreatedAt)
		if err != nil {
			continue
		}
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
		return
	}
	res, err := db.Exec("INSERT INTO tasks (title, done) VALUES (?, 0)", req.Title)
	if err != nil {
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	var task Task
	row := db.QueryRow("SELECT id, title, done, created_at FROM tasks WHERE id = ?", id)
	row.Scan(&task.Id, &task.Title, &task.Done, &task.CreatedAt)
	w.WriteHeader(http.StatusCreated)
	sendJson(w, task)
}

func getTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	var task Task
	row := db.QueryRow("SELECT id, title, done, created_at FROM tasks WHERE id = ?", id)
	err := row.Scan(&task.Id, &task.Title, &task.Done, &task.CreatedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	sendJson(w, task)
}

func doneTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	res, err := db.Exec("UPDATE tasks SET done = 1 WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	var task Task
	row := db.QueryRow("SELECT id, title, done, created_at FROM tasks WHERE id = ?", id)
	row.Scan(&task.Id, &task.Title, &task.Done, &task.CreatedAt)
	sendJson(w, task)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	res, err := db.Exec("DELETE FROM tasks WHERE id = ?", id)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Клиентские функции
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

// Основная программа

func main() {
	initDB()
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
