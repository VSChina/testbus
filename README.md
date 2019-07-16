# `./testbus` Service Bus cmd helper

Send and receive from Service Bus while tracing using either OpenCensus or OpenTracing

## Trace with OpenCensus
```bash
TRACING=true go run main.go send --msg-count 100 --conn-str 'Endpoint=...'
```
```bash
TRACING=true go run main.go receive --conn-str 'Endpoint=...'
```

## Low-Level AMQP Protocol Logging
```bash
DEBUG-LEVEL=3 go run -tags debug main.go
```
