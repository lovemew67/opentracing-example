package main

import (
	"flag"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
)

var (
	globalTracer opentracing.Tracer
)

var (
	port = flag.Int("port", 8080, "Example app port.")
)

func main() {
	flag.Parse()

	var tracer opentracing.Tracer

	// docker run -d --name jaeger \
	// -e COLLECTOR_ZIPKIN_HTTP_PORT=9411 \
	// -p 5775:5775/udp \
	// -p 6831:6831/udp \
	// -p 6832:6832/udp \
	// -p 5778:5778 \
	// -p 16686:16686 \
	// -p 14268:14268 \
	// -p 14250:14250 \
	// -p 9411:9411 \
	// jaegertracing/all-in-one:1.21

	// jaeger
	sConf := &jaegercfg.SamplerConfig{
		Type:  jaeger.SamplerTypeRateLimiting,
		Param: float64(1),
	}
	rConf := &jaegercfg.ReporterConfig{
		QueueSize:           128,
		BufferFlushInterval: 10 * time.Second,
		LocalAgentHostPort:  "127.0.0.1:6831",
		LogSpans:            false,
	}
	cfg := jaegercfg.Configuration{
		ServiceName: "example",
		Sampler:     sConf,
		Reporter:    rConf,
	}
	jLogger := jaegerlog.StdLogger
	jMetricsFactory := metrics.NullFactory
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jLogger),
		jaegercfg.Metrics(jMetricsFactory),
		jaegercfg.ZipkinSharedRPCSpan(true),
	)
	if err != nil {
		panic(err)
	}
	if closer != nil {
		defer closer.Close()
	}
	opentracing.SetGlobalTracer(tracer)
	globalTracer = tracer

	// start up rest server
	// startUpRest()
	// startUpGRPC()
	// startUpGraphQL()
	startUpWebsocket()
}
