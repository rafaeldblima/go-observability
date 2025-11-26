# Sistema de Temperatura por CEP com Observabilidade

Este projeto implementa um sistema distribuído em Go composto por dois serviços que trabalham em conjunto para fornecer informações de temperatura baseadas em CEP brasileiro, com implementação completa de observabilidade usando OpenTelemetry e Zipkin.

## Arquitetura

### Serviço A (Input Service)
- **Porta**: 8080
- **Responsabilidade**: Validação de entrada e encaminhamento
- **Endpoint**: `POST /`
- **Funcionalidades**:
  - Recebe CEP via POST no formato `{"cep": "29902555"}`
  - Valida se o CEP tem 8 dígitos e é uma string
  - Encaminha requisições válidas para o Serviço B
  - Retorna erro 422 para CEPs inválidos

### Serviço B (Orchestration Service)
- **Porta**: 8081
- **Responsabilidade**: Orquestração de APIs externas
- **Endpoint**: `POST /weather`
- **Funcionalidades**:
  - Consulta informações do CEP via API ViaCEP
  - Busca dados meteorológicos via WeatherAPI
  - Converte temperaturas (Celsius, Fahrenheit, Kelvin)
  - Retorna dados consolidados

## Observabilidade

### OpenTelemetry (OTEL)
- **Distributed Tracing**: Rastreamento completo entre serviços
- **Spans Customizados**: 
  - `handle-cep-request`: Processamento da requisição no Serviço A
  - `forward-to-service-b`: Encaminhamento para Serviço B
  - `handle-weather-request`: Processamento no Serviço B
  - `fetch-cep-info`: Consulta à API ViaCEP
  - `fetch-weather-info`: Consulta à API WeatherAPI

### Zipkin
- **Interface Web**: http://localhost:9411
- **Visualização**: Traces distribuídos e métricas de performance
- **Análise**: Tempo de resposta e identificação de gargalos

## APIs Utilizadas

### ViaCEP
- **URL**: https://viacep.com.br/ws/{cep}/json/
- **Propósito**: Obter informações de localização do CEP
- **Sem autenticação necessária**

### WeatherAPI
- **URL**: http://api.weatherapi.com/v1/current.json
- **Propósito**: Obter dados meteorológicos atuais
- **Requer API Key** (configurável via variável de ambiente)

## Estrutura do Projeto

```
go-observability/
├── service-a/
│   ├── main.go
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── service-b/
│   ├── main.go
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── docker-compose.yml
├── otel-collector-config.yml
└── README.md
```

## Configuração e Execução

### Pré-requisitos
- Docker e Docker Compose instalados
- Chave da API WeatherAPI (opcional para testes)

### Variáveis de Ambiente

```bash
# Opcional - Para usar a WeatherAPI real
export WEATHER_API_KEY=sua_chave_aqui
```

### Executando o Sistema

1. **Clone e navegue para o diretório**:
```bash
cd /Users/rafael.lima/GolandProjects/go-observability
```

2. **Inicie todos os serviços**:
```bash
docker-compose up --build
```

3. **Verifique se os serviços estão rodando**:
```bash
# Health check Serviço A
curl http://localhost:8080/health

# Health check Serviço B
curl http://localhost:8081/health
```

### Testando o Sistema

#### Teste com CEP Válido
```bash
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"cep": "01310100"}'
```

**Resposta esperada (200)**:
```json
{
  "city": "São Paulo",
  "temp_C": 22.5,
  "temp_F": 72.5,
  "temp_K": 295.5
}
```

#### Teste com CEP Inválido (formato)
```bash
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"cep": "123"}'
```

**Resposta esperada (422)**:
```json
{
  "message": "invalid zipcode"
}
```

#### Teste com CEP Inexistente
```bash
curl -X POST http://localhost:8080/ \
  -H "Content-Type: application/json" \
  -d '{"cep": "99999999"}'
```

**Resposta esperada (404)**:
```json
{
  "message": "can not find zipcode"
}
```

## Monitoramento

### Zipkin Dashboard
- **URL**: http://localhost:9411
- **Funcionalidades**:
  - Visualizar traces distribuídos
  - Analisar latência entre serviços
  - Identificar gargalos de performance
  - Debugar problemas de conectividade

### OpenTelemetry Collector
- **Porta gRPC**: 4317
- **Porta HTTP**: 4318
- **Métricas Prometheus**: 8888

## Fórmulas de Conversão

### Celsius para Fahrenheit
```
F = C × 1.8 + 32
```

### Celsius para Kelvin
```
K = C + 273
```

## Desenvolvimento Local

### Executando Serviços Individualmente

#### Serviço A
```bash
cd service-a
go run main.go
```

#### Serviço B
```bash
cd service-b
WEATHER_API_KEY=sua_chave go run main.go
```

### Executando Apenas Infraestrutura
```bash
# Apenas Zipkin e OTEL Collector
docker-compose up zipkin otel-collector
```

## Troubleshooting

### Problemas Comuns

1. **Erro de conexão entre serviços**:
   - Verifique se todos os containers estão na mesma rede
   - Confirme se as portas estão corretas

2. **WeatherAPI retorna erro**:
   - Verifique se a API Key está configurada
   - Confirme se há limite de requisições

3. **Traces não aparecem no Zipkin**:
   - Verifique se o Zipkin está acessível
   - Confirme a configuração do OTEL_EXPORTER_ZIPKIN_ENDPOINT

### Logs dos Serviços
```bash
# Ver logs de todos os serviços
docker-compose logs -f

# Ver logs de um serviço específico
docker-compose logs -f service-a
docker-compose logs -f service-b
```

## Parar o Sistema

```bash
# Parar todos os serviços
docker-compose down

# Parar e remover volumes
docker-compose down -v

# Parar e remover imagens
docker-compose down --rmi all
```

## Tecnologias Utilizadas

- **Go 1.21**: Linguagem de programação
- **Gin**: Framework web HTTP
- **OpenTelemetry**: Observabilidade e tracing
- **Zipkin**: Visualização de traces distribuídos
- **Docker**: Containerização
- **Docker Compose**: Orquestração de containers

## Contribuição

Para contribuir com o projeto:

1. Faça um fork do repositório
2. Crie uma branch para sua feature
3. Implemente suas mudanças
4. Adicione testes se necessário
5. Submeta um pull request

## Licença

Este projeto é desenvolvido para fins educacionais e demonstração de conceitos de observabilidade em sistemas distribuídos.
