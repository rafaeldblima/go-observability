package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type CEPRequest struct {
	CEP string `json:"cep" binding:"required"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

var tracer oteltrace.Tracer

func initTracer() func() {
	zipkinURL := os.Getenv("ZIPKIN_URL")
	if zipkinURL == "" {
		zipkinURL = "http://localhost:9411/api/v2/spans"
	}

	exporter, err := zipkin.New(zipkinURL)
	if err != nil {
		log.Fatal("Failed to create Zipkin exporter:", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("service-a"),
			semconv.ServiceVersionKey.String("1.0.0"),
		)),
	)

	otel.SetTracerProvider(tp)

	// Set up propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = otel.Tracer("service-a")

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}
}

func validateCEP(cep string) bool {
	if len(cep) != 8 {
		return false
	}

	matched, _ := regexp.MatchString(`^\d{8}$`, cep)
	return matched
}

func forwardToServiceB(ctx context.Context, cep string) (*http.Response, error) {
	span := oteltrace.SpanFromContext(ctx)
	span.SetAttributes(
		semconv.HTTPMethodKey.String("POST"),
		semconv.HTTPURLKey.String("http://service-b:8081/weather"),
	)

	serviceBURL := os.Getenv("SERVICE_B_URL")
	if serviceBURL == "" {
		serviceBURL = "http://service-b:8081"
	}

	requestBody := CEPRequest{CEP: cep}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", serviceBURL+"/weather", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Use instrumented HTTP client to propagate trace context
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   30 * time.Second,
	}
	return client.Do(req)
}

func handleCEP(c *gin.Context) {
	ctx := c.Request.Context()
	ctx, span := tracer.Start(ctx, "handle-cep-request")
	defer span.End()

	var req CEPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Message: "invalid zipcode"})
		return
	}

	span.SetAttributes(semconv.HTTPRequestBodySizeKey.Int(len(req.CEP)))

	if !validateCEP(req.CEP) {
		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(422))
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{Message: "invalid zipcode"})
		return
	}

	ctx, forwardSpan := tracer.Start(ctx, "forward-to-service-b")
	resp, err := forwardToServiceB(ctx, req.CEP)
	forwardSpan.End()

	if err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "internal server error"})
		return
	}
	defer resp.Body.Close()

	span.SetAttributes(semconv.HTTPStatusCodeKey.Int(resp.StatusCode))

	var responseBody interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "internal server error"})
		return
	}

	c.JSON(resp.StatusCode, responseBody)
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "service-a"})
}

func main() {
	shutdown := initTracer()
	defer shutdown()

	r := gin.Default()
	r.Use(otelgin.Middleware("service-a"))

	r.POST("/", handleCEP)
	r.GET("/health", healthCheck)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Service A starting on port %s\n", port)
	log.Fatal(r.Run(":" + port))
}
