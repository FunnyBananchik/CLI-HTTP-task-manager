package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
	logger  *log.Logger
)

//Функция для логирования

func loggingRequest(handler func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		begin := time.Now()
		logger.Printf("[%s] %s", r.Method, r.URL.Path)
		handler(w, r)
		logger.Printf("Запрос обработан за %v", time.Since(begin))
	}
}

func errorResponse(w http.ResponseWriter, mes string, status int) {
	logger.Printf("[ERROR] %d %s", status, mes)
	http.Error(w, mes, status)
}

// Функции для БД

var db *sql.DB

func initDB() {
	logger.Println("[DB] Подключение базы данных!")
	var err error
	db, err = sql.Open("sqlite", "./tasks.db")
	if err != nil {
		logger.Println("[ERROR] Ошибка при создании базы данных!")
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
		logger.Println("[ERROR] Ошибка при создании таблицы базы данных!", err)
		return
	}
	logger.Println("[DB] База данных подключена!")
}

// http сервер
func startServer() {
	logger.Println("[SERVER] Запуск сервера")
	http.HandleFunc("/tasks", loggingRequest(tasksHandler))
	http.HandleFunc("/tasks/", loggingRequest(taskHandler))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		logger.Println("[ERROR] Ошибка запуска сервера: ", err)
	}
}

func tasksHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTasksHandler(w, r)
	case http.MethodPost:
		createTaskHandler(w, r)
	default:
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func taskHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/tasks/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		errorResponse(w, "Invalid ID", http.StatusBadRequest)
	}
	switch r.Method {
	case http.MethodGet:
		getTaskHandler(w, r, id)
	case http.MethodPut:
		doneTaskHandler(w, r, id)
	case http.MethodDelete:
		deleteTaskHandler(w, r, id)
	default:
		errorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func sendJson(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, title, done, created_at FROM tasks ORDER BY id")
	if err != nil {
		errorResponse(w, "Database error", http.StatusInternalServerError)
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
		errorResponse(w, "Error parsing json", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		errorResponse(w, "Title is required", http.StatusBadRequest)
		return
	}
	res, err := db.Exec("INSERT INTO tasks (title, done) VALUES (?, 0)", req.Title)
	if err != nil {
		errorResponse(w, "Failed to create task", http.StatusInternalServerError)
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
		errorResponse(w, "Task not found", http.StatusNotFound)
		return
	} else if err != nil {
		errorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}
	sendJson(w, task)
}

func doneTaskHandler(w http.ResponseWriter, r *http.Request, id int) {
	res, err := db.Exec("UPDATE tasks SET done = 1 WHERE id = ?", id)
	if err != nil {
		errorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		errorResponse(w, "Task not found", http.StatusNotFound)
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
		errorResponse(w, "Database error", http.StatusInternalServerError)
		return
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		errorResponse(w, "Task not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Клиентские функции
func addTask() {
	title := os.Args[2]
	logger.Println("[CLI] Запрос на добавление задачи")
	jsonData := fmt.Sprintf(`{"title":"%s"}`, title)
	resp, err := http.Post(baseURL+"/tasks", "application/json", strings.NewReader(jsonData))
	if err != nil {
		logger.Printf("[ERROR] Ошибка отправки запроса: %v", err)
		fmt.Println("Ошибка отправки запроса на сервер!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		logger.Printf("[ERROR] Ошибка добавления задачи: %v", err)
		fmt.Println("Ошибка добавления задачи!", err)
		return
	}
	var task Task
	json.NewDecoder(resp.Body).Decode(&task)
	logger.Printf("[CLI] Задача добавлена: Номер = %d , текст \"%s\"", task.Id, task.Title)
	fmt.Printf("Задача добавлена! Номер = %d , текст \"%s\"", task.Id, task.Title)
}

func getAllTasks() {
	logger.Println("[CLI] Запрос на вывод всех задач")
	resp, err := http.Get(baseURL + "/tasks")
	if err != nil {
		logger.Printf("[ERROR] Ошибка отправки запроса: %v", err)
		fmt.Println("Ошибка отправки запроса на сервер!", err)
		return
	}
	defer resp.Body.Close()
	var list []Task
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) == 0 {
		logger.Println("[WARN] Нет задач")
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
	logger.Println("[CLI] Запрос на вывод всех задач выполнен")
}

func getTask() {
	idTask := os.Args[2]
	logger.Printf("[CLI] Запрос на вывод задачи %s", idTask)
	resp, err := http.Get(baseURL + "/tasks/" + idTask)
	if err != nil {
		logger.Printf("[ERROR] Ошибка отправки запроса: %v", err)
		fmt.Println("Ошибка отправки запроса!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		logger.Printf("[WARN] Задача с номером %s не найдена!", idTask)
		fmt.Printf("Задача с номером %s не найдена!", idTask)
		return
	}
	var task Task
	json.NewDecoder(resp.Body).Decode(&task)
	status := "не выполнена"
	if task.Done {
		status = "выполнена"
	}
	logger.Printf("[CLI] Запрос на вывод задачи с номером %s выполнен", idTask)
	fmt.Printf("Номер задачи: %d,\t текст задачи: %s,\t статус выполнения: %s.\n", task.Id, task.Title, status)
}

func doneTask() {
	idTask := os.Args[2]
	logger.Printf("[CLI] Запрос на изменение статуса задачи %s", idTask)
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/tasks/"+idTask, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("[ERROR] Ошибка отправки запроса: %v", err)
		fmt.Println("Ошибка отправки запроса!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		logger.Printf("[WARN] Задача с номером %s не найдена", idTask)
		fmt.Printf("Задача с номером %s не найдена!", idTask)
		return
	}
	logger.Printf("[CLI] Задача %s отмечена как выполненная", idTask)
	fmt.Printf("Задача %s отмечена как выполненная!", idTask)
}

func deleteTask() {
	idTask := os.Args[2]
	logger.Printf("[CLI] Запрос на удаление задачи %s", idTask)
	req, _ := http.NewRequest(http.MethodDelete, baseURL+"/tasks/"+idTask, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("[ERROR] Ошибка отправки запроса: %v", err)
		fmt.Println("Ошибка отправки запроса!", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		logger.Printf("[WARN] Задача с номером %s не найдена", idTask)
		fmt.Printf("Задача с номером %s не найдена!", idTask)
		return
	}
	logger.Printf("[CLI] Задача %s удалена", idTask)
	fmt.Printf("Задача %s удалена!", idTask)
}

// Основная программа

func main() {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger = log.New(file, "[TaskManager]", log.LstdFlags)
	initDB()
	defer db.Close()
	if len(os.Args) < 2 {
		startServer()
		return
	}
	command := os.Args[1]
	switch command {
	case "add":
		if len(os.Args) < 3 {
			logger.Println("[TaskManager] Не указан текст задачи")
			fmt.Println("Не указан текст задачи")
			return
		}
		addTask()
	case "list":
		getAllTasks()
	case "get":
		if len(os.Args) < 3 {
			logger.Println("[TaskManager] Не указан номер задачи")
			fmt.Println("Не указан номер задачи")
			return
		}
		getTask()
	case "done":
		if len(os.Args) < 3 {
			logger.Println("[TaskManager] Не указан номер задачи")
			fmt.Println("Не указан номер задачи")
			return
		}
		doneTask()
	case "delete":
		if len(os.Args) < 3 {
			logger.Println("[TaskManager] Не указан номер задачи")
			fmt.Println("Не указан номер задачи")
			return
		}
		deleteTask()
	case "server":
		startServer()
	default:
		logger.Println("[TaskManager] Неизвестная команда")
		fmt.Printf("Неизвестная команда!\nВот список команд:\n server - запуск сервера;\n add \"задача\" - добавление новой задачи;\n get <id> - открыть задачу по номеру;\n list - просмотреть все задачи;\n done <id>- отметить задачу как выполненную;\n delete <id> - удалить задачу по номеру\n")
	}
}
