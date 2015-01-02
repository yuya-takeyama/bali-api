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
	Id        uint64 `db:"id"         json:"id"`
	ListId    uint64 `db:"list_id"    json:"listId"`
	Name      string `db:"name"       json:"name"`
	IsChecked bool   `db:"is_checked" json:"isChecked"`
}

func NewBaggage() *Baggage {
	return &Baggage{0, 0, "", false}
}

func NewBaggageWithListId(listId uint64) *Baggage {
	return &Baggage{0, listId, "", false}
}

type List struct {
	Id   uint64 `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}

type ListWithBaggages struct {
	List
	Baggages []Baggage `json:"_baggages"`
}

func NewList() *List {
	return &List{0, ""}
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func NewErrorResponse(message string) *ErrorResponse {
	return &ErrorResponse{message}
}

func (er *ErrorResponse) Json() ([]byte, error) {
	return json.Marshal(er)
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
		checkErr(err, "Failed to insert list")

		json, err := json.Marshal(list)
		checkErr(err, "Failed to encode inserted data")

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	})

	goji.Get("/lists/:list_id", func(c web.C, w http.ResponseWriter, r *http.Request) {
		list := NewList()
		err := dbmap.SelectOne(list, "SELECT * FROM lists WHERE id = ? LIMIT 1", c.URLParams["list_id"])
		if err != nil {
			handleSelectOneErr(err, w, "List")
			return
		}

		var baggages []Baggage
		_, err = dbmap.Select(&baggages, "SELECT * FROM baggages WHERE list_id = ? ORDER BY id", c.URLParams["list_id"])

		ListWithBaggages := ListWithBaggages{*list, baggages}

		json, err := json.Marshal(ListWithBaggages)
		checkErr(err, "Failed to encode fetched data")

		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	})

	goji.Post("/lists/:list_id/baggages", func(c web.C, w http.ResponseWriter, r *http.Request) {
		list := NewList()
		err := dbmap.SelectOne(list, "SELECT * FROM lists WHERE id = ? LIMIT 1", c.URLParams["list_id"])
		if err != nil {
			handleSelectOneErr(err, w, "List")
			return
		}

		baggage := NewBaggageWithListId(list.Id)
		err = json.NewDecoder(r.Body).Decode(baggage)
		checkErr(err, "Failed to decode JSON")

		err = dbmap.Insert(baggage)
		checkErr(err, "Failed to insert baggege")

		json, err := json.Marshal(baggage)
		checkErr(err, "Failed to encode inserted data")

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	})

	goji.Delete("/lists/:list_id/baggages/:baggage_id", func(c web.C, w http.ResponseWriter, r *http.Request) {
		list := NewList()
		err := dbmap.SelectOne(list, "SELECT * FROM lists WHERE id = ? LIMIT 1", c.URLParams["list_id"])
		if err != nil {
			handleSelectOneErr(err, w, "List")
			return
		}

		baggage := NewBaggage()
		err = dbmap.SelectOne(baggage, "SELECT * FROM baggages WHERE id = ? AND list_id = ? LIMIT 1", c.URLParams["baggage_id"], c.URLParams["list_id"])
		if err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNoContent)
				return
			} else {
				handleSelectOneErr(err, w, "Baggage")
			}
			return
		}

		_, err = dbmap.Delete(baggage)
		checkErr(err, "Failed to delete baggage")

		w.WriteHeader(http.StatusNoContent)
	})

	updateIsChecked := func(c web.C, w http.ResponseWriter, r *http.Request, isChecked bool) {
		list := NewList()
		err := dbmap.SelectOne(list, "SELECT * FROM lists WHERE id = ? LIMIT 1", c.URLParams["list_id"])
		if err != nil {
			handleSelectOneErr(err, w, "List")
			return
		}

		baggage := NewBaggage()
		err = dbmap.SelectOne(baggage, "SELECT * FROM baggages WHERE id = ? AND list_id = ? LIMIT 1", c.URLParams["baggage_id"], c.URLParams["list_id"])
		if err != nil {
			handleSelectOneErr(err, w, "Baggage")
			return
		}

		baggage.IsChecked = isChecked

		_, err = dbmap.Update(baggage)
		checkErr(err, "Failed to update baggege")

		json, err := json.Marshal(baggage)
		checkErr(err, "Failed to encode updated data")

		fmt.Fprintln(w, bytes.NewBuffer(json).String())
	}

	goji.Post("/lists/:list_id/baggages/:baggage_id/check", func(c web.C, w http.ResponseWriter, r *http.Request) {
		updateIsChecked(c, w, r, true)
	})

	goji.Post("/lists/:list_id/baggages/:baggage_id/uncheck", func(c web.C, w http.ResponseWriter, r *http.Request) {
		updateIsChecked(c, w, r, false)
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

func handleSelectOneErr(err error, w http.ResponseWriter, name string) {
	if err == sql.ErrNoRows {
		er := NewErrorResponse(fmt.Sprintf("%s is not found", name))
		json, err := er.Json()
		checkErr(err, "Failed to encode error response")
		http.Error(w, string(json), http.StatusNotFound)
		return
	}
	checkErr(err, fmt.Sprintf("Failed to fetch a %s", name))
}
