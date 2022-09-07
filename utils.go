package main

import (
	"encoding/json"
	"fmt"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/saygik/go-glpi-to-matt/models"
	mattermost "github.com/saygik/mattermost/client"
	"os"
	"path/filepath"
	"strconv"
)

type MattermostPost struct {
	Id           string        `json:"id"`
	Filepath     string        `json:"filepath"`
	Ticket       models.Ticket `json:"ticket"`
	LastComment  int           `json:"last_comment"`
	LastSolution int           `json:"last_solution"`
}

func StringToInt(value string) int {
	newValue, err := strconv.Atoi(value)
	if err != nil {
		newValue = 0
	}
	return newValue
}
func MattermostPostToJSON(post MattermostPost) string {
	b, _ := json.Marshal(post)
	return string(b)
}
func MattermostPostFromJSON(text []byte) (MattermostPost, error) {
	data := MattermostPost{}
	err := json.Unmarshal([]byte(text), &data)
	if err != nil {
		return data, err
	}
	return data, nil
}
func savePostToFile(post MattermostPost) error {
	jsonMattermostPost := MattermostPostToJSON(post)
	return saveToFile(post.Id+".conf", jsonMattermostPost)
}

func saveToFile(id, body string) error {
	f, err := os.Create(id)
	if err != nil {
		log.Errorf("Невозможно создать файл для записи последнего id  объекта GLPI")
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)
	_, _ = f.WriteString(body)
	return nil
}
func WalkFiles(root, pattern string) ([]string, error) {
	var matches []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}
func MattermostPostMsgTextFromTicket(ticket models.Ticket) string {
	message := "**" + ticket.Org + "**\n" + ticket.DateMod
	return message
}
func ConvertToMarkdown(text string) string {
	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())
	converter.Use(plugin.Table())
	content, err := converter.ConvertString(text)
	content, err = converter.ConvertString(content)
	//content, err = converter.ConvertString(content)
	if err != nil {
		content = ""
	}

	return content
}

func MattermostPostMsgPropertieFromTicket(ticket models.Ticket) (mattermost.MsgProperties, error) {
	content := ConvertToMarkdown(ticket.Content)
	//	fields := []Field{{Short: "true", Title: "влияние", Value: "среднее"}, {Short: "true", Title: "статус", Value: ticket.Status}}
	//	color := colorByStatus(ticket.Status)
	mLevel := GetMessageLevelByStatus(ticket.Status)
	//	fields := []mattermost.MsgAttachmentField{{Short: "false", Title: "Влияние", Value: ticket.Impact}, {Short: "false", Title: "Статус", Value: ticket.Status}}
	msgProperties := mattermost.MsgProperties{
		[]mattermost.MsgAttachment{
			{
				//				Author:    ticket.Org,
				Color:     mattermost.GetAttachmentColor(mLevel), //		"critical", "info", "success", "warning"
				Title:     "ОТКАЗ: " + ticket.Name,
				TitleLink: "https://grafana.rw/d/MePJcn3nk/kartochka-otkaza?orgId=1&var-idz=" + ticket.Id,
				Text: "`КАТЕГОРИЯ:` " + ticket.Kat +
					"\n`ОПИСАНИЕ:` " + content +
					"\n `Автор:` " + ticket.Author +
					"\n `Статус:` " + ticket.Status,
				Footer:   fmt.Sprintf(`%s , ID: %s `, ticket.Date, ticket.Id),
				ThumbUrl: "https://support.rw/pics/glpi_project_logo.png",
				//				Fields:    fields,
			}}}
	return msgProperties, nil
}
func sendMessageToMattermost(message, rootId string) (postId string, err error) {
	createdPost, err := MattermostModel.CreateSimplePost(message, rootId)
	if err != nil {
		return "", err
	}
	return createdPost.Id, nil
}
func sendTicketToMattermost(ticket models.Ticket) (postId string, err error) {

	message := MattermostPostMsgTextFromTicket(ticket)
	msgProperties, err := MattermostPostMsgPropertieFromTicket(ticket)
	if err != nil {
		msgProperties = mattermost.MsgProperties{}
	}

	createdPost, err := MattermostModel.CreatePostWithAttachtent(message, "", msgProperties)
	if err != nil {
		return "", err
	}
	return createdPost.Id, nil
}
func updateTicketInMattermost(postId string, ticket models.Ticket) error {

	message := MattermostPostMsgTextFromTicket(ticket)
	msgProperties, err := MattermostPostMsgPropertieFromTicket(ticket)
	if err != nil {
		msgProperties = mattermost.MsgProperties{}
	}
	_, err = MattermostModel.UpdatePostWithAttachtent(postId, message, msgProperties)
	if err != nil {
		return err
	}
	return nil
}

const (
	statusCritical = "critical"
	statusInfo     = "info"
	statusSuccess  = "success"
	statusWarning  = "warning"
)

func GetMessageLevelByStatus(status string) string {
	var color = map[string]string{
		"новый":                   statusCritical,
		"в работе (назначен)":     statusCritical,
		"в работе (запланирован)": statusWarning,
		"ожидающий":               statusWarning,
		"решен":                   statusSuccess,
		"закрыт":                  statusInfo,
	}

	if c, found := color[status]; found {
		return c
	}

	return statusInfo
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
