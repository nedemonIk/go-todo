package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	_ "github.com/go-sql-driver/mysql"
	"github.com/thedevsaddam/renderer"
)

var db *sql.DB
var rnd *renderer.Render

const (
	dsn        = "nedemonIk:mysqltop_123@tcp(localhost:3306)/tododb"
	tableName  = "tasks"
	listenAddr = ":9000"
)

type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

func init() {
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	rnd = renderer.New()
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, err)
		return
	}
}

func createTodo(w http.ResponseWriter, r *http.Request) {
	var t Todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusBadRequest, err)
		return
	}

	// Simple validation
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "The title field is required"})
		return
	}

	insertSQL := "INSERT INTO " + tableName + " (title, completed, created_at) VALUES (?, ?, NOW())"
	result, err := db.Exec(insertSQL, t.Title, t.Completed)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, err)
		return
	}

	insertID, _ := result.LastInsertId()

	rnd.JSON(w, http.StatusCreated, renderer.M{"message": "Todo created successfully", "todo_id": insertID})
}

func updateTodo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t Todo
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		rnd.JSON(w, http.StatusBadRequest, err)
		return
	}

	// Simple validation
	if t.Title == "" {
		rnd.JSON(w, http.StatusBadRequest, renderer.M{"message": "The title field is required"})
		return
	}

	updateSQL := "UPDATE " + tableName + " SET title = ?, completed = ? WHERE id = ?"
	_, err := db.Exec(updateSQL, t.Title, t.Completed, id)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, err)
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"message": "Todo updated successfully"})
}

func fetchTodos(w http.ResponseWriter, r *http.Request) {
	selectSQL := "SELECT id, title, completed, created_at FROM " + tableName
	rows, err := db.Query(selectSQL)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	todoList := []Todo{}
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Completed, &t.CreatedAt); err != nil {
			rnd.JSON(w, http.StatusInternalServerError, err)
			return
		}
		todoList = append(todoList, t)
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"data": todoList})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	deleteSQL := "DELETE FROM " + tableName + " WHERE id = ?"
	_, err := db.Exec(deleteSQL, id)
	if err != nil {
		rnd.JSON(w, http.StatusInternalServerError, err)
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{"message": "Todo deleted successfully"})
}

func initMysqlDB() error {
	dsn := "nedemonIk:mysqltop_123@tcp(localhost:3306)/tododb"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	err = db.Ping()
	if err != nil {
		return err
	}
	// Глобальная переменная db теперь будет доступна для использования в вашем коде.
	return nil
}

func main() {
	stopChan := make(chan os.Signal)
	signal.Notify(stopChan, os.Interrupt)

	if err := initMysqlDB(); err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Get("/", homeHandler)
	r.Route("/todo", func(r chi.Router) {
		r.Get("/", fetchTodos)
		r.Post("/", createTodo)
		r.Put("/{id}", updateTodo)
		r.Delete("/{id}", deleteTodo)
	})

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Println("Listening on port", listenAddr)
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("listen: %s\n", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	} else {
		log.Println("Server gracefully stopped!")
	}
}
