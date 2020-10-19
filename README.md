# Hight Cardinality Analyzer

That is a simple tool to help you analyze your Prometheus Query Log and generate
recording rules according to total query time. It looks for aggregation functions
and generates a template for a recording rule.

For more information about recording rules, please follow: https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/

For more information about how to enable query log: https://prometheus.io/docs/guides/query-log/

## Building
To build it simply call `go build .`

## Running
To analyze query log and show queries which in total (including all same queries) took longer than 50 seconds:

`./high-cardinality-analyzer --query-log query.log --min-query-time 50s`

To generate recording rules for queries which in total took longer than 50 seconds and where used at least 10 times:

`./high-cardinality-analyzer --query-log query.log --min-query-time 50s --min-query-count 10 --generate-recording-rules`

## Future development
- Support of M3 query log and rules
- Filter queries by a percent of aggregated records. For example, we would like to see long taking queries which
data points count were reduced 100 or more times (for example from 10k to 100). Such queries are potentially are good candidates for optimization using recording rules.