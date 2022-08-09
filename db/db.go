package db

import (
	"database/sql"
	"fmt"
	"github.com/go-gorp/gorp"
	_ "github.com/go-sql-driver/mysql"
	"log"
)

//_ "github.com/lib/pq" //import postgres

// DB ...
type DB struct {
	*sql.DB
}

var db *gorp.DbMap

// Init ...
func Init(connectionString string) {
	dbinfo := connectionString
	var err error
	db, err = ConnectDB(dbinfo)
	if err != nil {
		log.Fatal(err)
	}
}

// ConnectDB ...
func ConnectDB(dataSourceName string) (*gorp.DbMap, error) {
	db, err := sql.Open("mysql", dataSourceName)
	//	db, _ := sql.Open("mysql", "dellis:@/shud")
	//defer db.Close()
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	var version string
	db.QueryRow("SELECT VERSION()").Scan(&version)
	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{}}
	//dbmap.TraceOn("[gorp]", log.New(os.Stdout, "golang-gin:", log.Lmicroseconds)) //Trace database requests
	return dbmap, nil
}

// GetDB ...
func GetDB() *gorp.DbMap {
	return db
}
func CloseDB() {
	db.Db.Close()
}

type Ticket struct {
	Id      string `db:"id" json:"id"`
	Kat     string `db:"kat" json:"kat"`
	Status  string `db:"status" json:"status"`
	Impact  string `db:"impact" json:"impact"`
	Date    string `db:"date" json:"date"`
	Name    string `db:"name" json:"name"`
	Content string `db:"content" json:"content"`
	Author  string `db:"author" json:"author"`
	Org     string `db:"org" json:"org"`
}

func Tickets(lastId int) (tickets []Ticket, err error) {
	sql := fmt.Sprintf(`SELECT glpi_tickets.id , glpi_tickets.content,
                                CONCAT(ifnull(NULLIF(glpi_users.realname, ''), glpi_users.name),' ', ifnull(NULLIF(glpi_users.firstname, ''),'')) AS author,
								ifnull(glpi_plugin_fields_failcategoryfielddropdowns.completename,"-") AS kat,
								CASE glpi_tickets.status
									WHEN 1 THEN "новый" WHEN 2 THEN "в работе (назначен)" WHEN 3 THEN "в работе (запланирован)" WHEN 4 THEN "ожидающий" WHEN 5 THEN "решен" WHEN 6 THEN "закрыт"
									ELSE "неизвестен"
								END AS status,
								glpi_tickets.name, glpi_tickets.impact, glpi_entities.completename as org, glpi_tickets.date FROM glpi_tickets 
								LEFT JOIN glpi_entities ON glpi_tickets.entities_id = glpi_entities.id
							    LEFT JOIN glpi_users ON glpi_tickets.users_id_recipient=glpi_users.id
								LEFT JOIN glpi_plugin_fields_ticketfailures ON glpi_plugin_fields_ticketfailures.items_id=glpi_tickets.id
								LEFT JOIN glpi_plugin_fields_failcategoryfielddropdowns ON glpi_plugin_fields_failcategoryfielddropdowns.id=glpi_plugin_fields_ticketfailures.plugin_fields_failcategoryfielddropdowns_id
								WHERE glpi_tickets.is_deleted<>TRUE  AND glpi_plugin_fields_failcategoryfielddropdowns.id>4 
                               AND glpi_tickets.id>%d limit 10`, lastId)
	_, err = GetDB().Select(&tickets, sql)

	//rows, err := GetDB().Query("SELECT glpi_tickets.id, glpi_tickets.name FROM glpi_tickets")
	if err != nil {
		return nil, err
	}

	//	_, err = db.GetDB().Select(&tickets, "SELECT glpi_tickets.id, glpi_tickets.name, glpi_tickets.date, glpi_tickets.closedate, glpi_tickets.solvedate, glpi_tickets.date_mod, glpi_tickets.`status` FROM glpi_tickets ")
	return tickets, nil
}
