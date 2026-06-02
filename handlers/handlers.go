package handlers

import (
	"context"
	"discord/storage"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	DB *pgxpool.Pool
}

func NewDB(db *pgxpool.Pool) *Database {
	return &Database{DB: db}
}

/*
В теле запроса нам передают все необходимые поля, для создания задачи
*/
func (d *Database) HandleAddTask(w http.ResponseWriter, r *http.Request) {
	conn := d.DB
	ctx := context.Background()

	body_byte, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error, can't read body", http.StatusInternalServerError)
		return
	}

	task := storage.CreateTask()
	json.Unmarshal(body_byte, task)

	sql := `INSERT INTO task_list(name, user_id, description, alarmtime, iscompleted)
	VALUES ($1, $2, $3, $4, $5)`
	_, err = conn.Exec(ctx, sql, task.Name, task.User_id, task.Description, task.AlarmTime, task.IsCompleted)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	task_byte, err := json.MarshalIndent(task, "", "    ")
	if err != nil {
		http.Error(w, "Can't convert task to byte..", http.StatusInternalServerError)
		return
	}
	w.Write(task_byte)
}

func (d *Database) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	conn := d.DB
	ctx := context.Background()

	task_id := r.URL.Query().Get("id")
	if task_id == "" {
		http.Error(w, "id can't be empty", http.StatusBadRequest)
	}

	sql := `DELETE FROM task_list WHERE id=$1`
	_, err := conn.Exec(ctx, sql, task_id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Удалил!"))
}

func (d *Database) HandleShowTasks(w http.ResponseWriter, r *http.Request) {

	ctx := context.Background()
	conn := d.DB
	task := storage.CreateTask()
	list := storage.CreateTaskList()
	sql := ""
	var rows pgx.Rows
	var err error

	id := r.URL.Query().Get("id")
	user_id := r.URL.Query().Get("user_id")
	if id == "" {
		sql = `SELECT * FROM task_list WHERE user_id=$1`
		rows, err = conn.Query(ctx, sql, user_id)
	} else {
		sql = `SELECT * FROM task_list WHERE id=$1 AND user_id=$2`
		rows, err = conn.Query(ctx, sql, id, user_id)
	}
	if err != nil {
		fmt.Println(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		rows.Scan(&task.ID, &task.User_id, &task.Name, &task.Description, &task.AlarmTime, &task.IsCompleted)
		list.AddTask(*task)
	}

	resp_body, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		fmt.Println("Error while trying to json marshal")
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp_body)
}

func (d Database) HandleCompleteTask(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	conn := d.DB
	task := storage.CreateTask()
	var status bool

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id can't be empty", http.StatusBadRequest)
	}

	sql := `SELECT iscompleted FROM task_list WHERE id=$1`
	conn.QueryRow(ctx, sql, id).Scan(&status)

	if !status {
		sql = `UPDATE task_list SET iscompleted = true WHERE id=$1`
	} else {
		sql = `UPDATE task_list SET iscompleted = false WHERE id=$1`
	}
	conn.Exec(ctx, sql, id)

	sql = `SELECT * FROM task_list WHERE id=$1`
	conn.QueryRow(ctx, sql, id).Scan(&task.ID, &task.User_id, &task.Name, &task.Description, &task.AlarmTime, &task.IsCompleted)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(task.Name + " теперь " + strconv.FormatBool(task.IsCompleted)))
}
