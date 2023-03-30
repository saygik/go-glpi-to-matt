package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/saygik/go-glpi-to-matt/db"
	"github.com/saygik/go-glpi-to-matt/models"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()
var GLPIModel = new(models.GLPIModel)
var MattermostModel = new(models.MattermostModel)

func LoadConfiguration(file string) (MattChannnelsConfig, error) {
	cfg := MattChannnelsConfig{}
	configFile, err := os.ReadFile(file)

	if err != nil {
		return cfg, err
	}
	json.Unmarshal(configFile, &cfg.Channels)
	//jsonParser := json.NewDecoder(configFile)
	//jsonParser.Decode(&config)
	return cfg, err
}

func main() {
	exPath, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	fmt.Println(exPath) // for example /home/user

	err = godotenv.Load()
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
	//	log.Info("-")
	lastid, err := os.ReadFile("id.id")
	if err != nil {
		log.Warn("Error loading id.id file or wrong file, please create one in the root directory: " + err.Error())
		lastid = []byte("0")
	}
	id, _ := strconv.Atoi(string(lastid))
	fmt.Printf("Last ticket number is %d", id)
	lastChangeId, err := os.ReadFile("id-changes.id")
	if err != nil {
		log.Warn("Error loading idc.id file or wrong file, please create one in the root directory: " + err.Error())
		lastid = []byte("0")
	}
	idc, _ := strconv.Atoi(string(lastChangeId))
	fmt.Printf("Last ticket number is %d", idc)

	mattChannelsConfig, err = LoadConfiguration("channels.json")
	if err != nil || mattChannelsConfig.Channels == nil {
		log.Fatal("Error loading channels.json file or file is wrong, please create one in the root directory")
	}

	//	db.Init(fmt.Sprintf("%s:%s@tcp(%s)/%s", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_SERVER"), os.Getenv("DB_NAME")))
	db.Init(fmt.Sprintf("%s:%s@tcp(%s)/%s", os.Getenv("GLPI_TO_MATT_DB_USER"), os.Getenv("GLPI_TO_MATT_DB_PASS"), os.Getenv("GLPI_TO_MATT_DB_SERVER"), os.Getenv("GLPI_TO_MATT_DB_NAME")))
	defer db.CloseDB()
	MattermostModel.Init()

	ticketsDir := fmt.Sprintf("%s/tickets/", exPath)
	changesDir := fmt.Sprintf("%s/changes/", exPath)

	enumerateTicketsFromID(id, ticketsDir)
	enumerateChangesFromID(idc, changesDir)

	posts, err := enumeratePostsFiles(ticketsDir)
	if err != nil {
		return
	} else if posts != nil {
		enumeratePostsFromFiles(posts, ticketsDir)
	}
	postsWithChanges, err := enumeratePostsFiles(changesDir)
	if err != nil {
		return
	} else if postsWithChanges != nil {
		enumeratePostsFromFilesChanges(postsWithChanges, changesDir)
	}

}
func enumeratePostsFiles(dir string) ([]MattermostPost, error) {
	confFiles, err := WalkFiles(dir, "*.conf")
	if err != nil {
		return nil, err
	}
	posts := []MattermostPost{}
	for _, file := range confFiles {
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		post, err := MattermostPostFromJSON(content)
		post.Filepath = file
		if err == nil {
			posts = append(posts, post)
		}
	}
	return posts, nil
}
func enumeratePostsFromFilesChanges(posts []MattermostPost, dir string) error {
	itemtype := "Change"
	for _, post := range posts {
		ticket, err := GLPIModel.OneChange(post.Ticket.Id)
		if err == nil {
			if post.Id == "" {
				channel := FindChannelConfigByKey(post.ChannelID)
				if channel == nil {
					os.Remove(post.Filepath)
					continue
				}

			} else {

				//********* Комментарии
				oldCommentsCount := StringToInt(post.Ticket.CommentsCount)
				newCommentsCount := StringToInt(ticket.CommentsCount)
				if newCommentsCount > oldCommentsCount {
					comments, err := GLPIModel.TicketComments(post.Ticket.Id, post.LastComment, itemtype)
					if err != nil {
						log.Fatal("Error selecting tickets from db: " + err.Error())
					}
					if len(comments) == 0 {
						log.Warn("No comments")
					}
					lastComment := 0
					for _, comment := range comments {
						content := ConvertToMarkdown(comment.Content)
						sendMessageToMattermost(post.ChannelID, "**Комментарий работ** /*"+comment.Author+"*/: "+content, post.Id)
						lastComment, _ = strconv.Atoi(comment.Id)
					}
					post.LastComment = lastComment
				}
				//********* Решения
				oldSolutionsCount := StringToInt(post.Ticket.SolutionsCount)
				newSolutionsCount := StringToInt(ticket.SolutionsCount)
				if newSolutionsCount > oldSolutionsCount {
					solutions, err := GLPIModel.TicketSolutions(post.Ticket.Id, post.LastSolution, itemtype)
					if err != nil {
						log.Fatal("Error selecting solutions from db: " + err.Error())
					}
					if len(solutions) == 0 {
						log.Warn("No solutions")
					}
					lastSolution := 0
					for _, solution := range solutions {
						content := ConvertToMarkdown(solution.Content)
						sendMessageToMattermost(post.ChannelID, fmt.Sprintf("**Решение работ** /*"+solution.Author+"*/: "+content), post.Id)
						lastSolution, _ = strconv.Atoi(solution.Id)
					}
					post.LastSolution = lastSolution
				}
				//********* Статус
				if post.Ticket.StatusID != ticket.StatusID {
					sendMessageToMattermost(post.ChannelID, "Статус изменён на **"+ticket.Status+"**", post.Id)
				}
				//********* Категория
				if post.Ticket.Kat != ticket.Kat {
					sendMessageToMattermost(post.ChannelID, "Система изменена на **"+ticket.Kat+"**", post.Id)
				}

				//********** Update root post and save file
				if post.Ticket.StatusID != ticket.StatusID ||
					post.Ticket.Kat != ticket.Kat ||
					post.Ticket.Name != ticket.Name ||
					post.Ticket.Content != ticket.Content ||
					post.Ticket.Impact != ticket.Impact ||
					newSolutionsCount != oldSolutionsCount ||
					post.Ticket.DateMod != ticket.DateMod ||
					newCommentsCount != oldCommentsCount {
					ticket.CommentsCount = strconv.Itoa(newCommentsCount)
					err := updateChangeInMattermost(post.Id, ticket)
					if err == nil {
						post.Ticket = ticket
						savePostToFile(dir+post.Id+".conf", post)
					}
				}
			}
			if ticket.StatusID == "6" {
				os.Remove(post.Filepath)
			}
		} else {
			log.Error(err)
		}

	}
	return nil
}
func enumeratePostsFromFiles(posts []MattermostPost, dir string) error {
	itemtype := "Ticket"
	for _, post := range posts {
		ticket, err := GLPIModel.OneTicket(post.Ticket.Id)
		if err == nil {
			if post.Id == "" {
				channel := FindChannelConfigByKey(post.ChannelID)
				if channel == nil {
					os.Remove(post.Filepath)
					continue
				}

				ticketKategoryId := StringToInt(ticket.KatId)
				if channel.Level <= ticketKategoryId {
					postId, _ := sendTicketToMattermost(channel, ticket)
					newpost := MattermostPost{Id: postId, ChannelID: channel.Key, Ticket: ticket, LastComment: 0}
					savePostToFile(newpost.Id+".conf", newpost)
					os.Remove(post.Filepath)
				}
			} else {

				//********* Комментарии
				oldCommentsCount := StringToInt(post.Ticket.CommentsCount)
				newCommentsCount := StringToInt(ticket.CommentsCount)
				if newCommentsCount > oldCommentsCount {
					comments, err := GLPIModel.TicketComments(post.Ticket.Id, post.LastComment, itemtype)
					if err != nil {
						log.Fatal("Error selecting tickets from db: " + err.Error())
					}
					if len(comments) == 0 {
						log.Warn("No comments")
					}
					lastComment := 0
					for _, comment := range comments {
						content := ConvertToMarkdown(comment.Content)
						sendMessageToMattermost(post.ChannelID, "**Комментарий в заявке** /*"+comment.Author+"*/: "+content, post.Id)
						lastComment, _ = strconv.Atoi(comment.Id)
					}
					post.LastComment = lastComment
				}
				//********* Решения
				oldSolutionsCount := StringToInt(post.Ticket.SolutionsCount)
				newSolutionsCount := StringToInt(ticket.SolutionsCount)
				if newSolutionsCount > oldSolutionsCount {
					solutions, err := GLPIModel.TicketSolutions(post.Ticket.Id, post.LastSolution, itemtype)
					if err != nil {
						log.Fatal("Error selecting solutions from db: " + err.Error())
					}
					if len(solutions) == 0 {
						log.Warn("No solutions")
					}
					lastSolution := 0
					for _, solution := range solutions {
						content := ConvertToMarkdown(solution.Content)
						sendMessageToMattermost(post.ChannelID, fmt.Sprintf("**Решение заявки** /*"+solution.Author+"*/: "+content), post.Id)
						lastSolution, _ = strconv.Atoi(solution.Id)
					}
					post.LastSolution = lastSolution
				}
				//********* Статус
				if post.Ticket.StatusID != ticket.StatusID {
					sendMessageToMattermost(post.ChannelID, "Статус изменён на **"+ticket.Status+"**", post.Id)
				}
				//********* Категория
				if post.Ticket.Kat != ticket.Kat {
					sendMessageToMattermost(post.ChannelID, "Категория тяжести последствий отказа изменён на **"+ticket.Kat+"**", post.Id)
				}

				//********** Update root post and save file
				if post.Ticket.StatusID != ticket.StatusID ||
					post.Ticket.Kat != ticket.Kat ||
					post.Ticket.Name != ticket.Name ||
					post.Ticket.Content != ticket.Content ||
					post.Ticket.Impact != ticket.Impact ||
					post.Ticket.DateMod != ticket.DateMod ||
					newSolutionsCount != oldSolutionsCount ||
					newCommentsCount != oldCommentsCount {
					ticket.CommentsCount = strconv.Itoa(newCommentsCount)
					err := updateTicketInMattermost(post.Id, ticket)
					if err == nil {
						post.Ticket = ticket
						savePostToFile(dir+post.Id+".conf", post)
					}
				}
			}
			if ticket.StatusID == "6" {
				os.Remove(post.Filepath)
			}
		} else {
			log.Error(err)
		}

	}
	return nil
}

func enumerateChangesFromID(id int, ChangesDir string) error {
	changes, err := GLPIModel.Changes(id)
	if err != nil {
		log.Fatal("Error selecting tickets from db: " + err.Error())
	}
	if len(changes) == 0 {
		//		log.Warn("No tickets")
		return nil
	}
	var post MattermostPost
	for _, ticket := range changes {

		for _, channel := range mattChannelsConfig.Channels {
			if channel.Tip != "changes" {
				continue
			}
			if !StringStartWith(ticket.Org, channel.Org) {
				continue
			}
			postId, _ := sendChangeToMattermost(&channel, ticket)
			post = MattermostPost{Id: postId, ChannelID: channel.Key, Ticket: ticket, LastComment: 0}
			savePostToFile(ChangesDir+post.Id+".conf", post)
		}
	}
	lastTicketId, err := strconv.Atoi(changes[len(changes)-1].Id)
	if err == nil {
		saveToFile("id-changes.id", strconv.Itoa(lastTicketId))
	}
	return nil
}
func enumerateTicketsFromID(id int, ticketsDir string) error {
	tickets, err := GLPIModel.Tickets(id)
	if err != nil {
		log.Fatal("Error selecting tickets from db: " + err.Error())
	}

	if len(tickets) == 0 {
		//		log.Warn("No tickets")
		return nil
	}
	var post MattermostPost
	for _, ticket := range tickets {
		// if in_array(ticket, posts) {
		// 	continue
		// }
		for _, channel := range mattChannelsConfig.Channels {
			if channel.Tip != "tickets" {
				continue
			}
			if !StringStartWith(ticket.Org, channel.Org) {
				continue
			}

			ticketKategoryId := StringToInt(ticket.KatId)
			if channel.Level <= ticketKategoryId {
				postId, _ := sendTicketToMattermost(&channel, ticket)
				post = MattermostPost{Id: postId, ChannelID: channel.Key, Ticket: ticket, LastComment: 0}
				savePostToFile(ticketsDir+post.Id+".conf", post)
			} else {
				post = MattermostPost{Id: "", ChannelID: channel.Key, Ticket: ticket, LastComment: 0}
				savePostToFile(ticketsDir+channel.Key+"-"+ticket.Id+".conf", post)
			}
		}
	}

	lastTicketId, err := strconv.Atoi(tickets[len(tickets)-1].Id)
	if err == nil {
		saveToFile("id.id", strconv.Itoa(lastTicketId))
	}
	return nil
}

func in_array(val models.Ticket, posts []MattermostPost) (exists bool) {
	exists = false
	for _, post := range posts {
		if post.Ticket.Id == val.Id {
			exists = true
			return
		}
	}
	return
}
