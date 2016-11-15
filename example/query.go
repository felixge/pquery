package main

import (
	"database/sql"
	"log"
	"math"
	"time"

	"github.com/felixge/pquery"
)

const pSize = 1000

func query(db *sql.DB, c int) error {
	querySQL := querySQL(c)
	log.Printf("Executing sql with concurrency %d\n%s\n", c, querySQL)
	start := time.Now()
	var queryErr error
	execQuery(db, querySQL, c, func(rows *sql.Rows, err error) {
		if err != nil {
			queryErr = err
			return
		}
		defer rows.Close()
		var count = 0
		for rows.Next() {
			count++
		}
		if err := rows.Err(); err != nil {
			queryErr = err
			return
		}
		log.Printf("%d results in %s", count, time.Since(start))
	})
	return queryErr
}

// execQuery executes sql with concurrency c, calling fn with the results. This
// API is a bit awkward, but it's hard to make it nicer, since we need to allow
// the caller to drain the sql.Rows before the transactions commits when
// executing with c > 0.
func execQuery(db *sql.DB, sql string, c int, fn func(*sql.Rows, error)) {
	if c == 0 {
		rows, err := db.Query(sql)
		fn(rows, err)
		return
	}
	p := &pquery.Plan{
		AggDB: db,
		AggTable: &pquery.Table{
			Name: "results",
			Columns: []*pquery.Column{
				{Name: "user_id", Type: "integer"},
			},
		},
		AggQuery: &pquery.Query{SQL: "SELECT * FROM results;"},
	}
	for i := 0; i < c; i++ {
		bucketSize := int(math.Ceil(float64(pSize) / float64(c)))
		pStart := i * bucketSize
		pEnd := pStart + bucketSize - 1
		q := &pquery.DBQuery{
			DB: db,
			Query: &pquery.Query{
				SQL:  sql,
				Args: []interface{}{pStart, pEnd},
			},
		}
		p.Parallel = append(p.Parallel, q)
	}
	p.Query(fn)
}

func querySQL(c int) string {
	var partitionCond string
	if c > 0 {
		partitionCond = `AND user_id % 1000 BETWEEN $1 AND $2`
	}
	// This is a bogus query that's returning all user ids that have have at
	// least 3 transactions in category 1 where each transaction is bigger than
	// than the previous one. It's meant to benefit from an index scan, which
	// postgres 9.6 can't execute in parallel. It's also meant to be an expensive
	// query, so I didn't attempt to optimize it.
	return `
SELECT user_id
FROM (
	SELECT
		user_id,
		dough > lag(dough) OVER w AS more_dough,
		row_number() OVER w
	FROM transactions
	WHERE
		category_id = 1
		` + partitionCond + ` 
	WINDOW w AS (PARTITION BY user_id ORDER BY id)
) s
WHERE row_number > 1
GROUP BY 1
HAVING
	bool_and(more_dough) = true
	AND count(*) >= 3;
`
}
