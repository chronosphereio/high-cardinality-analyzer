package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/pborman/getopt"
	"github.com/prometheus/prometheus/promql/parser"
	"gopkg.in/yaml.v2"
)

type Query struct {
	Params *QueryParams
	Stats  *QueryStats
}

type QueryParams struct {
	Query string
}

type QueryStats struct {
	Timings *QueryStatsTimings
}

type QueryStatsTimings struct {
	EvalTotalTime        float64
	ResultSortTime       float64
	QueryPreparationTime float64
	InnerEvalTime        float64
	ExecQueueTime        float64
	ExecTotalTime        float64
}

type queryStats struct {
	query    string
	tookTime float64
	count    uint32
}

func main() {

	var (
		optPathToQueryLog         = getopt.StringLong("query-log", 'l', "", "Path to query log file [e.g. query.log]")
		optMinQueryTime           = getopt.DurationLong("min-query-time", 'm', 0, "Min query duration time to include in a result")
		optMinQueryCount          = getopt.Uint32Long("min-query-count", 'c', 0, "Min query count it should reappear in the log to include in a result")
		optGenerateRecordingRules = getopt.BoolLong("generate-recording-rules", 'r', "Generate recording rules and output")
		optGenerateRollupRules    = getopt.BoolLong("generate-rollup-rules", 'g', "Generate rollup rules and output")
		slowQueries               = make(map[string]*queryStats, 100)
	)
	getopt.Parse()

	if *optPathToQueryLog == "" {
		getopt.Usage()
		os.Exit(1)
	}

	logFile, err := os.Open(*optPathToQueryLog)
	if err != nil {
		log.Fatalf("Error opening query log file: %v\n", err)
	}
	defer logFile.Close()

	reader := bufio.NewReader(logFile)
	var line string
	lineNo := 1
	for {
		line, err = reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("error reading json line %d: %v", lineNo, err)
			continue
		}

		query := &Query{}
		err = json.Unmarshal([]byte(line), query)
		if err != nil {
			log.Errorf("error unmarshaling json line %d: %v", lineNo, err)
			continue
		}

		qd, err := time.ParseDuration(fmt.Sprintf("%fs", query.Stats.Timings.InnerEvalTime))
		if err != nil {
			log.Errorf("error parsing query duration '%f': %v", query.Stats.Timings.InnerEvalTime, err)
			continue
		}
		if qd < *optMinQueryTime {
			continue
		}

		log.Debugf("Query: %s, Took time: %fs\n", query.Params.Query, query.Stats.Timings.InnerEvalTime)
		expr, err := parser.ParseExpr(query.Params.Query)
		if err != nil {
			log.Errorf("error parsing query at line %d: %v", lineNo, err)
			continue
		}

		parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
			if node == nil {
				// Skip empty nodes.
				return nil
			}

			switch n := node.(type) {
			case *parser.AggregateExpr:
				qs := n.String()
				q, ok := slowQueries[qs]
				if ok {
					q.count++
					// If the whole query has more operations then tookTime will be incorrect for only
					// a single aggregation operation, but lets ignore it for now.
					q.tookTime += query.Stats.Timings.InnerEvalTime
				} else {
					slowQueries[qs] = &queryStats{
						query:    qs,
						count:    1,
						tookTime: query.Stats.Timings.InnerEvalTime,
					}
				}
			}
			return nil
		})

		lineNo++
	}

	arr := make([]*queryStats, 0, len(slowQueries))
	for _, v := range slowQueries {
		if v.count >= *optMinQueryCount {
			arr = append(arr, v)
		}
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].tookTime > arr[j].tookTime
	})

	if *optGenerateRecordingRules {
		fmt.Println("groups:")
		fmt.Println("- name: high_cardinality_analyzer_group")
		fmt.Println("  rules:")
		for _, v := range arr {
			fmt.Println("  - record:", toRuleName(v.query))
			fmt.Println("    expr:", v.query)
		}
		return
	}

	if *optGenerateRollupRules {
		failedQueries := make([]string, 0, len(arr))
		cfg := &Configuration{
			Rules: &RulesConfiguration{
				RollupRules: make([]RollupRuleConfiguration, 0, len(arr)),
			},
		}
		for _, v := range arr {
			r := queryToRollupRule(v.query)
			if r != nil {
				cfg.Rules.RollupRules = append(cfg.Rules.RollupRules, *r)
			} else {
				failedQueries = append(failedQueries, v.query)
			}
		}

		y, err := yaml.Marshal(cfg)
		if err != nil {
			fmt.Printf("Failed to serialize rules to YAML: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", string(y))

		if len(failedQueries) > 0 {
			fmt.Println("\nFailed to convert the following queries:")
			for _, s := range failedQueries {
				fmt.Println(s)
			}
		}
		return
	}

	for _, v := range arr {
		fmt.Printf("%s, count: %d, took total: %fs\n", v.query, v.count, v.tookTime)
	}
}

var re = regexp.MustCompile(`[^\w]`)

func toRuleName(expr string) string {
	s := re.ReplaceAllLiteralString(expr, ":")
	s = strings.ReplaceAll(s, "::", ":")
	for strings.HasPrefix(s, ":") {
		s = s[1:]
	}
	for strings.HasSuffix(s, ":") {
		s = s[0 : len(s)-1]
	}
	return s
}
