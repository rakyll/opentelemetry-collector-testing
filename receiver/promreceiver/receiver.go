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

	logger *zap.Logger

	startOnce sync.Once
	stopOnce  sync.Once
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
		scrapeManager := scrape.NewManager(nil, &collector{})
		if err := scrapeManager.ApplyConfig(r.cfg.PrometheusConfig); err != nil {
			startErr = err
			return
		}

		discoveryManager := discovery.NewManager(discoveryCtx, nil)
		discoveryCfg := make(map[string]discovery.Configs)
		for _, scrapeConfig := range r.cfg.PrometheusConfig.ScrapeConfigs {
			discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		}
		if err := discoveryManager.ApplyConfig(discoveryCfg); err != nil {
			host.ReportFatalError(err)
		}

		<-r.runDiscovery(host, discoveryManager)
		scrapeManager.Run(discoveryManager.SyncCh()) // TODO(jbd): Fix error handling.
	})
	return startErr
}

func (r *reciever) runDiscovery(host component.Host, discoveryManager *discovery.Manager) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		if err := discoveryManager.Run(); err != nil {
			host.ReportFatalError(err) // TODO(jbd): Should halt or just report?
		}
	}()
	ch <- struct{}{}
	return ch
}

func (r *reciever) runScraper() {

}

func (r *reciever) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(r.cancelFunc)
	return nil
}
