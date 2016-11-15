package main

import (
	"database/sql"
	"flag"
	"log"

	_ "github.com/lib/pq"
)

const schema = "pquery"

func main() {
	var (
		initialize = flag.Bool("i", false, "Initialize db")
		scale      = flag.Int("s", 1, "Scale factor for init.")
		c          = flag.Int("c", 0, "Concurrency, 0 = run non-concurrent query version.")
		dsn        = flag.String("dsn", "user=postgres sslmode=disable search_path="+schema, "lib/pq database DSN")
	)
	flag.Parse()
	db, err := sql.Open("postgres", *dsn)
	if err != nil {
		log.Fatal(err)
	}
	if *initialize {
		if err := initDB(db, *scale); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := query(db, *c); err != nil {
			log.Fatal(err)
		}
	}
}
