package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

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

	lastChangeId, err := os.ReadFile("id-changes.id")
	if err != nil {
		log.Warn("Error loading idc.id file or wrong file, please create one in the root directory: " + err.Error())
		lastChangeId = []byte("0")
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

	enumerateTicketsFromID(ticketsDir)
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
						content = strings.ReplaceAll(content, "\n", "\n>")
						userProps := getUserPropsInComments(comment.AuthorName)
						commentText := ":user: " + comment.Author + " " + userProps + "\n>" + content
						sendMessageToMattermost(post.ChannelID, commentText, post.Id)
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
						content = strings.ReplaceAll(content, "\n", "\n>")
						userProps := getUserPropsInComments(solution.AuthorName)
						solutionText := "**Решение** \n:user: " + solution.Author + " " + userProps + "\n>" + content
						sendMessageToMattermost(post.ChannelID, solutionText, post.Id)
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
					post.Ticket.SolveDate != ticket.SolveDate ||
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
			if strings.Contains(err.Error(), "no rows in result set") {
				log.Error("Ошибка: нет изменения с таким id в базе данных: " + post.Ticket.Id + ". Файл: " + post.Filepath + " удалён")
				os.Remove(post.Filepath)
				continue
			}

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
				comments, err := GLPIModel.TicketComments(post.Ticket.Id, 0, itemtype)

				if err != nil {
					log.Fatal("Error selecting tickets from db: " + err.Error())
				}
				commentsUpdated := false
				if len(comments) > 0 {
					for i, comment := range comments {
						content := ConvertToMarkdown(comment.Content)
						content = strings.ReplaceAll(content, "\n", "\n>")
						userProps := getUserPropsInComments(comment.AuthorName)
						commentText := ":user: " + comment.Author + " " + userProps + "\n>" + content
						if num := comment_in_array(comment, post.Ticket.Comments); num > -1 {
							if comment.DateMod != post.Ticket.Comments[num].DateMod {
								commentsUpdated = true
								updatePost(post.Ticket.Comments[num].PostId, commentText)
							}
							comments[i].Content = ""
							comments[i].Author = ""
							comments[i].PostId = post.Ticket.Comments[num].PostId
							continue // break here
						}
						commentsUpdated = true
						postId_c, err := sendMessageToMattermost(post.ChannelID, commentText, post.Id)
						if err == nil {
							comments[i].PostId = postId_c
						} else {
							comments[i].PostId = "x"
						}
						comments[i].Content = ""
						comments[i].Author = ""
					}
				}
				ticket.Comments = comments
				//********* Решения
				solutions, err := GLPIModel.TicketSolutions(post.Ticket.Id, post.LastSolution, itemtype)
				if err != nil {
					log.Fatal("Error selecting solutions from db: " + err.Error())
				}
				solutionsUpdated := false
				if len(solutions) > 0 {
					for i, solution := range solutions {
						content := ConvertToMarkdown(solution.Content)
						content = strings.ReplaceAll(content, "\n", "\n>")
						userProps := getUserPropsInComments(solution.AuthorName)
						solutionText := "**Решение** \n:user: " + solution.Author + " " + userProps + "\n>" + content
						if num := comment_in_array(solution, post.Ticket.Solutions); num > -1 {
							if solution.DateMod != post.Ticket.Solutions[num].DateMod {
								solutionsUpdated = true
								updatePost(post.Ticket.Solutions[num].PostId, solutionText)
							}
							solutions[i].PostId = post.Ticket.Solutions[num].PostId
							solutions[i].Content = ""
							solutions[i].Author = ""
							continue // break here
						}
						solutionsUpdated = true
						postId_c, err := sendMessageToMattermost(post.ChannelID, solutionText, post.Id)
						if err == nil {
							solutions[i].PostId = postId_c
						} else {
							solutions[i].PostId = "x"
						}
						solutions[i].Content = ""
						solutions[i].Author = ""
					}
				}
				ticket.Solutions = solutions
				//********* Решения

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
					post.Ticket.SolveDate != ticket.SolveDate ||
					post.Ticket.DateMod != ticket.DateMod ||
					commentsUpdated ||
					solutionsUpdated {
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
			if strings.Contains(err.Error(), "no rows in result set") {
				log.Error("Ошибка: нет заявки с таким id в базе данных: " + post.Ticket.Id + ". Файл: " + post.Filepath + " удалён")
				os.Remove(post.Filepath)
				continue
			}
			log.Error(err)
			//			errors.New(err)
			//			post.Ticket.Id
		}

	}
	return nil
}

func enumerateChangesFromID(id int, ChangesDir string) error {
	changes, err := GLPIModel.Changes(id)
	//* Только для тестирования
	//changes, err := GLPIModel.ChangesTest(id)
	//**
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
func enumerateTicketsFromID(ticketsDir string) error {
	tickets, err := GLPIModel.Tickets()
	//* Только для тестирования
	//tickets, err := GLPIModel.TicketsTest()
	//**
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
		GLPIModel.AddOtkaz(ticket.Id)
		//* Только для тестирования
		//GLPIModel.AddOtkazTest(ticket.Id)
		//**

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

func comment_in_array(val models.Comment, comments []models.Comment) (idComment int) {
	idComment = -1
	for i, comment := range comments {
		if comment.Id == val.Id {
			idComment = i
			return
		}
	}
	return
}
