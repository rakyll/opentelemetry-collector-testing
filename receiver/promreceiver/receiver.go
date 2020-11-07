package promreceiver

import (
	"context"
	"sync"

	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type reciever struct {
	cfg      *Config
	consumer consumer.MetricsConsumer

	cancelFunc context.CancelFunc // cancels the prometheus discovery

	startOnce sync.Once
	stopOnce  sync.Once

	logger *zap.Logger
}

func newReciever(logger *zap.Logger, cfg *Config, consumer consumer.MetricsConsumer) *reciever {
	return &reciever{
		cfg:      cfg,
		consumer: consumer,
		logger:   logger,
	}
}

func (r *reciever) Start(ctx context.Context, host component.Host) error {
	discoveryCtx := context.Background()
	discoveryCtx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	var startErr error
	// TODO(jbd): Make logger happen.
	r.startOnce.Do(func() {
		discoveryManager := discovery.NewManager(discoveryCtx, nil)
		discoveryCfg := make(map[string]discovery.Configs)
		for _, scrapeConfig := range r.cfg.PrometheusConfig.ScrapeConfigs {
			discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		}
		if err := discoveryManager.ApplyConfig(discoveryCfg); err != nil {
			host.ReportFatalError(err)
		}
		go func() {
			if err := discoveryManager.Run(); err != nil {
				host.ReportFatalError(err) // TODO(jbd): Should halt or just report?
			}
		}()

		scrapeManager := scrape.NewManager(nil, &collector{
			sink: r.consumer,
		})
		if err := scrapeManager.ApplyConfig(r.cfg.PrometheusConfig); err != nil {
			startErr = err // TODO(jbd): Should halt or just report?
			return
		}
		if err := scrapeManager.Run(discoveryManager.SyncCh()); err != nil {
			host.ReportFatalError(err) // TODO(jbd): Should halt or just report?
		}
	})
	return startErr
}

func (r *reciever) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(r.cancelFunc)
	return nil
}
