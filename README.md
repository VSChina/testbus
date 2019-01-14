# `./testbus` Service Bus cmd helper

Send and receive from Service Bus while tracing using either OpenCensus or OpenTracing

## Trace with OpenCensus
```bash
go run main.go send --msg-count 100 --conn-str 'Endpoint=...'
```
```bash
go run main.go receive --conn-str 'Endpoint=...'
```


## Trace with OpenTracing
```bash
TRACING=OT go run main.go send --msg-count 100 --conn-str 'Endpoint=...'
```
```bash
TRACING=OT go run main.go receive --conn-str 'Endpoint=...'
```
