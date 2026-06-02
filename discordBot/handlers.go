package discordbot

import (
	"context"
	"discord/handlers"
	"discord/storage"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	DB *pgx.Conn
}

var (
	Commands = []*discordgo.ApplicationCommand{
		{
			Name:        "add-task",
			Description: "Добавляет новую задачу",
		},
		{
			Name:        "delete-task",
			Description: "Удаляет задачу",
		},
		{
			Name:        "show-task",
			Description: "Показывает все активные задачи",
		},
	}

	CommandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"add-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "add_task_modal",
					Title:    "Добавление новой напоминалки",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.TextInput{
									CustomID:    "task_name",
									Label:       "Название задачи",
									Placeholder: "Например: Сделать уроки",
									Style:       discordgo.TextInputShort,
									Required:    true,
								},
							},
						},
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.TextInput{
									CustomID:    "task_description",
									Label:       "Описание задачи",
									Placeholder: "Например: Сделать домашку по математике на завтра, упражнение №10",
									Style:       discordgo.TextInputParagraph,
									Required:    false,
								},
							},
						},
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.TextInput{
									CustomID:    "task_alarm",
									Label:       "Когда оповестить",
									Placeholder: "Укажи день, месяц, год, и время в формате HH:MM", // Через ИИ переделай в YYYY-MM-DDTHH:MM:SSZ
									Style:       discordgo.TextInputShort,
									Required:    true,
								},
							},
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}
		},
		"show-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			client := &http.Client{}
			req, err := http.NewRequest("GET", "http://localhost:8080/show?user_id="+i.Member.User.ID, nil)
			if err != nil {
				fmt.Println("Error while trying to create request:", err)
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Error while trying to do request:", err)
				return
			}
			b, err := io.ReadAll(resp.Body)
			list := storage.CreateTaskList()
			json.Unmarshal(b, &list)
			var msg string = `Список дел ` + i.Member.User.Username + "\n-------------------------------------\n"
			if len(list.List) == 0 {
				msg += "**Список пуст! Добавь новую задачу :)**"
			} else {
				for i := 0; i < len(list.List); i++ {
					if !list.List[i].IsCompleted {
						timeData := list.List[i].AlarmTime
						msg += "**ID: " + strconv.Itoa(list.List[i].ID) + "** - " + list.List[i].Name + " (:alarm_clock: " + timeData.Format("02-01-2006 15:04") + ")" + "\n*" + list.List[i].Description + "*\n"
						if i != len(list.List)-1 {
							msg += "\n"
						}
					}
				}
			}
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: msg,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				panic(err)
			}
		},
		"delete-task": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseModal,
				Data: &discordgo.InteractionResponseData{
					CustomID: "delete-task",
					Title:    "Удаление напоминалки",
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.TextInput{
									CustomID:    "task-id",
									Label:       "ID задачи для удаления",
									Placeholder: "Максимум 1 номер, например 1, 2, 4...",
									Style:       discordgo.TextInputShort,
									Required:    true,
								},
							},
						},
					},
				},
			})
			if err != nil {
				panic(err)
			}
		},
	}
)

func AddTask(name string, description string, time time.Time, id string, d *handlers.Database) {
	task := storage.CreateTask()
	_ = task

	conn := d.DB
	ctx := context.Background()

	task.AddTask(name, description, time, "")

	sql := `INSERT INTO task_list(name, user_id, description, alarmtime, iscompleted)
	VALUES ($1, $2, $3, $4, $5)`

	fmt.Println(task.Name, id, task.Description, task.AlarmTime, task.IsCompleted)
	_, err := conn.Exec(ctx, sql, task.Name, id, task.Description, task.AlarmTime, task.IsCompleted)
	if err != nil {
		fmt.Println("Bad request")
		return
	}

	fmt.Println("Я успешно добавил!")
}

func DeleteTask(id int, conn *pgxpool.Pool) {
	time.Sleep(1 * time.Second)
	sql := `DELETE FROM task_list WHERE id=$1`
	_, err := conn.Exec(context.Background(), sql, id)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
}

func NotifyUser(s *discordgo.Session, user_id string, task_id int, d *handlers.Database) {
	var name, description string

	conn := d.DB
	fmt.Println(task_id, user_id)

	sql := "SELECT name, description FROM task_list WHERE id=$1"
	row := conn.QueryRow(context.Background(), sql, task_id)
	row.Scan(&name, &description)

	ch, err := s.UserChannelCreate(user_id)
	if err != nil {
		fmt.Println("Error while trying to create a private message channel")
	}
	s.ChannelMessageSend(ch.ID, "Напоминаю тебе про \""+name+"\" ("+description+")")
	DeleteTask(task_id, conn)
}
