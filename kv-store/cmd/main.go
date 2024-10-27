package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	kvstore "github.com/shyim/tanjun/kv-store"
	_ "modernc.org/sqlite"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	db, err := sql.Open("sqlite", "kv.db")
	if err != nil {
		panic(err)
	}

	_, _ = db.Exec(`PRAGMA journal_mode = WAL`)
	_, _ = db.Exec(`PRAGMA synchronous = NORMAL`)
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS secrets (\n                                       `key` TEXT NOT NULL,\n                                       `value` BLOB NOT NULL,\n                                       PRIMARY KEY (`key`)\n    );")

	if err != nil {
		panic(err)
	}

	for scanner.Scan() {
		var parsed kvstore.KVInput

		if err := json.Unmarshal(scanner.Bytes(), &parsed); err != nil {
			encodeResponse(kvstore.KVResponse{Type: "error", ErrorMessage: err.Error()})
			continue
		}

		if parsed.Operation == "del" {
			_, err = db.Exec("DELETE FROM secrets WHERE `key` = ?", parsed.Key)
			if err != nil {
				encodeResponse(kvstore.KVResponse{Type: "error", ErrorMessage: err.Error()})
				continue
			}

			encodeResponse(kvstore.KVResponse{Type: "success"})
		} else if parsed.Operation == "set" {
			_, err = db.Exec("REPLACE INTO secrets VALUES(?, ?)", parsed.Key, parsed.Value)
			if err != nil {
				encodeResponse(kvstore.KVResponse{Type: "error", ErrorMessage: err.Error()})
				continue
			}

			encodeResponse(kvstore.KVResponse{Type: "success"})
		} else if parsed.Operation == "get" {
			row := db.QueryRow("SELECT value FROM secrets WHERE key = ?", parsed.Key)

			if row.Err() != nil {
				encodeResponse(kvstore.KVResponse{Type: "error", ErrorMessage: err.Error()})
				continue
			}

			var dbVal string
			if err := row.Scan(&dbVal); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					encodeResponse(kvstore.KVResponse{Type: "success", Value: ""})
					continue
				}

				encodeResponse(kvstore.KVResponse{Type: "error", ErrorMessage: err.Error()})
				continue
			}

			encodeResponse(kvstore.KVResponse{Type: "success", Value: dbVal})
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		encodeResponse(kvstore.KVResponse{Type: "error", ErrorMessage: err.Error()})
	}
}

func encodeResponse(input kvstore.KVResponse) {
	bytes, err := json.Marshal(input)

	if err != nil {
		panic(err)
	}

	fmt.Println(string(bytes))
}
