package main

import (
	"bytes"
	"context"
	discordbot "discord/discordBot"
	"discord/handlers"
	"discord/storage"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type Database struct {
	DB *pgx.Conn
}

type Ai_Json struct {
	Model       string  `json:"model"`
	Messages    []Msgs  `json:"messages"`
	Temperature float64 `json:"temperature"`
	Max_tokens  int     `json:"max_tokens"`
}

type Msgs struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AiResponse struct {
	Choices []Choices `json:"choices"`
}

type Choices struct {
	Message Msgs `json:"message"`
}

type ModalResponseJson struct {
	Components []ModalResponse `json:"components"`
}

type ModalResponse struct {
	Value     string `json:"value"`
	Custom_ID string `json:"custom_id"`
}

func main() {
	if err := godotenv.Load("data.env"); err != nil {
		fmt.Println("Error loading the data.env file")
	}
	db_address := os.Getenv("DATABASE_CONNECTION")
	client_id := os.Getenv("CLIENT_ID")
	base_url := os.Getenv("BASE_URL")
	openrouter_ai_api := os.Getenv("AI_API_KEY")
	conn, err := pgxpool.New(context.Background(), db_address)
	if err != nil {
		fmt.Println("Error while trying to connect db")
	}

	db := handlers.NewDB(conn)

	defer conn.Close()

	sql := `CREATE TABLE IF NOT EXISTS task_list(
	id SERIAL PRIMARY KEY,
	user_id VARCHAR(50) NOT NULL,
	name VARCHAR(100) NOT NULL,
	description VARCHAR(500) NOT NULL,
	alarmtime TIMESTAMP NOT NULL,
	iscompleted BOOL NOT NULL
	)`

	_, err = conn.Exec(context.Background(), sql)
	if err != nil {
		fmt.Println(err)
	}

	bot_token := os.Getenv("BOT_TOKEN")
	session, err := discordgo.New("Bot " + bot_token)
	if err != nil {
		fmt.Println("Error creating new discord session")
		return
	}

	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Bot is up!")
	})

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		fmt.Println("type:", i.Type)
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			if h, ok := discordbot.CommandHandlers[i.ApplicationCommandData().Name]; ok {
				h(s, i)
			}
		case discordgo.InteractionModalSubmit:
			msgs := AiResponse{}
			question_p1 := `Сейчас я тебе отправлю время, которое мне отправил пользователь. Оно может быть в каком угодно формате, но
			твоя главная задача переделать его в формат YYYY-MM-DD HH:MM:SS
			Ну например, тебе дадут 01.02.1980 12:00, тебе надо будет вернуть 1980-02-01 12:00:00. Также пользователь может отправить
			месяц в формате названия, тогда его тоже нужно будет преобразовать в цифру. Если вдруг у тебя не получается преобразовать, так
			как например не хватает критически важных данных, например там.. дня нет, или месяца нет, или года нет, то просто
			возвращай обычную дату 1980-02-01 12:00:00. Твоя задача в ответ мне дать только отформатированную дату, и больше ничего.
			Чисто в одну строчку написать эту дату в нужном мне формате и ничего более
			
			Время, которое дал пользователь:`

			/*
				question_p1 := `Сейчас я тебе отправлю время, которое мне отправил пользователь. Оно может быть в каком угодно формате, но
				твоя главная задача переделать его в формат YYYY-MM-DDTHH:MM:SSZ, где T и Z это просто вот такие разделители.
				Ну например, тебе дадут 01.02.1980 12:00, тебе надо будет вернуть 1980-02-01T12:00:00Z. Также пользователь может отправить
				месяц в формате названия, тогда его тоже нужно будет преобразовать в цифру. Если вдруг у тебя не получается преобразовать, так
				как например не хватает критически важных данных, например там.. дня нет, или месяца нет, или года нет, то просто
				возвращай обычную дату 1980-02-01T12:00:00Z. Твоя задача в ответ мне дать только отформатированную дату, и больше ничего.
				Чисто в одну строчку написать эту дату в нужном мне формате и ничего более

				Время, которое дал пользователь:`
			*/

			var question_p2 string
			if err != nil {
				fmt.Println("Error while trying to create new json from struct")
				return
			}

			data := i.ModalSubmitData()
			switch data.CustomID {
			case "add_task_modal":
				session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Создаю...",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})
				modal_response := ModalResponseJson{}
				modal_response_map := make(map[string]string, 0)

				for i := 0; i < len(data.Components); i++ {
					modal_data, err := data.Components[i].MarshalJSON()
					if err != nil {
						fmt.Println("Error while trying to marshal modal response")
						return
					}

					json.Unmarshal(modal_data, &modal_response)

					modal_response_map[modal_response.Components[0].Custom_ID] = modal_response.Components[0].Value

					if modal_response.Components[0].Custom_ID == "task_alarm" {
						question_p2 = modal_response.Components[0].Value
					}
				}

				question := question_p1 + question_p2
				new_msg_for_body_json_temp := Msgs{Role: "user", Content: question}
				msg_map := make([]Msgs, 0)
				msg_map = append(msg_map, new_msg_for_body_json_temp)
				new_body_json_temp := &Ai_Json{Model: "mistral-large-latest", Messages: msg_map, Temperature: 0.7, Max_tokens: 1000}
				new_body_json, err := json.Marshal(new_body_json_temp)

				client := &http.Client{}
				req, err := http.NewRequest("POST", base_url+"/chat/completions", bytes.NewBuffer(new_body_json))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+openrouter_ai_api)
				if err != nil {
					fmt.Println("Can't create a new request")
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					fmt.Println("Error while trying to do request")
					return
				}
				resp_body_byte, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Println("Error while trying to read from response body")
					return
				}
				json.Unmarshal(resp_body_byte, &msgs)
				formatted_alarm_time_str := msgs.Choices[0].Message.Content

				formatted_alarm_time, err := time.Parse("2006-01-02 15:04:05", formatted_alarm_time_str)

				_ = formatted_alarm_time

				//discordbot.AddTask(modal_response_map["task_name"], modal_response_map["task_description"], formatted_alarm_time, i.Member.User.ID, db)
				task := storage.CreateTask()
				task.AddTask(modal_response_map["task_name"], modal_response_map["task_description"], formatted_alarm_time, i.Member.User.ID)
				add_body, err := json.Marshal(task)
				req, err = http.NewRequest("POST", "http://localhost:8080/add", bytes.NewBuffer(add_body))
				if err != nil {
					fmt.Println("Error while trying to create a request")
					return
				}
				resp, err = client.Do(req)
				if err != nil {
					fmt.Println("Error while trying to do request")
					return
				}
				fmt.Println(resp)
			case "delete-task":

				// действия при удалении
				delete_response_byte, err := data.Components[0].MarshalJSON()
				if err != nil {
					fmt.Println("Error while trying to marshal json")
				}

				modal_response := ModalResponseJson{}

				if err := json.Unmarshal(delete_response_byte, &modal_response); err != nil {
					fmt.Println("Error while trying to unmarshal")
					return
				}

				commandTag, err := conn.Exec(context.Background(), "SELECT * FROM task_list WHERE id=$1 AND user_id=$2", modal_response.Components[0].Value, i.Member.User.ID)
				if err != nil {
					fmt.Println("Error while trying to send SQL request")
					return
				}
				if commandTag.RowsAffected() == 0 {
					fmt.Println("Тут нолик")
					session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "Такого ID нету.",
							Flags:   discordgo.MessageFlagsEphemeral,
						},
					})
					return
				}
				session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Создаю...",
						Flags:   discordgo.MessageFlagsEphemeral,
					},
				})

				client := &http.Client{}
				req, err := http.NewRequest("DELETE", "http://localhost:8080/delete?id="+modal_response.Components[0].Value, nil)
				if err != nil {
					fmt.Println("Error while trying to create a new request")
					return
				}
				_, err = client.Do(req)
				if err != nil {
					fmt.Println("Error while trying to do request")
				}
			}
		}
	})

	err = session.Open()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer session.Close()

	registeredCommands := make([]*discordgo.ApplicationCommand, len(discordbot.Commands))
	for i, v := range discordbot.Commands {
		cmd, err := session.ApplicationCommandCreate(session.State.User.ID, client_id, v)
		if err != nil {
			fmt.Println("Cannot create '%v' comamnd: '%v'", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	r := mux.NewRouter()
	r.HandleFunc("/add", db.HandleAddTask).Methods("POST")           // Для добавлевния новых задач
	r.HandleFunc("/delete", db.HandleDeleteTask).Methods("DELETE")   // Для удаления задачи из списка
	r.HandleFunc("/show", db.HandleShowTasks).Methods("GET")         // "?id= "для номера конкретного, без "?id=" для всех
	r.HandleFunc("/complete", db.HandleCompleteTask).Methods("POST") // Для выполнения/невыполнения задачи
	http.Handle("/", r)

	go http.ListenAndServe(":8080", nil)

	go func() {
		for {
			fmt.Println("Я в цикле")
			var alarmtime time.Time
			var user_id string
			var task_id int
			time.Sleep(10 * time.Second)
			sql := "SELECT alarmtime, user_id, id FROM task_list"
			//alarmtime, err := time.Parse("01.02.2026 12:05:06", alarmtime_str)
			rows, err := conn.Query(context.Background(), sql)
			if err != nil {
				fmt.Println("Error while trying to exec sql request")
				return
			}
			for rows.Next() {
				if err := rows.Scan(&alarmtime, &user_id, &task_id); err != nil {
					fmt.Println("Error while trying to scan SQL row:", err)
					return
				}
				if time.Until(alarmtime) < (4*time.Hour + 10*time.Second) { // для подгонки под UTC+4
					discordbot.NotifyUser(session, user_id, task_id, db)
					fmt.Println("ОПАА")
				} else {
					fmt.Println("Пока нет. Еще...", time.Until(alarmtime.Add(-4*time.Hour)))
				}
			}
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-sc
	fmt.Println("Graceful Shutdown")
}
