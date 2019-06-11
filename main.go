package main

import (
	"log"
	"os"

	"contrib.go.opencensus.io/exporter/jaeger"
	_ "github.com/devigned/tab/opentracing"
	"go.opencensus.io/trace"

	"github.com/devigned/testbus/cmd"
)

func main() {
	if os.Getenv("TRACING") == "true" {
		closer, err := initOpenCensus()
		if err != nil {
			log.Fatalln(err)
		}
		defer closer()
	}
	cmd.Execute()
}

func initOpenCensus() (func(), error) {
	exporter, err := jaeger.NewExporter(jaeger.Options{
		AgentEndpoint:     "localhost:6831",
		CollectorEndpoint: "http://localhost:14268/api/traces",
		Process: jaeger.Process{
			ServiceName: "testbus",
		},
	})

	if err != nil {
		return nil, err
	}

	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	trace.RegisterExporter(exporter)
	return exporter.Flush, nil
}
