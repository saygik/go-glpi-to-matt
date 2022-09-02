package db

import (
	"database/sql"
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
	//var version string
	//db.QueryRow("SELECT VERSION()").Scan(&version)
	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{}}
	//dbmap.TraceOn("[gorp]", log.New(os.Stdout, "golang-gin:", log.Lmicroseconds)) //Trace database requests
	return dbmap, nil
}

// GetDB ...
func GetDB() *gorp.DbMap {
	return db
}
func CloseDB() {
	_ = db.Db.Close()
}
