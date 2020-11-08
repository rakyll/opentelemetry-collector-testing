package promreceiver

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/receiver/promreceiver/internal"
	"go.opentelemetry.io/collector/translator/internaldata"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
)

var txidSeq int64

type collector struct {
	sink          consumer.MetricsConsumer
	scrapeManager *scrape.Manager
}

func (c *collector) Appender(ctx context.Context) storage.Appender {
	return &tx{
		ctx:   ctx,
		sink:  c.sink,
		cache: newCache(c.scrapeManager),
	}
}

type tx struct {
	ctx   context.Context
	sink  consumer.MetricsConsumer
	cache *cache

	node     *commonpb.Node
	resource *resourcepb.Resource
}

func (tt *tx) Add(l labels.Labels, t int64, v float64) (uint64, error) {
	err := tt.cache.Add(l, t, v)
	return 0, err
}

func (tt *tx) AddFast(ref uint64, t int64, v float64) error {
	return storage.ErrNotFound
}

func (tt *tx) Commit() error {
	if len(tt.cache.metrics) == 0 {
		return nil
	}

	// var metrics []*v1.Metric
	// for _, m := range tt.cache.metrics {
	// 	metric, _, _, err := m.Build()
	// 	if err != nil {
	// 		metrics = append(metrics, metric...)
	// 	}
	// }
	md := consumerdata.MetricsData{
		Node:     tt.node,
		Resource: tt.resource,
		// Metrics:  metrics,
	}
	return tt.sink.ConsumeMetrics(tt.ctx, internaldata.OCToMetrics(md))
}

func (tt *tx) Rollback() error {
	return nil // noop
}

type cache struct {
	metrics       map[string]*internal.MetricBuilder
	scrapeManager *scrape.Manager
}

func newCache(scrapeManager *scrape.Manager) *cache {
	return &cache{
		metrics:       make(map[string]*internal.MetricBuilder, 0),
		scrapeManager: scrapeManager,
	}
}

func (c *cache) Add(l labels.Labels, t int64, v float64) error {
	fmt.Println(l, t, v)
	return nil
	origname := l.Get(model.MetricNameLabel)
	if origname == "" {
		return errors.New("no metric name")
	}

	name := internal.NormalizeMetricName(origname)
	m, ok := c.metrics[name]
	if !ok {
		// TODO(jbd): Add job and instance.
		m = internal.NewMetricBuilder(false, "", nil)
	}

	metadata, ok := c.getMetadata(name) // TODO(jbd): Do it only for once.
	if !ok {
		return nil // skip
	}
	err := m.AddDataPoint(metadata, l, t, v)
	c.metrics[name] = m
	return err
}

func (c *cache) getMetadata(metric string) (scrape.MetricMetadata, bool) {
	// TODO(jbd): Any cheaper way to do this?
	// We are looking this for once only when we recieve
	// a metric type we haven't seen before.
	targets := c.scrapeManager.TargetsActive()
	for _, t := range targets {
		for _, tt := range t {
			m, ok := tt.Metadata(metric)
			if ok {
				return m, ok
			}
		}
	}
	return scrape.MetricMetadata{}, false
}
