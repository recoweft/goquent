package profiling

import (
	"log"
	"time"
)

func Profile(query string, executeFunc func()) {
	start := time.Now()
	executeFunc()
	elapsed := time.Since(start)
	log.Printf("Query executed in %s", elapsed)
}
