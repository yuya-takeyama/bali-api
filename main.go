package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
	"log"
	"net/http"
)

type Baggage struct {
	Id        int64  `db:"id"         json:"id"`
	ListId    int64  `db:"list_id"    json:"listId"`
	Name      string `db:"name"       json:"name"`
	IsChecked bool   `db:"is_checked" json:"isChecked"`
}

type List struct {
	Id   int64  `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}

type ListWithBaggages struct {
	List
	Baggages []Baggage `json:"_baggages"`
}

func NewList() *List {
	return &List{0, ""}
}

func main() {
	dbmap := initDb()

	goji.Get("/lists", func(c web.C, w http.ResponseWriter, r *http.Request) {
		var lists []List
		_, err := dbmap.Select(&lists, "SELECT * FROM lists ORDER BY id DESC")

		json, err := json.Marshal(lists)
		checkErr(err, "Failed to encode fetched data")

		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	})

	goji.Post("/lists", func(c web.C, w http.ResponseWriter, r *http.Request) {
		list := NewList()
		err := json.NewDecoder(r.Body).Decode(list)
		checkErr(err, "Failed to decode JSON")

		err = dbmap.Insert(list)
		checkErr(err, "Failed to insert")

		json, err := json.Marshal(list)
		checkErr(err, "Failed to encode inserted data")

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	})

	goji.Get("/lists/:id", func(c web.C, w http.ResponseWriter, r *http.Request) {
		list := NewList()
		err := dbmap.SelectOne(list, "SELECT * FROM lists WHERE id = ? LIMIT 1", c.URLParams["id"])
		checkErr(err, "Failed to fetch a list")

		var baggages []Baggage
		_, err = dbmap.Select(&baggages, "SELECT * FROM baggages WHERE list_id = ? ORDER BY id", c.URLParams["id"])

		ListWithBaggages := ListWithBaggages{*list, baggages}

		json, err := json.Marshal(ListWithBaggages)
		checkErr(err, "Failed to encode fetched data")

		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	})

	goji.Serve()
}

func initDb() *gorp.DbMap {
	db, err := sql.Open("sqlite3", "./db/development.sqlite3")
	checkErr(err, "Failed to open database")

	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}

	dbmap.AddTableWithName(Baggage{}, "baggages").SetKeys(true, "Id")
	dbmap.AddTableWithName(List{}, "lists").SetKeys(true, "Id")

	err = dbmap.CreateTablesIfNotExists()
	checkErr(err, "Create tables failed")

	return dbmap
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, err)
		panic(err)
	}
}
