package orm

import (
	"context"
	"github.com/recoweft/goquent/orm/observation"
	"os"
	"testing"
)

func BenchmarkObservationEnabled(b *testing.B) {
	for _, mode := range []string{"collector", "budget", "repeat", "drain"} {
		b.Run(mode, func(b *testing.B) {
			db := observationBenchDB(b)
			limit := int64(16)
			o := observation.Options{}
			if mode == "budget" {
				o.QueryLimit = &limit
			}
			if mode == "repeat" {
				o.DetectRepeated = true
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, s, e := observation.Start(context.Background(), o)
				if e != nil {
					b.Fatal(e)
				}
				observationBenchRequest(b, db, ctx)
				s.Finish()
				if mode == "drain" {
					_, _, _ = s.Drain(func(observation.Event) {})
				}
			}
		})
	}
}

// Native latency includes the local connection pool, ORM, driver and server.
// It is deliberately separate from the mock CPU/allocation benchmark above.
func BenchmarkObservationNative(b *testing.B) {
	for _, dialect := range []string{MySQL, Postgres} {
		b.Run(dialect, func(b *testing.B) {
			key := "TEST_MYSQL_DSN"
			if dialect == Postgres {
				key = "TEST_POSTGRES_DSN"
			}
			dsn := os.Getenv(key)
			if dsn == "" {
				b.Skip("explicit native source absent")
			}
			db, e := OpenWithDriver(dialect, dsn)
			if e != nil {
				b.Fatal("native unavailable")
			}
			defer db.Close()
			db = db.RequireRawApproval("native benchmark")
			for _, enabled := range []bool{false, true} {
				name := "disabled"
				if enabled {
					name = "collector"
				}
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						ctx := context.Background()
						var s *observation.Scope
						if enabled {
							ctx, s, e = observation.Start(ctx, observation.Options{})
							if e != nil {
								b.Fatal(e)
							}
						}
						var n int
						if db.QueryRowContext(ctx, "SELECT 1").Scan(&n) != nil || n != 1 {
							b.Fatal("native query")
						}
						if s != nil {
							s.Finish()
						}
					}
				})
			}
		})
	}
}
