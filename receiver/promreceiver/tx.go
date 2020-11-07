package promreceiver

import (
	"context"
	"errors"
	"sync/atomic"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/promreceiver/internal"
	"go.opentelemetry.io/collector/translator/internaldata"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
)

var txidSeq int64

type collector struct {
	sink consumer.MetricsConsumer
}

func (c *collector) Appender(ctx context.Context) storage.Appender {
	return &tx{
		txid:  atomic.AddInt64(&txidSeq, 1),
		sink:  c.sink,
		cache: newCache(),
	}
}

type tx struct {
	txid int64 // access atomically
	sink consumer.MetricsConsumer

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
	m := tt.cache.ConvertToMetrics()
	if m.Size() == 0 {
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
	metrics map[string]*internal.MetricFamily
}

func newCache() *cache {
	return &cache{
		metrics: make(map[string]*internal.MetricFamily, 0),
	}
}

func (c *cache) Add(l labels.Labels, t int64, v float64) error {
	// TODO(jbd): Normalize metric name.
	name := l.Get(model.MetricNameLabel)
	if name == "" {
		return errors.New("no metric name")
	}

	m, ok := c.metrics[name]
	if !ok {
		m = internal.NewMetricFamily(name, nil)
	}

	err := m.Add(name, l, t, v)
	c.metrics[name] = m
	return err
}

func (c *cache) ConvertToMetrics() pdata.Metrics {
	m := pdata.NewMetrics()
	return m
}
