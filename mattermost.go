package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"strconv"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/saygik/go-glpi-to-matt/models"
	mattermost "github.com/saygik/mattermost/client"
	"golang.org/x/exp/slices"
)

type MattermostChannelConf struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Tip   string `json:"tip"`
	Org   string `json:"org"`
	Level int    `json:"ticket-category-level"`
}

type MattChannnelsConfig struct {
	Channels []MattermostChannelConf
}

var mattChannelsConfig MattChannnelsConfig

type MattermostPost struct {
	Id           string        `json:"id"`
	ChannelID    string        `json:"channel-id"`
	Filepath     string        `json:"filepath"`
	Ticket       models.Ticket `json:"ticket"`
	LastComment  int           `json:"last_comment"`
	LastSolution int           `json:"last_solution"`
}

func FindChannelConfigByKey(key string) *MattermostChannelConf {
	item := MattermostChannelConf{}
	idx := slices.IndexFunc(mattChannelsConfig.Channels, func(c MattermostChannelConf) bool { return c.Key == key })
	if idx == -1 {
		return nil
	}
	item = mattChannelsConfig.Channels[idx]
	return &item

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
func savePostToFile(filename string, post MattermostPost) error {
	jsonMattermostPost := MattermostPostToJSON(post)
	return saveToFile(filename, jsonMattermostPost)
}

func MattermostPostMsgTextFromTicket(ticket models.Ticket, isIssue bool) string {
	ticketTitle := ""
	link := ""
	icon := ""
	if isIssue {
		icon = "###### :issue: "
		link = "https://grafana.rw/d/MePJcn3nk/kartochka-otkaza?orgId=1&var-idz=" + ticket.Id
		ticketTitle = "ОТКАЗ: " + ticket.Name
		if ticket.KatId == "0" || ticket.KatId == "-" {
			ticketTitle = ticketTitle + " (категория отказа отозвана)"
		}
	} else {
		icon = "###### :work: "
		link = "https://support.rw/front/change.form.php?id=" + ticket.Id
		ticketTitle = "РАБОТЫ: " + ticket.Name
	}
	content := ConvertToMarkdown(ticket.Content)
	content = strings.ReplaceAll(content, "\n", "\n>")
	message := icon + "**[" + ticketTitle + "](" + link + ")**\n" +
		"  >" + content

	return message
}
func ConvertToMarkdown(text string) string {
	var err error
	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())
	converter.Use(plugin.Table())
	content, _ := converter.ConvertString(text)
	content, err = converter.ConvertString(content)
	//content, err = converter.ConvertString(content)
	if err != nil {
		content = ""
	}

	return content
}

func MattermostPostMsgPropertieFromTicket(ticket models.Ticket) (mattermost.MsgProperties, error) {
	//content := ConvertToMarkdown(ticket.Content)
	//	fields := []Field{{Short: "true", Title: "влияние", Value: "среднее"}, {Short: "true", Title: "статус", Value: ticket.Status}}
	//color := colorByStatus(ticket.Status)
	user, usererr := getAduser(ticket.AuthorName)
	userProps := ""

	if usererr == nil {
		telephone := ""
		if user.Telephone != "" {
			telephone = fmt.Sprintf(`, тел.%s`, user.Telephone)
		}
		userProps = fmt.Sprintf(`(%s %s%s)`, user.Title, user.Department, telephone)
		if len(userProps) < 6 {
			userProps = ""
		}
	}

	mLevel := GetMessageLevelByStatus(ticket.Status)

	//	fields := []mattermost.MsgAttachmentField{{Short: "false", Title: "Влияние", Value: ticket.Impact}, {Short: "false", Title: "Статус", Value: ticket.Status}}
	msgProperties := mattermost.MsgProperties{
		Attachments: []mattermost.MsgAttachment{
			{
				//				Author:    ticket.Org,
				Color: mattermost.GetAttachmentColor(mLevel), //		"critical", "info", "success", "warning"
				//Title: ticketTitle,
				//				TitleLink: "https://grafana.rw/d/MePJcn3nk/kartochka-otkaza?orgId=1&var-idz=" + ticket.Id,
				Text: "**" + ticket.Org + "**" +
					"\n:list: `Категория    :` `" + ticket.Kat + "`" +
					"\n" + GetIconByStatus(ticket.Status) + " `Статус       :` `" + ticket.Status + "`" +
					"\n:user: `Автор        :` `" + ticket.Author + " " + userProps + "`" +
					"\n" +
					"\n:clock-g: `регистрация  :` `" + parseGlpiDate(ticket.DateCreation) + "`" +
					"\n:clock-r: `возникновение:` `" + parseGlpiDate(ticket.Date) + "`" +
					"\n:clock-m: `устранение   :` `" + parseGlpiDate(ticket.SolveDate) + "`",
				Footer:   fmt.Sprintf(`Изменено: %s , ID: %s `, parseGlpiDate(ticket.DateMod), ticket.Id),
				ThumbUrl: "https://support.rw/pics/glpi_project_logo.png",
				//				Fields:    fields,
			}}}
	return msgProperties, nil
}
func MattermostPostMsgPropertieFromChange(ticket models.Ticket) (mattermost.MsgProperties, error) {
	//content := ConvertToMarkdown(ticket.Content)
	//	fields := []Field{{Short: "true", Title: "влияние", Value: "среднее"}, {Short: "true", Title: "статус", Value: ticket.Status}}
	//	color := colorByStatus(ticket.Status)
	user, usererr := getAduser(ticket.AuthorName)
	userProps := ""
	if usererr == nil {
		telephone := ""
		if user.Telephone != "" {
			telephone = fmt.Sprintf(`, тел.%s`, user.Telephone)
		}
		userProps = fmt.Sprintf(`(%s %s%s)`, user.Title, user.Department, telephone)
		if len(userProps) < 6 {
			userProps = ""
		}
	}
	mLevel := GetMessageLevelByStatus(ticket.Status)
	//	fields := []mattermost.MsgAttachmentField{{Short: "false", Title: "Влияние", Value: ticket.Impact}, {Short: "false", Title: "Статус", Value: ticket.Status}}
	msgProperties := mattermost.MsgProperties{
		Attachments: []mattermost.MsgAttachment{
			{
				//				Author:    ticket.Org,
				Color:     mattermost.GetAttachmentColor(mLevel), //		"critical", "info", "success", "warning"
				TitleLink: "https://support.rw/front/change.form.php?id=" + ticket.Id,
				Text: ":soft: `Система        :` `" + ticket.Kat + "`" +
					"\n" + GetIconByStatus(ticket.Status) + " `Статус         :` `" + ticket.Status + "`" +
					"\n:user: `Автор          :` `" + ticket.Author + " " + userProps + "`" +
					"\n" +
					"\n:clock-g: `регистрация    :` `" + parseGlpiDate(ticket.DateCreation) + "`" +
					"\n:clock-r: `начало работ   :` `" + parseGlpiDate(ticket.Date) + "`" +
					"\n:clock-m: `окончание работ:` `" + parseGlpiDate(ticket.SolveDate) + "`",
				Footer:   fmt.Sprintf(`Изменено: %s , ID: %s `, parseGlpiDate(ticket.DateMod), ticket.Id),
				ThumbUrl: "https://support.rw/pics/glpi_project_logo.png",
				//				Fields:    fields,
			}}}
	return msgProperties, nil
}
func sendMessageToMattermost(channelID, message, rootId string) (postId string, err error) {

	createdPost, err := MattermostModel.CreateSimplePost(channelID, message, rootId)
	if err != nil {
		return "", err
	}
	return createdPost.Id, nil
}
func mattermostPriorityFromTicket(ticket models.Ticket) mattermost.MsgMetadata {
	priority := "standart" //The priority filed should probably only accept the values of standard, important, and urgent (and blank).
	kat, err := strconv.Atoi(ticket.KatId)
	if err != nil {
		return mattermost.MsgMetadata{Priority: mattermost.MsgPriority{Priority: priority, RequestedAck: false}}
	}
	if kat > 8 {
		priority = "important"
	}
	return mattermost.MsgMetadata{Priority: mattermost.MsgPriority{Priority: priority, RequestedAck: false}}
}
func sendTicketToMattermost(channel *MattermostChannelConf, ticket models.Ticket) (postId string, err error) {

	message := MattermostPostMsgTextFromTicket(ticket, true)
	msgProperties, err := MattermostPostMsgPropertieFromTicket(ticket)
	if err != nil {
		msgProperties = mattermost.MsgProperties{}
	}

	msgMetadata := mattermostPriorityFromTicket(ticket)
	createdPost, err := MattermostModel.CreatePostWithAttachtent(channel.Key, message, "", msgProperties, msgMetadata)
	if err != nil {
		log.Warn("Error sending ticket " + ticket.Id + " to channel " + channel.Name + ":" + err.Error())
	} else {
		log.Info("Sended ticket " + ticket.Id + " to channel" + channel.Name)

	}
	return createdPost.Id, nil
}
func sendChangeToMattermost(channel *MattermostChannelConf, ticket models.Ticket) (postId string, err error) {

	message := MattermostPostMsgTextFromTicket(ticket, false)
	msgProperties, err := MattermostPostMsgPropertieFromChange(ticket)
	if err != nil {
		msgProperties = mattermost.MsgProperties{}
	}
	priority := "standart" //The priority filed should probably only accept the values of standard, important, and urgent (and blank).
	msgMetadata := mattermost.MsgMetadata{Priority: mattermost.MsgPriority{Priority: priority, RequestedAck: false}}

	createdPost, err := MattermostModel.CreatePostWithAttachtent(channel.Key, message, "", msgProperties, msgMetadata)
	if err != nil {
		log.Warn("Error sending ticket " + ticket.Id + " to channel " + channel.Name + ":" + err.Error())
	} else {
		log.Info("Sended ticket " + ticket.Id + " to channel" + channel.Name)

	}
	return createdPost.Id, nil
}

func updatePost(postId string, message string) error {
	_, err := MattermostModel.UpdatePost(postId, message)
	if err != nil {
		return err
	}
	return nil
}
func updateTicketInMattermost(postId string, ticket models.Ticket) error {

	message := MattermostPostMsgTextFromTicket(ticket, true)
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
func updateChangeInMattermost(postId string, ticket models.Ticket) error {

	message := MattermostPostMsgTextFromTicket(ticket, false)
	msgProperties, err := MattermostPostMsgPropertieFromChange(ticket)
	if err != nil {
		msgProperties = mattermost.MsgProperties{}
	}
	_, err = MattermostModel.UpdatePostWithAttachtent(postId, message, msgProperties)
	if err != nil {
		return err
	}
	return nil
}

type User struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Company    string `json:"company"`
	Department string `json:"department"`
	Title      string `json:"title"`
	Mail       string `json:"mail"`
	Telephone  string `json:"telephone"`
}

func getAduser(upn string) (*User, error) {
	url := "https://userinfoapi.brnv.rw/api/ad/finduser/" + upn
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user User
	err = json.Unmarshal(body, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func parseGlpiDate(sDate string) string {
	t, err := time.Parse("2006-01-02 15:04:05", sDate)
	if err != nil {
		return sDate
	}
	return t.Format("02.01.2006 15:04:05")
}
