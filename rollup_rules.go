package main

import (
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql/parser"
)

// Configuration configurates a downsampler.
type Configuration struct {
	// Rules is a set of downsample rules. If set, this overrides any rules set
	// in the KV store (and the rules in KV store are not evaluated at all).
	Rules *RulesConfiguration `yaml:"rules"`
}

// RulesConfiguration is a set of rules configuration to use for downsampling.
type RulesConfiguration struct {
	// RollupRules are rollup rules that sets specific aggregations for sets
	// of metrics given a filter to match metrics against.
	RollupRules []RollupRuleConfiguration `yaml:"rollupRules"`
}

// RollupRuleConfiguration is a rollup rule configuration.
type RollupRuleConfiguration struct {
	// Filter is a space separated filter of label name to label value glob
	// patterns to which to filter the mapping rule.
	// e.g. "app:*nginx* foo:bar baz:qux*qaz*"
	Filter string `yaml:"filter"`

	// Transforms are a set of of rollup rule transforms.
	Transforms []TransformConfiguration `yaml:"transforms"`

	// Name is optional.
	Name string `yaml:"name"`
}

// TransformConfiguration is a rollup rule transform operation, only one
// single operation is allowed to be specified on any one transform configuration.
type TransformConfiguration struct {
	Rollup    *RollupOperationConfiguration    `yaml:"rollup"`
	Transform *TransformOperationConfiguration `yaml:"transform"`
}

// RollupOperationConfiguration is a rollup operation.
type RollupOperationConfiguration struct {
	// MetricName is the name of the new metric that is emitted after
	// the rollup is applied with its aggregations and group by's.
	MetricName string `yaml:"metricName"`

	// GroupBy is a set of labels to group by, only these remain on the
	// new metric name produced by the rollup operation.
	GroupBy []string `yaml:"groupBy"`

	// Aggregations is a set of aggregate operations to perform.
	Aggregations []string `yaml:"aggregations"`
}

// TransformOperationConfiguration is a transform operation.
type TransformOperationConfiguration struct {
	// Type is a transformation operation type.
	Type string `yaml:"type"`
}

var (
	emptyStruct   struct{}
	ValidAggTypes = map[string]struct{}{
		"last":   emptyStruct,
		"min":    emptyStruct,
		"max":    emptyStruct,
		"mean":   emptyStruct,
		"median": emptyStruct,
		"count":  emptyStruct,
		"sum":    emptyStruct,
		"sumsq":  emptyStruct,
		"stdev":  emptyStruct,
		"p10":    emptyStruct,
		"p20":    emptyStruct,
		"p30":    emptyStruct,
		"p40":    emptyStruct,
		"p50":    emptyStruct,
		"p60":    emptyStruct,
		"p70":    emptyStruct,
		"p80":    emptyStruct,
		"p90":    emptyStruct,
		"p95":    emptyStruct,
		"p99":    emptyStruct,
		"p999":   emptyStruct,
		"p9999":  emptyStruct,
	}
)

// func generateRollupRules(slowQueries []*queryStats) {
// 	for _, q := range slowQueries {
// 	}
// }

func queryToRollupRule(query string) *RollupRuleConfiguration {
	expr, err := parser.ParseExpr(query)
	if err != nil {
		// should never happen as it was validated already before
		return nil
	}

	valid := true
	rollupRule := &RollupRuleConfiguration{
		Name: query,
	}
	parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
		if node == nil || !valid {
			// Skip empty nodes.
			return nil
		}

		switch n := node.(type) {
		case *parser.Call:
			// for now we support only agg(rate(...)) combination
			if strings.EqualFold(n.Func.Name, "rate") && rollupRule != nil && len(rollupRule.Transforms) == 1 {
				rollupRule.Transforms[0].Rollup.MetricName += ":" + n.Func.Name
				rollupRule.Transforms = []TransformConfiguration{
					{
						Transform: &TransformOperationConfiguration{Type: "Increase"},
					},
					rollupRule.Transforms[0],
					{
						Transform: &TransformOperationConfiguration{Type: "Add"},
					},
				}
			} else {
				valid = false
				return nil
			}
		case *parser.AggregateExpr:
			// we support generation of a single aggregation for now
			if len(rollupRule.Transforms) != 0 {
				valid = false
				return nil
			}

			_, ok := ValidAggTypes[strings.ToLower(n.Op.String())]
			if !ok {
				valid = false
				return nil
			}
			rollupRule.Transforms = append(rollupRule.Transforms, TransformConfiguration{
				Rollup: &RollupOperationConfiguration{
					Aggregations: []string{n.Op.String()},
					MetricName:   strings.ToLower(n.Op.String()),
				},
			})

		case *parser.MatrixSelector:
			return nil

		case *parser.VectorSelector:
			if rollupRule == nil || len(rollupRule.Transforms) == 0 {
				valid = false
				return nil
			}
			for _, r := range rollupRule.Transforms {
				if r.Rollup != nil {
					r.Rollup.GroupBy = make([]string, 0, len(n.LabelMatchers))
					for _, l := range n.LabelMatchers {
						if l.Name == labels.MetricName {
							r.Rollup.MetricName += fmt.Sprintf(":%s", l.Value)
							rollupRule.Filter += fmt.Sprintf("%s:%s ", l.Name, l.Value)
							break
						}
					}
					for _, l := range n.LabelMatchers {
						if l.Name != labels.MetricName {
							r.Rollup.GroupBy = append(r.Rollup.GroupBy, l.Name)
							r.Rollup.MetricName += fmt.Sprintf(":%s:%s", l.Name, l.Value)
							rollupRule.Filter += fmt.Sprintf("%s:%s ", l.Name, l.Value)
						}
					}
					rollupRule.Filter = strings.Trim(rollupRule.Filter, " ")
					break
				}
			}

		default:
			valid = false
		}
		return nil
	})

	if !valid {
		return nil
	}

	return rollupRule
}
