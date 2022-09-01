package executionplan_test

import (
	"context"
	"fmt"
	"fpetkovski/promql-engine/executionplan"
	"testing"
	"time"

	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/stretchr/testify/require"
)

func BenchmarkExecutionPlan(b *testing.B) {
	test := setupStorage(b)
	defer test.Close()

	start := time.Unix(0, 0)
	end := start.Add(1 * time.Hour)
	step := time.Second * 30

	cases := []struct {
		name  string
		query string
	}{
		{
			name:  "vector selector",
			query: "http_requests_total",
		},
		{
			name:  "aggregation",
			query: "sum by (pod) (http_requests_total)",
		},
		{
			name:  "sum-rate",
			query: "sum by (pod) (rate(http_requests_total[1m]))",
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.Run("current_engine", func(b *testing.B) {
				opts := promql.EngineOpts{
					Logger:     nil,
					Reg:        nil,
					MaxSamples: 50000000,
					Timeout:    100 * time.Second,
				}
				engine := promql.NewEngine(opts)

				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					qry, err := engine.NewRangeQuery(test.Queryable(), nil, tc.query, start, end, step)
					require.NoError(b, err)

					res := qry.Exec(test.Context())
					require.NoError(b, res.Err)
				}
			})
			b.Run("new_engine", func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					expr, err := parser.ParseExpr(tc.query)
					require.NoError(b, err)

					p, err := executionplan.New(expr, test.Storage(), start, end, step)
					require.NoError(b, err)

					out, err := p.Next(context.Background())
					require.NoError(b, err)
					for range out {
					}
				}
			})
		})
	}
}

func setupStorage(b *testing.B) *promql.Test {
	load := synthesizeLoad(1000, 3)
	test, err := promql.NewTest(b, load)
	require.NoError(b, err)
	require.NoError(b, test.Run())

	return test
}

func synthesizeLoad(numPods, numContainers int) string {
	load := `
load 30s`
	for i := 0; i < numPods; i++ {
		for j := 0; j < numContainers; j++ {
			load += fmt.Sprintf(`
  http_requests_total{pod="p%d", container="c%d"} %d+%dx720`, i, j, i, j)
		}
	}

	return load
}
