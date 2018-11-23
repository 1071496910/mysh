package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"

	"github.com/1071496910/mysh/cons"
)

var (
	db *sql.DB
)

func init() {
	var err error
	db, err = sql.Open("mysql", cons.MysqlStr)
	if err != nil {
		log.Println(err)
		panic(err)
	}
}

func MakeSaltPasswordGetter() func(uid string) (string, string, error) {

	ctx, cancel := context.WithTimeout(context.Background(), cons.MysqlTimeout)
	stmt, err := db.PrepareContext(ctx, "SELECT salt, password FROM "+cons.UinfoTable+" WHERE uid= ? ;")
	defer cancel()

	if err != nil {
		log.Println(err)
		panic(err)
		//return nil
	}

	return func(uid string) (string, string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), cons.MysqlTimeout)
		var password string
		var salt string
		err := stmt.QueryRowContext(ctx, uid).Scan(&salt, &password)
		defer cancel()
		if err != nil {
			log.Println(err)
		}
		return salt, password, err
	}
}

func SaltPasswordOperator() func(uid string, salt string, password string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cons.MysqlTimeout)
	stmt, err := db.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %v (uid, salt, password) values (?, ?, ?)", cons.UinfoTable))
	defer cancel()
	if err != nil {
		panic(err)
	}

	return func(uid string, salt string, password string) (int64, error) {
		ctx, cancel := context.WithTimeout(context.Background(), cons.MysqlTimeout)
		result, err := stmt.ExecContext(ctx, uid, salt, password)
		defer cancel()
		if err != nil {
			return 0, err
		}
		return result.RowsAffected()

	}
}
