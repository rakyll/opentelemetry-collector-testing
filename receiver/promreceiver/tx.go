package promreceiver

import (
	"context"
	"fmt"

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/storage"
)

type collector struct{}

func (c *collector) Appender(ctx context.Context) storage.Appender {
	return &tx{}
}

type tx struct{}

func (a *tx) Add(l labels.Labels, t int64, v float64) (uint64, error) {
	fmt.Println("-----", l, t, v)
	return 0, nil
}

func (a *tx) AddFast(ref uint64, t int64, v float64) error {
	fmt.Println(ref, t, v)
	return nil
}

func (a *tx) Commit() error {
	fmt.Println("commit")
	return nil
}

func (a *tx) Rollback() error {
	fmt.Println("rollback")
	return nil
}
