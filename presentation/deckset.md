theme: Sketchnote, 1
footer: @felixge · PostgreSQL Meetup Berlin · Nov 17, 2016
slidenumbers: true

# [fit] DYI Parallel Queries

---

> Doesn't 9.6 already support parallel query execution?

---

# Doesn't 9.6 already support parallel query execution?

Yes, but ...

* Only when driven by sequential scan node 🙈
* Only works for "supported aggregates" 🤕
* Doesn't work across multiple machines 😕
* You might be stuck on an old version of postgres 😱

---

> So, I think we can all agree that it would be pretty awesome to skip 4 years
ahead.

---

# [fit] What do we need?

---

# A slow aggregate query driven by a index scan node

```sql
SELECT user_id
FROM (
	SELECT
		user_id,
		dough > lag(dough) OVER w AS more_dough,
		row_number() OVER w
	FROM transactions
	WHERE category_id = 1
	WINDOW w AS (PARTITION BY user_id ORDER BY id)
) s
WHERE row_number > 1
GROUP BY 1
HAVING
	bool_and(more_dough) = true
	AND count(*) >= 3;
```

---

# A slow aggregate query driven by a index scan node

```
GroupAggregate  (...) (...)
  Group Key: s.user_id
  Filter: (bool_and(s.more_dough) AND (count(*) >= 3))
  Rows Removed by Filter: 706445
  ->  Subquery Scan on s  (...) (...)
        Filter: (s.row_number > 1)
        Rows Removed by Filter: 917968
        ->  WindowAgg  (...) (...)
              ->  Sort  (...) (...)
                    Sort Key: transactions.user_id, transactions.id
                    Sort Method: quicksort  Memory: 204569kB
                    ->  Bitmap Heap Scan on transactions  (...) (...)
                          Recheck Cond: (category_id = 1)
                          Heap Blocks: exact=63695
                          ->  Bitmap Index Scan on transactions_partition_idx  (...) (...)
                                Index Cond: (category_id = 1)
Planning time: 0.564 ms
Execution time: 9302.230 ms
```


---

# A programming language with decent concurrency

**Probably:** Go, Node.js, Java, ...

**Maybe:** Twisted Python, Event Machine (Ruby)

**Perhaps not:** PHP

---

# A small modification to your index

E.g. changing this index:

```sql
CREATE INDEX ON transactions(category_id);
```

to this index:

```sql
CREATE INDEX ON transactions(category_id, (user_id % 1000));
```

Allows us to quickly retrieve partition ranges from our index.

---

# Executing a slightly modified version of our query concurrently

I.e. instead of execution the previous query with:

```go
WHERE category_id = 1
```

We execute e.g. 4 (number of CPUs) queries like this:

```go
WHERE category_id = 1 AND user_id % 4 BETWEEN 0 AND 249
WHERE category_id = 1 AND user_id % 4 BETWEEN 250 AND 499
WHERE category_id = 1 AND user_id % 4 BETWEEN 500 AND 749
WHERE category_id = 1 AND user_id % 4 BETWEEN 750 AND 999
```

---

# A temporary table

The table needs to have the same columns that the query produces, e.g.:

```sql
CREATE TEMP TABLE results (
  user_id integer
) ON COMMIT DROP;
```

Then, as results from the concurrent query come in, insert them into the temporary table. Ideally using `COPY`.

---

# And finally, a new query to select from the tmp table

Can be as simple as:

```sql
SELECT * FROM results;
```

Or in more complex cases even perform some more aggregations of its own.

---

# Benchmark (Toy Example)

On my quad core mid 2012 MBP:

* Original query: ~8s
* DYI Parallel Query (c=8): ~2.5s

**3.2x faster 🎉**

---

# Benchmark (Real World)

Enterprisy server with 24 cores:

* Original query: ~30s
* DYI Parallel Query (c=24): ~3s

**10x faster 🎉**

---

> This should also scale fairly well across multiple machines.

---

# Alternatives

* Stored procedures for pre-computing results when possible ❤️
* CitusDB, Postgres-XL, Redshift, ...

---

# Code for the toy example is available at:

[github.com/felixge/pquery]()

---

# Get in touch


