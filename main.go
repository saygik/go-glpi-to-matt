package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/joho/godotenv"
	"github.com/saygik/go-glpi-to-matt/db"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strconv"
)

var log = logrus.New()

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("err loading settings from env: %v", err)
	}

	src, err := os.OpenFile("app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("err", err)
		log.Fatal("Error to open log file: " + err.Error())
	}
	log.Out = src
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.Formatter = customFormatter
	log.Info("-")
	lastid, err := os.ReadFile("id.id")
	if err != nil {
		log.Warn("Error loading id.id file or wrong file, please create one in the root directory: " + err.Error())
		lastid = []byte("0")
	}
	id, _ := strconv.Atoi(string(lastid))
	fmt.Printf("Number is %d", id)
	db.Init(fmt.Sprintf("%s:%s@tcp(%s)/%s", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_SERVER"), os.Getenv("DB_NAME")))
	defer db.CloseDB()
	tickets, err := db.Tickets(id)
	if err != nil {
		log.Fatal("Error selecting tickets from db: " + err.Error())
	}
	if len(tickets) == 0 {
		log.Warn("No tickets")
		os.Exit(0)
	}

	for _, ticket := range tickets {
		err := sendToMattermost(ticket)
		if err != nil {
			log.Warn("Error sending ticket " + ticket.Id)
		}
		log.Info("Sended ticket " + ticket.Id)
	}

	lastTicketId, err := strconv.Atoi(tickets[len(tickets)-1].Id)
	if err == nil {
		f, err := os.Create("id.id")
		if err != nil {
			log.Errorf("Невозможно создать файл для записи последнего id  объекта GLPI")
		}
		defer func(f *os.File) {
			err := f.Close()
			if err != nil {

			}
		}(f)
		_, _ = f.WriteString(strconv.Itoa(lastTicketId))
	}

}

type Field struct {
	Short string `json:"short"`
	Title string `json:"title"`
	Value string `json:"value"`
}
type Attachment struct {
	Fallback   string  `json:"fallback"`
	Color      string  `json:"color"`
	AuthorName string  `json:"author_name"`
	Title      string  `json:"title"`
	TitleLink  string  `json:"title_link"`
	Text       string  `json:"text"`
	ThumbUrl   string  `json:"thumb_url"`
	Footer     string  `json:"footer"`
	Fields     []Field `json:"fields"`
}
type Message struct {
	IconUrl     string       `json:"icon_url"`
	Username    string       `json:"username"`
	Attachments []Attachment `json:"attachments"`
}

func sendToMattermost(ticket db.Ticket) error {
	url := os.Getenv("MATT_URL")
	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())
	converter.Use(plugin.Table())
	content, err := converter.ConvertString(ticket.Content)
	content, err = converter.ConvertString(content)
	//content, err = converter.ConvertString(content)
	if err != nil {
		content = ""
	}
	fields := []Field{{Short: "true", Title: "влияние", Value: "среднее"}, {Short: "true", Title: "статус", Value: ticket.Status}}
	color := colorByStatus(ticket.Status)
	attachments := []Attachment{{
		Fields:     fields,
		AuthorName: ticket.Org,
		Color:      color,
		Title:      "ОТКАЗ: " + ticket.Name,
		TitleLink:  "https://grafana.rw/d/MePJcn3nk/kartochka-otkaza?orgId=1&var-idz=" + ticket.Id,
		Text:       "**КАТЕГОРИЯ ТЯЖЕСТИ ПОСЛЕДСТВИЙ ОТКАЗА**: " + ticket.Kat + "\n*ОПИСАНИЕ*: " + content,
		ThumbUrl:   "https://support.rw/pics/glpi_project_logo.png",
		Footer:     fmt.Sprintf(`%s , ID: %s , Автор: %s`, ticket.Date, ticket.Id, ticket.Author),
	}}
	message := Message{IconUrl: "https://support.rw/pics/favicon.ico", Username: "GLPI", Attachments: attachments}
	jsonValue, _ := json.Marshal(message)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)
	return nil
}
func colorByStatus(status string) string {
	switch status {
	case "новый":
		return "#FF4059"
	case "в работе (назначен)":
		return "#FF4059"
	case "в работе (запланирован)":
		return "#FF4059"
	case "ожидающий":
		return "#FF4059"
	case "решен":
		return "#bbbe6d"
	case "закрыт":
		return "#343a40"
	default:
		return "#FF4059"
	}
}
