package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var rdb *redis.Client

func main() {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // default to localhost if no env variable
	}

	rdb = redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	_, err := readCache("tasks")
	if err != nil {
		log.Println("No cache found for key 'tasks'")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("ToDo list")
	})
	mux.HandleFunc("GET /list", checkAuth(getToDoList))
	mux.HandleFunc("POST /task", checkAuth(postTask))
	mux.HandleFunc("PUT /task", checkAuth(editTask))
	mux.HandleFunc("DELETE /task", checkAuth(deleteTask))
	err2 := http.ListenAndServe(":8081", mux)
	if err2 != nil {
		fmt.Println("Error happened", err2.Error())
		return
	}
}

type Authorisation struct {
	UserName string
	Password string
}

var User1 = Authorisation{
	UserName: "Mona",
	Password: "42",
}
var User2 = Authorisation{
	UserName: "Liza",
	Password: "315",
}

func checkAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if (username != User1.UserName || password != User1.Password) && (username != User2.UserName || password != User2.Password) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

type TaskManager struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}

var tasks = []TaskManager{
	{ID: 1, Description: "Open computer"},
	{ID: 2, Description: "Do homework"},
	{ID: 3, Description: "Close computer"},
}

func setCache(key string, value string, expiration time.Duration) error {
	err := rdb.Set(ctx, key, value, expiration).Err()
	if err != nil {
		log.Printf("Error setting cache for key %s: %v", key, err)
		return err
	}
	log.Printf("Successfully set cache for key %s, %s", key, value)
	return nil
}
func readCache(key string) (string, error) {
	value, err := rdb.Get(ctx, key).Result()
	if err != nil {
		log.Printf("Error reading cache for key %s: %v", key, err)
		return "", err
	}
	log.Printf("Cache for key %s: %s", key, value)
	return value, nil
}
func getToDoList(w http.ResponseWriter, r *http.Request) {
	cachedTasks, err := rdb.Get(ctx, "tasks").Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cachedTasks))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(tasks)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tasksJSON, err := json.Marshal(tasks)
	if err == nil {
		setCache("tasks", string(tasksJSON), time.Minute*10)
	}
}

func postTask(w http.ResponseWriter, r *http.Request) {
	var newTask TaskManager
	err := json.NewDecoder(r.Body).Decode(&newTask)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	tasks = append(tasks, newTask)
	err = json.NewEncoder(w).Encode(newTask)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tasksJSON, err := json.Marshal(tasks)
	if err == nil {
		setCache("tasks", string(tasksJSON), time.Minute*10)
	}
}

func editTask(w http.ResponseWriter, r *http.Request) {
	var updatedTask TaskManager
	err := json.NewDecoder(r.Body).Decode(&updatedTask)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for i, task := range tasks {
		if task.ID == updatedTask.ID {
			tasks[i].Description = updatedTask.Description
			break
		}
	}
	err = json.NewEncoder(w).Encode(updatedTask)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tasksJSON, err := json.Marshal(tasks)
	if err == nil {
		setCache("tasks", string(tasksJSON), time.Minute*10)
	}
}
func deleteTask(w http.ResponseWriter, r *http.Request) {
	var taskToDelete TaskManager
	err := json.NewDecoder(r.Body).Decode(&taskToDelete)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	for i, task := range tasks {
		if task.ID == taskToDelete.ID {
			tasks = append(tasks[:i], tasks[i+1:]...)
			break
		}
	}
	err = json.NewEncoder(w).Encode(taskToDelete)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tasksJSON, err := json.Marshal(tasks)
	if err == nil {
		setCache("tasks", string(tasksJSON), time.Minute*10)
	}
}
