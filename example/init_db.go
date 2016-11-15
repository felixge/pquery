package main

import (
	"database/sql"
	"fmt"
	"log"
)

func initDB(db *sql.DB, scale int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	log.Printf("Creating schema ...")
	sql := `
DROP SCHEMA IF EXISTS ` + schema + ` CASCADE;
CREATE SCHEMA ` + schema + `;

CREATE TABLE transactions (
	id integer PRIMARY KEY,
	user_id integer NOT NULL,
	category_id integer NOT NULL,
	dough money NOT NULL
);
`
	if _, err := tx.Exec(sql); err != nil {
		return err
	}
	base := 100000
	log.Printf("Generating %d transactions ...", scale*base)
	sql = `
INSERT INTO transactions
SELECT
	i,
	random() * $1 * 10000 + 1,
	i % 4 + 1,
	random() * 1000::money
FROM generate_series(1, $1::integer * $2) i
`
	if _, err := tx.Exec(sql, scale, base); err != nil {
		return err
	}
	log.Printf("Creating index ...")
	if _, err := tx.Exec(`CREATE INDEX transactions_partition_idx ON transactions(category_id, (user_id % ` + fmt.Sprintf("%d", pSize) + `))`); err != nil {
		return err
	} else if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("Running vacuum analyze ...")
	_, err = db.Exec(`VACUUM ANALYZE transactions`)
	return err
}
