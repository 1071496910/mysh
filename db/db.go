package db

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/go-sql-driver/mysql"

	"github.com/1071496910/mysh/cons"
)

func MakePasswordGetter() func(uid string) (string, error) {
	db, err := sql.Open("mysql", cons.MysqlStr)
	if err != nil {
		log.Println(err)
		panic(err)
		//return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), cons.MysqlTimeout)
	stmt, err := db.PrepareContext(ctx, "SELECT password FROM "+cons.UinfoTable+" WHERE uid= ? ;")
	defer cancel()

	if err != nil {
		log.Println(err)
		panic(err)
		//return nil
	}

	return func(uid string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), cons.MysqlTimeout)
		var password string
		err := stmt.QueryRowContext(ctx, uid).Scan(&password)
		defer cancel()
		if err != nil {
			log.Println(err)
		}
		return password, err
	}
}
