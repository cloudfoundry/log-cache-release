package promql

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	metrics "code.cloudfoundry.org/go-metric-registry"

	"code.cloudfoundry.org/go-log-cache/v2/rpc/logcache_v1"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/storage"
)

type PromQL struct {
	r            DataReader
	log          *log.Logger
	queryTimeout time.Duration

	failureCounter    metrics.Counter
	instantQueryTimer metrics.Gauge
	rangeQueryTimer   metrics.Gauge

	result int64

	logcache_v1.UnimplementedPromQLQuerierServer
}

type DataReader interface {
	Read(ctx context.Context, in *logcache_v1.ReadRequest) (*logcache_v1.ReadResponse, error)
}

type Metrics interface {
	NewCounter(name, helpText string, opts ...metrics.MetricOption) metrics.Counter
	NewGauge(name, helpText string, opts ...metrics.MetricOption) metrics.Gauge
}

func New(
	r DataReader,
	m Metrics,
	log *log.Logger,
	queryTimeout time.Duration,
) *PromQL {
	q := &PromQL{
		r:            r,
		log:          log,
		queryTimeout: queryTimeout,
		failureCounter: m.NewCounter(
			"log_cache_promql_timeout",
			"Total number of errors while executing queries.",
		),

		//TODO convert to histograms
		instantQueryTimer: m.NewGauge(
			"log_cache_promql_instant_query_time",
			"Duration of last instant query in milliseconds.",
			metrics.WithMetricLabels(map[string]string{"unit": "milliseconds"}),
		),
		rangeQueryTimer: m.NewGauge(
			"log_cache_promql_range_query_time",
			"Duration of last range query in milliseconds.",
			metrics.WithMetricLabels(map[string]string{"unit": "milliseconds"}),
		),
		result: 1,
	}

	return q
}

func (q *PromQL) InstantQuery(ctx context.Context, req *logcache_v1.PromQL_InstantQueryRequest) (*logcache_v1.PromQL_InstantQueryResult, error) {
	var closureErr error
	interval := time.Second
	lcq := &logCacheQueryable{
		log:        q.log,
		interval:   interval,
		dataReader: q.r,

		// Prometheus does not hand us back the error the way you might
		// expect.  Therefore, we have to propagate the error back up
		// manually.
		errf: func(e error) { closureErr = e },
	}
	queryable := promql.NewEngine(promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    50000000,
		Timeout:       q.queryTimeout,
	})

	var requestTime time.Time
	var err error
	if req.Time == "" {
		requestTime = time.Now().Truncate(time.Second)
	} else {
		requestTime, err = ParseTime(req.Time)
		if err != nil {
			return nil, err
		}
	}

	qq, err := queryable.NewInstantQuery(lcq, req.Query, requestTime)
	if err != nil {
		return nil, err
	}

	queryStartTime := time.Now()
	r := qq.Exec(ctx)
	q.instantQueryTimer.Set(float64(time.Since(queryStartTime) / time.Millisecond))

	if closureErr != nil {
		q.failureCounter.Add(1)
		return nil, closureErr
	}

	return q.toInstantQueryResult(r)
}

func (q *PromQL) toInstantQueryResult(r *promql.Result) (*logcache_v1.PromQL_InstantQueryResult, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	switch r.Value.Type() {
	case promql.ValueTypeScalar:
		s := r.Value.(promql.Scalar)
		return &logcache_v1.PromQL_InstantQueryResult{
			Result: &logcache_v1.PromQL_InstantQueryResult_Scalar{
				Scalar: &logcache_v1.PromQL_Scalar{
					Time:  formatPromqlTime(s.T),
					Value: s.V,
				},
			},
		}, nil

	case promql.ValueTypeVector:
		var samples []*logcache_v1.PromQL_Sample
		for _, s := range r.Value.(promql.Vector) {
			metric := make(map[string]string)
			for _, m := range s.Metric {
				metric[m.Name] = m.Value
			}
			samples = append(samples, &logcache_v1.PromQL_Sample{
				Metric: metric,
				Point: &logcache_v1.PromQL_Point{
					Time:  formatPromqlTime(s.T),
					Value: s.V,
				},
			})
		}

		return &logcache_v1.PromQL_InstantQueryResult{
			Result: &logcache_v1.PromQL_InstantQueryResult_Vector{
				Vector: &logcache_v1.PromQL_Vector{
					Samples: samples,
				},
			},
		}, nil

	case promql.ValueTypeMatrix:
		var series []*logcache_v1.PromQL_Series
		for _, s := range r.Value.(promql.Matrix) {
			metric := make(map[string]string)
			for _, m := range s.Metric {
				metric[m.Name] = m.Value
			}
			var points []*logcache_v1.PromQL_Point
			for _, p := range s.Points {
				points = append(points, &logcache_v1.PromQL_Point{
					Time:  formatPromqlTime(p.T),
					Value: p.V,
				})
			}

			series = append(series, &logcache_v1.PromQL_Series{
				Metric: metric,
				Points: points,
			})
		}

		return &logcache_v1.PromQL_InstantQueryResult{
			Result: &logcache_v1.PromQL_InstantQueryResult_Matrix{
				Matrix: &logcache_v1.PromQL_Matrix{
					Series: series,
				},
			},
		}, nil

	default:
		q.log.Panicf("PromQL: unknown type: %s", r.Value.Type())
		return nil, nil
	}
}

func (q *PromQL) RangeQuery(ctx context.Context, req *logcache_v1.PromQL_RangeQueryRequest) (*logcache_v1.PromQL_RangeQueryResult, error) {
	var closureErr error
	interval := time.Second
	lcq := &logCacheQueryable{
		log:        q.log,
		interval:   interval,
		dataReader: q.r,

		// Prometheus does not hand us back the error the way you might
		// expect.  Therefore, we have to propagate the error back up
		// manually.
		errf: func(e error) { closureErr = e },
	}
	queryable := promql.NewEngine(promql.EngineOpts{
		MaxConcurrent: 10,
		MaxSamples:    50000000,
		Timeout:       q.queryTimeout,
	})

	step, err := ParseStep(req.Step)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse step: %s", err)
	}

	// TODO: Should there be some boundary checking on Start and End?
	startTime, err := ParseTime(req.Start)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse start: %s", err)
	}

	endTime, err := ParseTime(req.End)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse end: %s", err)
	}

	qq, err := queryable.NewRangeQuery(lcq, req.Query, startTime, endTime, step)
	if err != nil {
		return nil, err
	}

	queryStartTime := time.Now()
	r := qq.Exec(ctx)
	q.rangeQueryTimer.Set(float64(time.Since(queryStartTime) / time.Millisecond))

	if closureErr != nil {
		q.failureCounter.Add(1)
		return nil, closureErr
	}

	return q.toRangeQueryResult(r)
}

func (q *PromQL) toRangeQueryResult(r *promql.Result) (*logcache_v1.PromQL_RangeQueryResult, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	switch r.Value.Type() {
	case promql.ValueTypeMatrix:
		var series []*logcache_v1.PromQL_Series
		for _, s := range r.Value.(promql.Matrix) {
			metric := make(map[string]string)
			for _, m := range s.Metric {
				metric[m.Name] = m.Value
			}
			var points []*logcache_v1.PromQL_Point
			for _, p := range s.Points {
				points = append(points, &logcache_v1.PromQL_Point{
					Time:  formatPromqlTime(p.T),
					Value: p.V,
				})
			}

			series = append(series, &logcache_v1.PromQL_Series{
				Metric: metric,
				Points: points,
			})
		}

		return &logcache_v1.PromQL_RangeQueryResult{
			Result: &logcache_v1.PromQL_RangeQueryResult_Matrix{
				Matrix: &logcache_v1.PromQL_Matrix{
					Series: series,
				},
			},
		}, nil

	default:
		q.log.Panicf("PromQL: unknown type: %s", r.Value.Type())
		return nil, nil
	}
}

type logCacheQueryable struct {
	log        *log.Logger
	interval   time.Duration
	dataReader DataReader
	errf       func(error)
}

func (l *logCacheQueryable) Querier(ctx context.Context, mint int64, maxt int64) (storage.Querier, error) {
	return &LogCacheQuerier{
		log:        l.log,
		ctx:        ctx,
		start:      time.Unix(0, mint*int64(time.Millisecond)),
		end:        time.Unix(0, maxt*int64(time.Millisecond)),
		interval:   l.interval,
		dataReader: l.dataReader,
		errf:       l.errf,
	}, nil
}

type LogCacheQuerier struct {
	log        *log.Logger
	ctx        context.Context
	start      time.Time
	end        time.Time
	interval   time.Duration
	dataReader DataReader
	errf       func(error)
}

func (l *LogCacheQuerier) Select(params *storage.SelectParams, ll ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	var (
		metric string
		ls     []labels.Label
	)
	sourceIDs := make(map[string]struct{})
	for _, l := range ll {
		if l.Name == "__name__" {
			metric = l.Value
			continue
		}
		if l.Name == "source_id" {
			addSourceIDsFromLabelMatcher(sourceIDs, l)
			continue
		}
		ls = append(ls, labels.Label{
			Name:  l.Name,
			Value: l.Value,
		})
	}

	if len(sourceIDs) == 0 {
		err := fmt.Errorf("Metric '%s' does not have a 'source_id' label.", metric)
		l.errf(err)
		return nil, nil, err
	}

	builder := newSeriesBuilder()

	for sourceID := range sourceIDs {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		envelopeBatch, err := l.dataReader.Read(ctx, &logcache_v1.ReadRequest{
			SourceId:  sourceID,
			StartTime: l.start.Add(-time.Second).UnixNano(),
			EndTime:   l.end.UnixNano(),
			EnvelopeTypes: []logcache_v1.EnvelopeType{
				logcache_v1.EnvelopeType_GAUGE,
				logcache_v1.EnvelopeType_COUNTER,
				logcache_v1.EnvelopeType_TIMER,
			},
		})

		if err != nil {
			l.errf(err)
			return nil, nil, err
		}

		for _, e := range envelopeBatch.GetEnvelopes().GetBatch() {
			if !l.hasLabels(e.GetTags(), ls) {
				continue
			}

			var f float64
			switch e.Message.(type) {
			case *loggregator_v2.Envelope_Counter:
				if SanitizeMetricName(e.GetCounter().GetName()) != metric {
					continue
				}

				f = float64(e.GetCounter().GetTotal())
			case *loggregator_v2.Envelope_Gauge:
				value := checkMapForSanitizedMetricName(e.GetGauge(), metric)

				if value == nil {
					continue
				}

				f = value.GetValue()
			case *loggregator_v2.Envelope_Timer:
				if SanitizeMetricName(e.GetTimer().GetName()) != metric {
					continue
				}

				timer := e.GetTimer()
				f = float64(timer.GetStop() - timer.GetStart())
			default:
				continue
			}

			e.Timestamp = time.Unix(0, e.GetTimestamp()).Truncate(l.interval).UnixNano()

			tags := e.GetTags()
			if tags == nil {
				tags = make(map[string]string)
			}

			tags["source_id"] = e.SourceId
			if e.InstanceId != "" {
				tags["instance_id"] = e.InstanceId
			}

			builder.add(tags, point{
				t: e.GetTimestamp() / int64(time.Millisecond),
				v: f,
			})
		}
	}

	return builder.buildSeriesSet(), nil, nil
}

func checkMapForSanitizedMetricName(gauge *loggregator_v2.Gauge, metric string) *loggregator_v2.GaugeValue {
	metricsMap := gauge.GetMetrics()
	for k, v := range metricsMap {
		if SanitizeMetricName(k) == metric {
			return v
		}
	}
	return nil
}

func SanitizeMetricName(name string) string {
	// Forcefully convert all invalid separators to underscores
	// First character: Match the if it's NOT A-z or underscore ^[^A-z_]
	// All others: Match if they're NOT alphanumeric or understore [\W_]+?

	var re = regexp.MustCompile(`^[^A-z_]|[\W_]+?`)
	return re.ReplaceAllString(name, "_")
}

func convertToLabels(tags map[string]string) []labels.Label {
	ls := make([]labels.Label, 0, len(tags))
	for n, v := range tags {
		ls = append(ls, labels.Label{
			Name:  n,
			Value: v,
		})
	}
	return ls
}

func (l *LogCacheQuerier) hasLabels(tags map[string]string, ls []labels.Label) bool {
	for _, l := range ls {
		if v, ok := tags[l.Name]; !ok || v != l.Value {
			return false
		}
	}

	return true
}

func (l *LogCacheQuerier) LabelValues(name string) ([]string, storage.Warnings, error) {
	panic("not implemented")
}

func (l *LogCacheQuerier) LabelNames() ([]string, storage.Warnings, error) {
	panic("not implemented")
}

func (l *LogCacheQuerier) Close() error {
	return nil
}

// concreteSeriesSet implements storage.SeriesSet.
type concreteSeriesSet struct {
	cur    int
	series []storage.Series
}

func (c *concreteSeriesSet) Next() bool {
	c.cur++
	return c.cur-1 < len(c.series)
}

func (c *concreteSeriesSet) At() storage.Series {
	return c.series[c.cur-1]
}

func (c *concreteSeriesSet) Err() error {
	return nil
}

// concreteSeries implements storage.Series.
type concreteSeries struct {
	labels labels.Labels
	points []point
}

type point struct {
	t int64
	v float64
}

func (c *concreteSeries) Labels() labels.Labels {
	return labels.New(c.labels...)
}

func (c *concreteSeries) Iterator() storage.SeriesIterator {
	return newConcreteSeriersIterator(c)
}

// concreteSeriesIterator implements storage.SeriesIterator.
type concreteSeriesIterator struct {
	cur    int
	series *concreteSeries
}

func newConcreteSeriersIterator(series *concreteSeries) storage.SeriesIterator {
	return &concreteSeriesIterator{
		cur:    -1,
		series: series,
	}
}

// Seek implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Seek(t int64) bool {
	c.cur = sort.Search(len(c.series.points), func(n int) bool {
		return c.series.points[n].t >= t
	})
	return c.cur < len(c.series.points)
}

// At implements storage.SeriesIterator.
func (c *concreteSeriesIterator) At() (t int64, v float64) {
	s := c.series.points[c.cur]

	return s.t, s.v
}

// Next implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Next() bool {
	c.cur++
	return c.cur < len(c.series.points)
}

// Err implements storage.SeriesIterator.
func (c *concreteSeriesIterator) Err() error {
	return nil
}

type seriesData struct {
	tags   map[string]string
	points []point
}

func newSeriesBuilder() *seriesSetBuilder {
	return &seriesSetBuilder{
		data: make(map[string]seriesData),
	}
}

type seriesSetBuilder struct {
	data map[string]seriesData
}

func (b *seriesSetBuilder) add(tags map[string]string, s point) {
	seriesID := b.getSeriesID(tags)
	d, ok := b.data[seriesID]

	if !ok {
		b.data[seriesID] = seriesData{
			tags:   tags,
			points: make([]point, 0),
		}

		d = b.data[seriesID]
	}

	d.points = append(d.points, s)
	b.data[seriesID] = d
}

func (b *seriesSetBuilder) getSeriesID(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var seriesID string
	for _, k := range keys {
		seriesID = seriesID + "-" + k + "-" + tags[k]
	}

	return seriesID
}

func (b *seriesSetBuilder) buildSeriesSet() storage.SeriesSet {
	set := &concreteSeriesSet{
		series: []storage.Series{},
	}

	for _, v := range b.data {
		set.series = append(set.series, &concreteSeries{
			labels: convertToLabels(v.tags),
			points: v.points,
		})
	}

	return set
}

// TODO - move elsewhere and clean up [#160353522]
func ExtractSourceIds(query string) ([]string, error) {
	expr, err := promql.ParseExpr(query)
	if err != nil {
		return nil, err
	}

	visitor := newSourceIDVisitor()

	err = promql.Walk(
		visitor,
		expr,
		nil,
	)
	if err != nil {
		return nil, err
	}

	var sourceIDs []string

	for sourceID := range visitor.sourceIDs {
		sourceIDs = append(sourceIDs, sourceID)
	}

	return sourceIDs, nil
}

type sourceIDVisitor struct {
	sourceIDs map[string]struct{}
}

func newSourceIDVisitor() *sourceIDVisitor {
	return &sourceIDVisitor{
		sourceIDs: make(map[string]struct{}),
	}
}

func (s *sourceIDVisitor) Visit(node promql.Node, _ []promql.Node) (promql.Visitor, error) {
	if node == nil {
		return nil, nil
	}

	switch selector := node.(type) {
	case *promql.VectorSelector:
		s.addSourceIDsFromMatchers(selector.LabelMatchers)
	case *promql.MatrixSelector:
		s.addSourceIDsFromMatchers(selector.LabelMatchers)
	}

	return s, nil
}

func (s *sourceIDVisitor) addSourceIDsFromMatchers(labelMatchers []*labels.Matcher) {
	for _, labelMatcher := range labelMatchers {
		if labelMatcher.Name == "source_id" {
			addSourceIDsFromLabelMatcher(s.sourceIDs, labelMatcher)
		}
	}
}

func addSourceIDsFromLabelMatcher(sourceIDs map[string]struct{}, labelMatcher *labels.Matcher) {
	switch labelMatcher.Type {
	case labels.MatchRegexp:
		matchedSourceIDs := strings.Split(labelMatcher.Value, "|")
		for _, matchedSourceID := range matchedSourceIDs {
			sourceIDs[matchedSourceID] = struct{}{}
		}
	case labels.MatchEqual:
		sourceIDs[labelMatcher.Value] = struct{}{}
	}
}

func ReplaceSourceIdSets(query string, sourceIDExpansions map[string][]string) (string, error) {
	expr, err := promql.ParseExpr(query)
	if err != nil {
		return "", err
	}

	visitor := newSourceIdReplacementVisitor(sourceIDExpansions)

	err = promql.Walk(
		visitor,
		expr,
		nil,
	)
	if err != nil {
		return "", err
	}

	return expr.String(), nil
}

type sourceIdReplacementVisitor struct {
	sourceIdSets map[string][]string
}

func newSourceIdReplacementVisitor(sourceIdSets map[string][]string) *sourceIdReplacementVisitor {
	return &sourceIdReplacementVisitor{
		sourceIdSets: sourceIdSets,
	}
}

func (s *sourceIdReplacementVisitor) Visit(node promql.Node, _ []promql.Node) (promql.Visitor, error) {
	if node == nil {
		return nil, nil
	}

	switch selector := node.(type) {
	case *promql.VectorSelector:
		s.replaceInMatchers(selector.LabelMatchers)
	case *promql.MatrixSelector:
		s.replaceInMatchers(selector.LabelMatchers)
	}

	return s, nil
}

func (s *sourceIdReplacementVisitor) replaceInMatchers(labelMatchers []*labels.Matcher) {
	for _, labelMatcher := range labelMatchers {
		if labelMatcher.Name == "source_id" {
			switch labelMatcher.Type {
			case labels.MatchEqual:
				s.replaceSourceIdsInEqualMatcher(labelMatcher)
			case labels.MatchRegexp:
				s.replaceSourceIdsInRegexpMatcher(labelMatcher)
			}
		}
	}
}

func (s *sourceIdReplacementVisitor) replaceSourceIdsInEqualMatcher(labelMatcher *labels.Matcher) {
	expansions, ok := s.sourceIdSets[labelMatcher.Value]

	if ok {
		if len(expansions) > 1 {
			labelMatcher.Type = labels.MatchRegexp
		} else {
			labelMatcher.Type = labels.MatchEqual
		}
		labelMatcher.Value = strings.Join(expansions, "|")
	}
}

func (s *sourceIdReplacementVisitor) replaceSourceIdsInRegexpMatcher(labelMatcher *labels.Matcher) {
	var expansions []string

	startingSourceIds := strings.Split(labelMatcher.Value, "|")

	for _, sourceId := range startingSourceIds {
		sourceIdExpansions := s.sourceIdSets[sourceId]
		expansions = append(expansions, sourceIdExpansions...)
	}

	if len(expansions) > 1 {
		labelMatcher.Type = labels.MatchRegexp
	} else {
		labelMatcher.Type = labels.MatchEqual
	}
	labelMatcher.Value = strings.Join(expansions, "|")
}

func formatPromqlTime(timeInMillis int64) string {
	return fmt.Sprintf("%.3f", float64(timeInMillis)/1000)
}
