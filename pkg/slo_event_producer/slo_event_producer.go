//revive:disable:var-naming
package slo_event_producer

//revive:enable:var-naming

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/event"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/stringmap"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.seznam.net/sklik-devops/slo-exporter/pkg/producer"
)

var (
	log *logrus.Entry
)

func init() {
	log = logrus.WithField("component", "slo_event_producer")
}

type ClassifiableEvent interface {
	GetEventKey() string
	IsClassified() bool
	GetSloMetadata() *stringmap.StringMap
	GetSloClassification() *producer.SloClassification
	GetTimeOccurred() time.Time
}

type sloEventProducerConfig struct {
	RulesFiles []string
}

func NewFromViper(viperConfig *viper.Viper) (*SloEventProducer, error) {
	var config sloEventProducerConfig
	if err := viperConfig.UnmarshalExact(&config); err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}
	return New(config)
}

func New(config sloEventProducerConfig) (*SloEventProducer, error) {
	eventEvaluator, err := NewEventEvaluatorFromConfigFiles(config.RulesFiles)
	if err != nil {
		return nil, err
	}
	return &SloEventProducer{eventEvaluator: eventEvaluator}, nil
}

type SloEventProducer struct {
	eventEvaluator EventEvaluator
	observer       prometheus.Observer
}

func (sep *SloEventProducer) SetPrometheusObserver(observer prometheus.Observer) {
	sep.observer = observer
}

func (sep *SloEventProducer) observeDuration(start time.Time) {
	if sep.observer != nil {
		sep.observer.Observe(time.Since(start).Seconds())
	}
}

func (sep *SloEventProducer) PossibleMetadataKeys() []string {
	return sep.eventEvaluator.PossibleMetadataKeys()
}

func (sep *SloEventProducer) generateSLOEvents(event *producer.RequestEvent, sloEventsChan chan<- *event.Slo) {
	sep.eventEvaluator.Evaluate(event, sloEventsChan)
}

// TODO move to interfaces in channels, those cannot be mixed so we have to stick to one type now
func (sep *SloEventProducer) Run(inputEventChan <-chan *producer.RequestEvent, outputSLOEventChan chan<- *event.Slo) {
	go func() {
		defer close(outputSLOEventChan)

		for newEvent := range inputEventChan {
			start := time.Now()
			sep.generateSLOEvents(newEvent, outputSLOEventChan)
			sep.observeDuration(start)
		}
		log.Info("input channel closed, finishing")
	}()
}
