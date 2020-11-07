package promreceiver

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/receiver/promreceiver/internal"

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
	return tt.sink.ConsumeMetrics(context.Background(), m) // TODO(jbd): Change the background context?
}

func (tt *tx) Rollback() error {
	return nil // noop
}

type cache struct {
	metrics map[string]pdata.Metric
}

func newCache() *cache {
	return &cache{
		metrics: make(map[string]pdata.Metric, 0),
	}
}

func (c *cache) Add(l labels.Labels, t int64, v float64) error {
	// TODO(jbd): Normalize metric name.
	name := l.Get(model.MetricNameLabel)
	if name == "" {
		return errors.New("no metric name")
	}

	metric, ok := c.metrics[name]
	if !ok {
		metric = pdata.NewMetric()
		metric.InitEmpty()
		metric.SetName(name)
		metric.SetDataType(internal.ParseMetricDataType(name))
		metric.SetUnit(internal.ParseMetricUnit(name))
		// metric.SetDescription() // TODO
	}

	switch metric.DataType() {
	case pdata.MetricDataTypeDoubleGauge:
		pt := pdata.NewDoubleDataPoint()
		pt.InitEmpty()
		pt.SetValue(v)
		// pt.SetStartTime() TODO
		// pt.SetTimestamp() TODO
		for _, label := range l {
			pt.LabelsMap().Insert(label.Name, label.Value)
		}
		metric.DoubleGauge().DataPoints().Append(pt)
	case pdata.MetricDataTypeDoubleSum:
		// TODO(jbd): Support summary type.
	case pdata.MetricDataTypeDoubleHistogram:
		pt := pdata.NewDoubleHistogramDataPoint()
		pt.InitEmpty()
		// pt.SetValue(v)
		// pt.SetBucketCounts()
		// pt.SetStartTime() TODO
		// pt.SetTimestamp() TODO
		for _, label := range l {
			pt.LabelsMap().Insert(label.Name, label.Value)
		}
		metric.DoubleHistogram().DataPoints().Append(pt)
	}

	c.metrics[name] = metric
	return nil
}

func (c *cache) ConvertToMetrics() pdata.Metrics {
	m := pdata.NewMetrics()
	return m
}
