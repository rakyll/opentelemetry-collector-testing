package promreceiver

import (
	"context"
	"sync"

	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/promreceiver/internal"
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

	logger := internal.NewZapToGokitLogAdapter(r.logger)

	var startErr error
	r.startOnce.Do(func() {
		discoveryManager := discovery.NewManager(discoveryCtx, logger)
		discoveryCfg := make(map[string]discovery.Configs)
		for _, scrapeConfig := range r.cfg.PrometheusConfig.ScrapeConfigs {
			discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		}
		if err := discoveryManager.ApplyConfig(discoveryCfg); err != nil {
			startErr = err
			return
		}
		go func() {
			if err := discoveryManager.Run(); err != nil {
				r.logger.Error("Discovery manager failed", zap.Error(err))
				host.ReportFatalError(err)
			}
		}()

		collector := &collector{
			sink: r.consumer,
		}
		scrapeManager := scrape.NewManager(logger, collector)
		// TODO(jbd): Remove collector's dependency on the scrape manager.
		// Required for metadata for now.
		collector.scrapeManager = scrapeManager
		if err := scrapeManager.ApplyConfig(r.cfg.PrometheusConfig); err != nil {
			startErr = err
			return
		}
		go func() {
			if err := scrapeManager.Run(discoveryManager.SyncCh()); err != nil {
				r.logger.Error("Scrape manager failed", zap.Error(err))
				host.ReportFatalError(err)
			}
		}()
	})
	return startErr
}

func (r *reciever) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(r.cancelFunc)
	return nil
}
