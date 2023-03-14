package db

import (
	"mariposa/models"

	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"os"
)

func Init() (SQLiteDriver *sql.DB, err error) {
	if _, err := os.Stat("./mariposa.db")
	os.IsNotExist(err) {
		fmt.Println("db does not exist, creating it")
		os.Create("./mariposa.db")
	}

	db, err := sql.Open("sqlite3", "./mariposa.db")
	if err != nil {
		return nil, err
	}

	sqlStmt := `
	create table if not exists police_log (
		id integer primary key,
		time_taken datetime,
		nature_of_call text,
		disposition text,
		location text,
		city text
	);
	create table if not exists days_processed (
		date text primary key
	);
	`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func InsertRecord(db *sql.DB, record models.Record) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert into police_log (time_taken, nature_of_call, disposition, location, city) values (?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}

	defer stmt.Close()
	_, err = stmt.Exec(record.TimeTaken, record.NatureOfCall, record.Disposition, record.Location, record.City)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func InsertDate(db *sql.DB, date string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare("insert into days_processed (date) values (?)")
	if err != nil {
		return err
	}

	defer stmt.Close()
	_, err = stmt.Exec(date)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func DateExists(db *sql.DB, date string) (bool, error) {
	var exists bool
	err := db.QueryRow("select exists(select 1 from days_processed where date = ?)", date).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}