# pquery

This repo contains the example code for my presentation called "DYI Parallel
Queries"

The code *is not* intended to be used in a library as is. It might be buggy,
there are no docs, and I won't continue developing it.

That being said, I'm releasing it under the PUBLIC DOMAIN. Do with it as you
wish.

## Run the example

Hint: There is `-dsn` flag if your postgres server is not running on localhost
with the default port and postgres user.

Initialize the database with e.g. 10.000.000 rows:

```bash
$ go run example/*.go -i -s 100
2016/11/17 17:30:44 Creating schema ...
2016/11/17 17:30:44 Generating 10000000 transactions ...
2016/11/17 17:32:42 Creating index ...
2016/11/17 17:33:24 Running vacuum analyze ...
```

Run the query in it's non-parallel version:

```bash
$ go run example/*.go -c 0
2016/11/17 17:33:42 Executing sql with concurrency 0

SELECT user_id
FROM (
	SELECT
		user_id,
		dough > lag(dough) OVER w AS more_dough,
		row_number() OVER w
	FROM transactions
	WHERE
		category_id = 1

	WINDOW w AS (PARTITION BY user_id ORDER BY id)
) s
WHERE row_number > 1
GROUP BY 1
HAVING
	bool_and(more_dough) = true
	AND count(*) >= 3;

2016/11/17 17:33:50 6093 results in 8.351718019s
```

And compare with the parallel execution:

```bash
$ go run example/*.go -c 8
2016/11/17 17:34:44 Executing sql with concurrency 8

SELECT user_id
FROM (
	SELECT
		user_id,
		dough > lag(dough) OVER w AS more_dough,
		row_number() OVER w
	FROM transactions
	WHERE
		category_id = 1
		AND user_id % 1000 BETWEEN $1 AND $2
	WINDOW w AS (PARTITION BY user_id ORDER BY id)
) s
WHERE row_number > 1
GROUP BY 1
HAVING
	bool_and(more_dough) = true
	AND count(*) >= 3;


CREATE TEMP TABLE results (
user_id integer
) ON COMMIT DROP

2016/11/17 17:34:47 6093 results in 2.48483626s
```
