package main

import (
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
	log.Info("-")
	lastid, err := os.ReadFile("id.id")
	if err != nil {
		log.Warn("Error loading id.id file or wrong file, please create one in the root directory: " + err.Error())
		lastid = []byte("0")
	}
	id, _ := strconv.Atoi(string(lastid))
	fmt.Printf("Number is %d", id)
	//	db.Init(fmt.Sprintf("%s:%s@tcp(%s)/%s", os.Getenv("DB_USER"), os.Getenv("DB_PASS"), os.Getenv("DB_SERVER"), os.Getenv("DB_NAME")))
	db.Init(fmt.Sprintf("%s:%s@tcp(%s)/%s", os.Getenv("GLPI_TO_MATT_DB_USER"), os.Getenv("GLPI_TO_MATT_DB_PASS"), os.Getenv("GLPI_TO_MATT_DB_SERVER"), os.Getenv("GLPI_TO_MATT_DB_NAME")))
	defer db.CloseDB()
	MattermostModel.Init()

	enumerateTicketsFromID(id)
	enumeratePostsFromFiles(exPath)

}
func enumeratePostsFromFiles(dir string) error {
	confFiles, err := WalkFiles(dir, "*.conf")
	if err != nil {
		return err
	}
	//fmt.Println(confFiles) // for example /home/user
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
	for _, post := range posts {
		ticket, err := GLPIModel.OneTicket(post.Ticket.Id)
		if err == nil {
			if post.Ticket.StatusID != ticket.StatusID {
				sendMessageToMattermost("Статус изменён на **"+ticket.Status+"**", post.Id)
			}
			if post.Ticket.Kat != ticket.Kat {
				sendMessageToMattermost("Категория тяжести последствий отказа изменён на **"+ticket.Kat+"**", post.Id)
			}
			oldCommentsCount := StringToInt(post.Ticket.CommentsCount)
			newCommentsCount := StringToInt(ticket.CommentsCount)
			if newCommentsCount > oldCommentsCount {
				comments, err := GLPIModel.TicketComments(post.Ticket.Id, post.LastComment)
				if err != nil {
					log.Fatal("Error selecting tickets from db: " + err.Error())
				}
				if len(comments) == 0 {
					log.Warn("No comments")
				}
				lastComment := 0
				for _, comment := range comments {
					content := ConvertToMarkdown(comment.Content)
					sendMessageToMattermost("`Комментарий в заявке: "+comment.Author+":` "+content, post.Id)
					lastComment, _ = strconv.Atoi(comment.Id)
				}
				post.LastComment = lastComment
			}
			oldSolutionsCount := StringToInt(post.Ticket.SolutionsCount)
			newSolutionsCount := StringToInt(ticket.SolutionsCount)
			if newSolutionsCount > oldSolutionsCount {
				solutions, err := GLPIModel.TicketSolutions(post.Ticket.Id, post.LastSolution)
				if err != nil {
					log.Fatal("Error selecting solutions from db: " + err.Error())
				}
				if len(solutions) == 0 {
					log.Warn("No solutions")
				}
				lastSolution := 0
				for _, solution := range solutions {
					content := ConvertToMarkdown(solution.Content)
					sendMessageToMattermost(fmt.Sprintf("`Решение заявки: "+solution.Author+":` "+content), post.Id)
					lastSolution, _ = strconv.Atoi(solution.Id)
				}
				post.LastSolution = lastSolution
			}

			if post.Ticket.StatusID != ticket.StatusID ||
				post.Ticket.Kat != ticket.Kat ||
				post.Ticket.Name != ticket.Name ||
				post.Ticket.Content != ticket.Content ||
				post.Ticket.Impact != ticket.Impact ||
				newCommentsCount != oldCommentsCount {
				ticket.CommentsCount = strconv.Itoa(newCommentsCount)
				err := updateTicketInMattermost(post.Id, ticket)
				if err == nil {
					post.Ticket = ticket
					savePostToFile(post)
				}
			}

			if post.Ticket.StatusID == "6" {
				os.Remove(post.Filepath)
			}
		} else {
			log.Error(err)
		}

	}
	return nil
}

func enumerateTicketsFromID(id int) error {
	tickets, err := GLPIModel.Tickets(id)
	if err != nil {
		log.Fatal("Error selecting tickets from db: " + err.Error())
	}

	if len(tickets) == 0 {
		log.Warn("No tickets")
		return nil
	}

	for _, ticket := range tickets {
		postId, err := sendTicketToMattermost(ticket)
		if err != nil {
			log.Warn("Error sending ticket " + ticket.Id + ":" + err.Error())
		}
		//		MattermostModel.UpdateThreadFollowAllUsersInChannel(postId)
		post := MattermostPost{Id: postId, Ticket: ticket, LastComment: 0}
		savePostToFile(post)
		log.Info("Sended ticket " + ticket.Id)

	}

	lastTicketId, err := strconv.Atoi(tickets[len(tickets)-1].Id)
	if err == nil {
		saveToFile("id.id", strconv.Itoa(lastTicketId))
	}
	return nil
}
