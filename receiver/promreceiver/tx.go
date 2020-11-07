package promreceiver

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
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
		sink:  c.sink,
		cache: newCache(c.scrapeManager),
	}
}

type tx struct {
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

	var metrics []*v1.Metric
	for _, m := range tt.cache.metrics {
		metric, _, _ := m.ToMetric()
		metrics = append(metrics, metric)
	}
	md := consumerdata.MetricsData{
		Node:     tt.node,
		Resource: tt.resource,
		Metrics:  metrics,
	}
	// TODO(jbd): Change the background context?
	return tt.sink.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(md))
}

func (tt *tx) Rollback() error {
	return nil // noop
}

type cache struct {
	metrics       map[string]*internal.MetricFamily
	scrapeManager *scrape.Manager
}

func newCache(scrapeManager *scrape.Manager) *cache {
	return &cache{
		metrics:       make(map[string]*internal.MetricFamily, 0),
		scrapeManager: scrapeManager,
	}
}

func (c *cache) Add(l labels.Labels, t int64, v float64) error {
	fmt.Println(l, t, v)
	name := l.Get(model.MetricNameLabel)
	if name == "" {
		return errors.New("no metric name")
	}

	targets := c.scrapeManager.TargetsActive() // TODO(jbd): Fix the contention.
	fmt.Println("targets ======>", targets)
	target, ok := targets[name]
	if !ok || len(target) == 0 {
		return nil // skip recording
	}

	m, ok := c.metrics[name]
	if !ok {
		metadata, _ := target[0].Metadata(name)
		m = internal.NewMetricFamily(name, metadata)
	}

	err := m.Add(name, l, t, v)
	c.metrics[name] = m
	return err
}
