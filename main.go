package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
	"os/signal"
	"time"
)

var (
	traceProvider *trace.TracerProvider
)

func main() {
	url := "localhost:4317"
	initProvider(name, url)
	l := log.New(os.Stdout, "", 0)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	errCh := make(chan error)
	app := NewApp(os.Stdin, l)
	go func() {
		errCh <- app.Run(context.Background())
	}()

	select {
	case <-sigCh:
		l.Println("\ngoodbye")
		return
	case err := <-errCh:
		if err != nil {
			l.Fatal(err)
		}
	}
	Shutdown()
}

func initProvider(name string, url string) error {
	ctx := context.Background()

	res, err := addAttributes(ctx, name)
	if err != nil {
		fmt.Errorf("failed to create resource %w", err)
	}

	traceExporter, err := connectToExporter(ctx, url)
	if err != nil {
		fmt.Errorf("failed to connect to exporter %w", err)
	}

	registerExporterWithTraceprovider(traceExporter, res)
	return nil
}

func registerExporterWithTraceprovider(traceExporter *otlptrace.Exporter, res *resource.Resource) {
	bsp := trace.NewSimpleSpanProcessor(traceExporter)
	traceProvider = trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(res),
		trace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func connectToExporter(ctx context.Context, url string) (*otlptrace.Exporter, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, url, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return traceExporter, nil
}

func addAttributes(ctx context.Context, name string) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(name),
		))
	return res, err
}

func Shutdown() {
	if traceProvider != nil {
		if err := traceProvider.Shutdown(context.Background()); err != nil {
			log.Fatal(err)
		}
	}
}
