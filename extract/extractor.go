package extract

import (
	"../etl"
	"../monitor"
	"../types"

	"crypto/rand"
	"encoding/base64"

	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"strconv"
)

func rand_str(str_size int) string {
	c := 30
	b := make([]byte, c)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal("Failt to gen random with error:", err)
	}
	s := base64.URLEncoding.EncodeToString(b)
	return s
}

//Run fetches data and put into channel for processing
func Run(etlSession *etl.Session, db *sqlx.DB) {
	extractChannel := etlSession.ExtractChannel
	table := etlSession.Get("table")

	defer etlSession.Wg.Done()
	defer close(extractChannel)

	offset := monitor.GetTableProgress(table)
	limit, _ := strconv.Atoi(etlSession.Config("PG_FETCH_LIMIT"))
	batch := 0
	rowCount := 0

	//for {
	//batch += 1
	//log.Printf("Fetch batch: %d. Params: offset %d, limit %d", batch, offset, limit)
	query := types.Query(table)
	query = strings.Replace(query, "[START_TIMESTAMP_IN_PACIFIC_TIME]", etlSession.Get("[START_TIMESTAMP_IN_PACIFIC_TIME]"), -1)
	query = strings.Replace(query, "[END_TIMESTAMP_IN_PACIFIC_TIME]", etlSession.Get("[END_TIMESTAMP_IN_PACIFIC_TIME]"), -1)
	if scope := etlSession.Get("scope"); scope != "" {
		query = fmt.Sprintf("%s AND %s", query, etlSession.Get("scope"))
	}
	log.Println("Extract data query: ", query)

	rows, err := db.Queryx(query)
	if !rows.Next() {
		log.Printf("No more rows to do. Stop at offset %d, Row Count: %d", offset, rowCount)
		return
		//break
	}
	if err != nil {
		log.Fatal(err)
	}

	for {
		row := make(map[string]interface{})
		err = rows.MapScan(row)
		//log.Printf("Row= %#v\n\n", row)

		if err != nil {
			log.Fatalln(err)
		}

		// fake id for benchmark helping when we don't have lot of data
		// row["git_id"] = []uint8(rand_str(40))

		rowCount += 1
		extractChannel <- row
		if !rows.Next() {
			break
		}

		if rowCount%limit == 0 {
			batch = rowCount / limit
			monitor.Report(table, batch)
			log.Printf("Extracted: %d rows", rowCount)
		}
	}
	log.Printf("No more rows to do. Stop at row count: %d", rowCount)

	//offset = offset + limit
	//}
}
