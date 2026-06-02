package storage

type TaskList struct {
	List []Task
}

func CreateTaskList() *TaskList {
	return &TaskList{}
}

func (t *TaskList) AddTask(task Task) {
	t.List = append(t.List, task)
}

func (t *TaskList) RemoveTask(id int) {
	tempList := CreateTaskList()
	for _, task := range t.List {
		if task.ID != id {
			tempList.List = append(tempList.List, task)
		}
	}
	t.List = tempList.List
}
