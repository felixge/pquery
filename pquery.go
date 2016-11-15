package pquery

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

type Plan struct {
	Parallel []*DBQuery
	AggDB    *sql.DB
	AggTable *Table
	AggQuery *Query
}

func (p *Plan) Query(fn func(*sql.Rows, error)) {
	newExecution(p).Query(fn)
}

func newExecution(p *Plan) *execution {
	return &execution{
		p:      p,
		rowCh:  make(chan []interface{}),
		errCh:  make(chan error),
		doneCh: make(chan struct{}),
	}
}

type execution struct {
	p      *Plan
	rowCh  chan []interface{}
	errCh  chan error
	doneCh chan struct{}
}

func (e *execution) Query(fn func(*sql.Rows, error)) {
	defer func() { close(e.doneCh) }()
	e.parallelQueries()
	t := e.p.AggTable
	tx, err := e.p.AggDB.Begin()
	if err != nil {
		fn(nil, err)
		return
	}
	defer tx.Rollback()
	fmt.Printf("%s\n", t.sql())
	if _, err := tx.Exec(t.sql()); err != nil {
		fn(nil, err)
		return
	}
	stmt, err := tx.Prepare(pq.CopyIn(t.Name, t.columnNames()...))
	if err != nil {
		fn(nil, err)
		return
	}
	defer stmt.Close()
	for count := 0; count < len(e.p.Parallel); {
		select {
		case row := <-e.rowCh:
			if row == nil {
				count++
			} else if _, err := stmt.Exec(row...); err != nil {
				fn(nil, err)
				return
			}
		case err := <-e.errCh:
			fn(nil, err)
			return
		}
	}
	if _, err := stmt.Exec(); err != nil {
		fn(nil, err)
		return
	}
	rows, err := tx.Query(e.p.AggQuery.SQL, e.p.AggQuery.Args...)
	fn(rows, err)
}

func (e *execution) parallelQueries() {
	for _, q := range e.p.Parallel {
		go func(q *DBQuery) {
			rows, err := q.DB.Query(q.Query.SQL, q.Query.Args...)
			if err != nil {
				e.sendErr(err)
				return
			}
			columns, err := rows.Columns()
			if err != nil {
				e.sendErr(err)
				return
			}
			defer rows.Close()
			for rows.Next() {
				row := make([]interface{}, len(columns))
				for i := range row {
					var val interface{}
					row[i] = &val
				}
				if err := rows.Scan(row...); err != nil {
					e.sendErr(err)
					return
				}
				e.sendRow(row)
			}
			if err := rows.Err(); err != nil {
				e.sendErr(err)
				return
			}
			e.sendRow(nil)
		}(q)
	}
}

func (e *execution) sendErr(err error) {
	select {
	case e.errCh <- err:
	case <-e.doneCh:
	}
}

func (e *execution) sendRow(row []interface{}) {
	select {
	case e.rowCh <- row:
	case <-e.doneCh:
	}
}

type DBQuery struct {
	DB    *sql.DB
	Query *Query
}

type Query struct {
	SQL  string
	Args []interface{}
}

type Table struct {
	Name    string
	Columns []*Column
}

// sql returns a sql string that creates the table as a temporary table.
func (t *Table) sql() string {
	columns := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		columns[i] = c.Name + " " + c.Type
	}
	sql := `
CREATE TEMP TABLE ` + t.Name + ` (` + `
` + strings.Join(columns, ",\n") + `
) ON COMMIT DROP
`
	return sql
}

func (t *Table) columnNames() []string {
	names := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		names[i] = c.Name
	}
	return names
}

type Column struct {
	Name string
	Type string
}
