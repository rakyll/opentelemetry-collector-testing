package promreceiver

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/consumer"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
)

var txidSeq int64

type collector struct {
	sink consumer.MetricsConsumer
}

func (c *collector) Appender(ctx context.Context) storage.Appender {
	return &tx{
		txid: atomic.AddInt64(&txidSeq, 1),
		sink: c.sink,
	}
}

type value struct {
	l labels.Labels
	t int64
	v float64
}

type tx struct {
	txid int64 // access atomically
	sink consumer.MetricsConsumer

	values []*value

	node     *commonpb.Node
	resource *resourcepb.Resource
}

func (tt *tx) Add(l labels.Labels, t int64, v float64) (uint64, error) {
	tt.values = append(tt.values, &value{l: l, t: t, v: v})
	return 0, nil
}

func (tt *tx) AddFast(ref uint64, t int64, v float64) error {
	return storage.ErrNotFound
}

func (tt *tx) Commit() error {
	for _, v := range tt.values {
		fmt.Println("----", v)
	}
	return nil
}

func (tt *tx) Rollback() error {
	return nil // noop
}
