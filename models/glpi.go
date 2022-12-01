package models

import (
	"fmt"

	"github.com/saygik/go-glpi-to-matt/db"
)

type Ticket struct {
	Id             string `db:"id" json:"id"`
	Kat            string `db:"kat" json:"kat"`
	KatId          string `db:"katid" json:"katid"`
	Status         string `db:"status" json:"status"`
	StatusID       string `db:"status_id" json:"status_id"`
	Impact         string `db:"impact" json:"impact"`
	Date           string `db:"date" json:"date"`
	DateMod        string `db:"date_mod" json:"date_mod"`
	Name           string `db:"name" json:"name"`
	Content        string `db:"content" json:"content"`
	Author         string `db:"author" json:"author"`
	Org            string `db:"org" json:"org"`
	CommentsCount  string `db:"comments_count" json:"comments_count"`
	SolutionsCount string `db:"solutions_count" json:"solutions_count"`
}
type Comment struct {
	Id      string `db:"id" json:"id"`
	Content string `db:"content" json:"content"`
	DateMod string `db:"date_mod" json:"date_mod"`
	Author  string `db:"author" json:"author"`
}

// GLPIModel ...
type GLPIModel struct{}

func (m GLPIModel) TicketComments(ticketID string, lastId int) (comments []Comment, err error) {
	var proc = fmt.Sprintf(`SELECT glpi_itilfollowups.id,glpi_itilfollowups.content, glpi_itilfollowups.date_mod,
                            CONCAT(realname," ", firstname) AS author  FROM glpi_itilfollowups 
                            LEFT JOIN glpi_users ON glpi_itilfollowups.users_id= glpi_users.id 
                            WHERE items_id=%s AND glpi_itilfollowups.id>%d ORDER BY glpi_itilfollowups.id`, ticketID, lastId)
	_, err = db.GetDB().Select(&comments, proc)
	if err != nil {
		return nil, err
	}
	return comments, nil

}
func (m GLPIModel) TicketSolutions(ticketID string, lastId int) (comments []Comment, err error) {
	var proc = fmt.Sprintf(`SELECT glpi_itilsolutions.id,glpi_itilsolutions.content, glpi_itilsolutions.date_mod,
                            CONCAT(realname," ", firstname) AS author  FROM glpi_itilsolutions 
                            LEFT JOIN glpi_users ON glpi_itilsolutions.users_id= glpi_users.id 
                            WHERE items_id=%s AND glpi_itilsolutions.id>%d ORDER BY glpi_itilsolutions.id`, ticketID, lastId)
	_, err = db.GetDB().Select(&comments, proc)
	if err != nil {
		return nil, err
	}
	return comments, nil

}
func (m GLPIModel) OneTicket(ticketID string) (ticket Ticket, err error) {
	var proc = fmt.Sprintf(`SELECT glpi_tickets.id , glpi_tickets.content,
                                CONCAT(ifnull(NULLIF(glpi_users.realname, ''), 'не опреденен'),' ', ifnull(NULLIF(glpi_users.firstname, ''),'')) AS author,
								ifnull(glpi_plugin_fields_failcategoryfielddropdowns.completename,"-") AS kat,
								ifnull(glpi_plugin_fields_failcategoryfielddropdowns.id,0) AS katid,
								CASE glpi_tickets.status
									WHEN 1 THEN "новый" WHEN 2 THEN "в работе (назначен)" WHEN 3 THEN "в работе (запланирован)" WHEN 4 THEN "ожидающий" WHEN 5 THEN "решен" WHEN 6 THEN "закрыт"
									ELSE "неизвестен"
								END AS status,
                                glpi_tickets.status as status_id,
                                (SELECT count(id) from glpi_itilfollowups WHERE items_id=%s) as comments_count,
                                (SELECT count(id) from glpi_itilsolutions WHERE items_id=%s) as solutions_count,
								glpi_tickets.name, glpi_tickets.impact, glpi_entities.completename as org, IFNULL(glpi_tickets.date,'') as date, glpi_tickets.date_mod FROM glpi_tickets 
								LEFT JOIN glpi_entities ON glpi_tickets.entities_id = glpi_entities.id
							    LEFT JOIN glpi_users ON glpi_tickets.users_id_recipient=glpi_users.id
								LEFT JOIN glpi_plugin_fields_ticketfailures ON glpi_plugin_fields_ticketfailures.items_id=glpi_tickets.id
								LEFT JOIN glpi_plugin_fields_failcategoryfielddropdowns ON glpi_plugin_fields_failcategoryfielddropdowns.id=glpi_plugin_fields_ticketfailures.plugin_fields_failcategoryfielddropdowns_id
								WHERE glpi_tickets.id=%s`, ticketID, ticketID, ticketID)

	err = db.GetDB().SelectOne(&ticket, proc)

	return ticket, err

}

func (m GLPIModel) Tickets(lastId int) (tickets []Ticket, err error) {
	var proc = fmt.Sprintf(`SELECT glpi_tickets.id , glpi_tickets.content,
                                CONCAT(ifnull(NULLIF(glpi_users.realname, ''), 'не опреденен'),' ', ifnull(NULLIF(glpi_users.firstname, ''),'')) AS author,
								ifnull(glpi_plugin_fields_failcategoryfielddropdowns.completename,"-") AS kat,
								ifnull(glpi_plugin_fields_failcategoryfielddropdowns.id,0) AS katid,
								CASE glpi_tickets.status
									WHEN 1 THEN "новый" WHEN 2 THEN "в работе (назначен)" WHEN 3 THEN "в работе (запланирован)" WHEN 4 THEN "ожидающий" WHEN 5 THEN "решен" WHEN 6 THEN "закрыт"
									ELSE "неизвестен"
								END AS status,
                                glpi_tickets.status as status_id,
								glpi_tickets.name, glpi_tickets.impact, glpi_entities.completename as org, IFNULL(glpi_tickets.date,'') as date, glpi_tickets.date_mod FROM glpi_tickets 
								LEFT JOIN glpi_entities ON glpi_tickets.entities_id = glpi_entities.id
							    LEFT JOIN glpi_users ON glpi_tickets.users_id_recipient=glpi_users.id
								LEFT JOIN glpi_plugin_fields_ticketfailures ON glpi_plugin_fields_ticketfailures.items_id=glpi_tickets.id
								LEFT JOIN glpi_plugin_fields_failcategoryfielddropdowns ON glpi_plugin_fields_failcategoryfielddropdowns.id=glpi_plugin_fields_ticketfailures.plugin_fields_failcategoryfielddropdowns_id
								WHERE glpi_tickets.is_deleted<>TRUE  AND glpi_plugin_fields_failcategoryfielddropdowns.id>4 
                     		    AND glpi_tickets.name not like '%%020202%%' AND glpi_tickets.name not like '%%test%%' 
                                AND glpi_tickets.id>%d limit 10`, lastId)
	_, err = db.GetDB().Select(&tickets, proc)

	//rows, err := GetDB().Query("SELECT glpi_tickets.id, glpi_tickets.name FROM glpi_tickets")
	if err != nil {
		return nil, err
	}

	//	_, err = db.GetDB().Select(&tickets, "SELECT glpi_tickets.id, glpi_tickets.name, glpi_tickets.date, glpi_tickets.closedate, glpi_tickets.solvedate, glpi_tickets.date_mod, glpi_tickets.`status` FROM glpi_tickets ")
	return tickets, nil
}
