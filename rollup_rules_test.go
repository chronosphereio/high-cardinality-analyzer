package main

import (
	"testing"

	"github.com/tj/assert"
)

func TestRollupRules(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedRule *RollupRuleConfiguration
	}{
		{
			name:  "sum rate should convert",
			query: "sum(rate(query_duration_seconds[1m]))",
			expectedRule: &RollupRuleConfiguration{
				Name:   "sum(rate(query_duration_seconds[1m]))",
				Filter: "__name__:query_duration_seconds",
				Transforms: []TransformConfiguration{
					{Transform: &TransformOperationConfiguration{Type: "Increase"}},
					{
						Rollup: &RollupOperationConfiguration{
							MetricName:   "sum:rate:query_duration_seconds",
							GroupBy:      []string{},
							Aggregations: []string{"sum"},
						},
					},
					{Transform: &TransformOperationConfiguration{Type: "Add"}},
				},
			},
		},
		{
			name:  "sum rate and tags should convert with tags",
			query: "sum(rate(query_duration_seconds{tag1=\"foo\",tag2=\"bar\"}[1m]))",
			expectedRule: &RollupRuleConfiguration{
				Name:   "sum(rate(query_duration_seconds{tag1=\"foo\",tag2=\"bar\"}[1m]))",
				Filter: "__name__:query_duration_seconds tag1:foo tag2:bar",
				Transforms: []TransformConfiguration{
					{Transform: &TransformOperationConfiguration{Type: "Increase"}},
					{
						Rollup: &RollupOperationConfiguration{
							MetricName:   "sum:rate:query_duration_seconds:tag1:foo:tag2:bar",
							GroupBy:      []string{"tag1", "tag2"},
							Aggregations: []string{"sum"},
						},
					},
					{Transform: &TransformOperationConfiguration{Type: "Add"}},
				},
			},
		},
		{
			name:  "sum should convert",
			query: "sum(query_count)",
			expectedRule: &RollupRuleConfiguration{
				Name:   "sum(query_count)",
				Filter: "__name__:query_count",
				Transforms: []TransformConfiguration{
					{
						Rollup: &RollupOperationConfiguration{
							MetricName:   "sum:query_count",
							GroupBy:      []string{},
							Aggregations: []string{"sum"},
						},
					},
				},
			},
		},
		{
			name:         "sum max ir not supported yet",
			query:        "sum(max(query_count))",
			expectedRule: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := queryToRollupRule(tt.query)
			assert.EqualValues(t, tt.expectedRule, rule)
		})
	}
}
