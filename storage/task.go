package storage

import "time"

type Task struct {
	ID          int       `json:"id"`
	User_id     string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	AlarmTime   time.Time `json:"alarmtime"`
	IsCompleted bool      `json:"iscompleted"`
}

type TaskID struct {
	ID int `json:"id"`
}

func CreateTask() *Task {
	return &Task{}
}

func CreateTaskID() *TaskID {
	return &TaskID{}
}

func (task *Task) AddTask(n string, d string, t time.Time, u string) {
	task.Name = n
	task.Description = d
	task.AlarmTime = t
	task.IsCompleted = false
	task.User_id = u
}

func (task *TaskID) AddID(id int) {
	task.ID = id
}
