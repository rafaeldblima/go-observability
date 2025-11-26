package main

import (
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

type ViaCEPResponse struct {
	CEP        string `json:"cep"`
	Logradouro string `json:"logradouro"`
	Bairro     string `json:"bairro"`
	Localidade string `json:"localidade"`
	UF         string `json:"uf"`
	Erro       bool   `json:"erro,omitempty"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type WeatherResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

var tracer oteltrace.Tracer
var httpClient *http.Client

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
			semconv.ServiceNameKey.String("service-b"),
			semconv.ServiceVersionKey.String("1.0.0"),
		)),
	)

	otel.SetTracerProvider(tp)

	// Set up propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = otel.Tracer("service-b")

	httpClient = &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   30 * time.Second,
	}

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

func fetchCEPInfo(ctx context.Context, cep string) (*ViaCEPResponse, error) {
	ctx, span := tracer.Start(ctx, "fetch-cep-info")
	defer span.End()

	span.SetAttributes(
		semconv.HTTPMethodKey.String("GET"),
		semconv.HTTPURLKey.String(fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)),
	)

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	span.SetAttributes(semconv.HTTPStatusCodeKey.Int(resp.StatusCode))

	var cepResp ViaCEPResponse
	if err := json.NewDecoder(resp.Body).Decode(&cepResp); err != nil {
		span.RecordError(err)
		return nil, err
	}

	if cepResp.Erro {
		return nil, fmt.Errorf("CEP not found")
	}

	return &cepResp, nil
}

func fetchWeatherInfo(ctx context.Context, city string) (*WeatherAPIResponse, error) {
	ctx, span := tracer.Start(ctx, "fetch-weather-info")
	defer span.End()

	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" || apiKey == "demo_key" {
		// Return mock data when no valid API key is provided
		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(200))
		mockTemp := 22.5 // Mock temperature in Celsius
		return &WeatherAPIResponse{
			Current: struct {
				TempC float64 `json:"temp_c"`
			}{
				TempC: mockTemp,
			},
		}, nil
	}

	url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s&aqi=no", apiKey, city)

	span.SetAttributes(
		semconv.HTTPMethodKey.String("GET"),
		semconv.HTTPURLKey.String(url),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	span.SetAttributes(semconv.HTTPStatusCodeKey.Int(resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather API returned status %d", resp.StatusCode)
	}

	var weatherResp WeatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&weatherResp); err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &weatherResp, nil
}

func celsiusToFahrenheit(celsius float64) float64 {
	return celsius*1.8 + 32
}

func celsiusToKelvin(celsius float64) float64 {
	return celsius + 273
}

func handleWeather(c *gin.Context) {
	// Get context from Gin (should already have trace context from otelgin middleware)
	ctx := c.Request.Context()

	// Extract trace context from the incoming request if not already present
	spanCtx := oteltrace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		log.Printf("Warning: No valid span context found in request")
	}

	ctx, span := tracer.Start(ctx, "handle-weather-request")
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

	cepInfo, err := fetchCEPInfo(ctx, req.CEP)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(404))
		c.JSON(http.StatusNotFound, ErrorResponse{Message: "can not find zipcode"})
		return
	}

	weatherInfo, err := fetchWeatherInfo(ctx, cepInfo.Localidade)
	if err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Message: "failed to fetch weather information"})
		return
	}

	tempC := weatherInfo.Current.TempC
	tempF := celsiusToFahrenheit(tempC)
	tempK := celsiusToKelvin(tempC)

	response := WeatherResponse{
		City:  cepInfo.Localidade,
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	span.SetAttributes(
		semconv.HTTPStatusCodeKey.Int(200),
		semconv.HTTPResponseBodySizeKey.Int64(int64(len(fmt.Sprintf("%+v", response)))),
	)

	c.JSON(http.StatusOK, response)
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "service-b"})
}

func main() {
	shutdown := initTracer()
	defer shutdown()

	r := gin.Default()
	r.Use(otelgin.Middleware("service-b"))

	r.POST("/weather", handleWeather)
	r.GET("/health", healthCheck)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	fmt.Printf("Service B starting on port %s\n", port)
	log.Fatal(r.Run(":" + port))
}
