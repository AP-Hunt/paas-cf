package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type dbInstance struct {
	Connection *sql.DB
}

const (
	value = 5
)

var (
	dbtable string
	created bool

	chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890_")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	dbtable = RandStringRunes(16)

	conn, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalln(err)
	}

	db := dbInstance{conn}

	err = db.create()
	if err != nil {
		log.Fatalln(err)
	}

	if !created {
		err = db.insert()
		if err != nil {
			log.Fatalln(err)
		}

		created = true
	}

	for {
		err := db.check()
		if err != nil {
			log.Fatalln(err)
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func (i *dbInstance) check() error {
	statement := fmt.Sprintf("SELECT * FROM \"%s\" LIMIT 1;", dbtable)
	rows, err := i.Connection.Query(statement)
	if err != nil {
		return err
	}

	for rows.Next() {
		var val int64
		err := rows.Scan(&val)
		if err != nil {
			return err
		}

		if val != value {
			log.Fatalf("Expected %d got %d", value, val)
		}
	}

	return nil
}

func (i *dbInstance) create() error {
	table := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (value integer);", dbtable)
	_, err := i.Connection.Exec(table)
	if err != nil {
		return err
	}

	return nil
}

func (i *dbInstance) insert() error {
	statement := fmt.Sprintf("INSERT INTO \"%s\" VALUES (%d);", dbtable, value)
	_, err := i.Connection.Exec(statement)
	if err != nil {
		return err
	}

	return nil
}

// RandStringRunes asda
func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}