package main

import (
	"io"
	"log"
	"os"

	"github.com/devigned/tab"
	tot "github.com/devigned/tab/opentracing"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-lib/metrics"
	"go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/trace"

	"github.com/uber/jaeger-client-go/config"

	"github.com/devigned/testbus/cmd"
)

type (
	flusher interface {
		Flush()
	}

	closeFlush struct {
		closer io.Closer
	}
)

func (cf *closeFlush) Flush() {
	_ = cf.closer.Close()
}

func main() {
	var f flusher
	if os.Getenv("TRACER") == "OT" {
		f = setupOpenTracing()
	} else {
		f = setupOpenCensus()
	}
	defer f.Flush()
	cmd.Execute()
}

func setupOpenTracing() flusher {
	tracer, closer, err := config.Configuration{
		ServiceName: "testbus",
	}.NewTracer(config.Metrics(metrics.NullFactory))

	if err != nil {
		log.Fatal(err)
	}

	tab.Register(new(tot.Trace)) // tab defaults to opencensus

	opentracing.InitGlobalTracer(tracer)
	return &closeFlush{closer: closer}
}

func setupOpenCensus() flusher {
	exporter, err := jaeger.NewExporter(jaeger.Options{
		CollectorEndpoint: "http://localhost:14268/api/traces",
		Process: jaeger.Process{
			ServiceName: "testbus",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	return exporter
}
