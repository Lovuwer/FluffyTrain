# Titan Deployment Guide

This guide covers deploying Titan in various environments.

## Prerequisites

- Go 1.24+ (for building from source)
- Docker and Docker Compose (for containerized deployment)
- kubectl + Kustomize (for Kubernetes deployment)

---

## Docker Compose (Development/Testing)

The simplest way to run Titan locally.

### Quick Start

```bash
cd deployments
docker-compose up -d
```

This starts:
- Redis (port 6379)
- PostgreSQL (port 5432)
- 1 API server (port 8080)
- 3 Workers

### View Logs

```bash
docker-compose logs -f api
docker-compose logs -f worker-1
```

### Stop Services

```bash
docker-compose down
```

### Clean Up Data

```bash
docker-compose down -v  # Removes volumes
```

### Custom Configuration

Copy and edit the override file:

```bash
cp docker-compose.override.yml.example docker-compose.override.yml
```

Edit settings:
```yaml
services:
  api:
    environment:
      TITAN_LOGGING_LEVEL: debug
  worker-1:
    environment:
      TITAN_WORKER_CONCURRENCY: 20
```

---

## Kubernetes (Production)

Titan uses Kustomize for Kubernetes deployments with environment overlays.

### Directory Structure

```
deployments/k8s/
├── base/
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── api-deployment.yaml
│   ├── api-service.yaml
│   ├── worker-deployment.yaml
│   ├── pdb.yaml
│   └── hpa.yaml
└── overlays/
    ├── dev/
    │   └── kustomization.yaml
    └── prod/
        └── kustomization.yaml
```

### Deploy to Development

```bash
kubectl apply -k deployments/k8s/overlays/dev
```

### Deploy to Production

```bash
kubectl apply -k deployments/k8s/overlays/prod
```

### Verify Deployment

```bash
kubectl -n titan get pods
kubectl -n titan get svc
kubectl -n titan get hpa
```

### Check Logs

```bash
kubectl -n titan logs -l app=titan-api -f
kubectl -n titan logs -l app=titan-worker -f
```

### Secrets Configuration

Create secrets before deploying:

```bash
kubectl -n titan create secret generic titan-secrets \
  --from-literal=redis-password=your-redis-password \
  --from-literal=postgres-password=your-postgres-password
```

Or use the provided secret template (edit first):

```bash
kubectl apply -f deployments/k8s/base/secret.yaml
```

---

## Health Checks

### Liveness Probe

- **Endpoint**: `/health`
- **Purpose**: Detect if process is stuck
- **Action on failure**: Restart pod

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
```

### Readiness Probe

- **Endpoint**: `/ready`
- **Purpose**: Check Redis/Postgres connectivity
- **Action on failure**: Remove from service

```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

---

## Scaling

### API Servers

The API is stateless. Scale horizontally:

```bash
kubectl -n titan scale deployment titan-api --replicas=5
```

Or configure HPA:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: titan-api
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Workers

Workers scale based on queue depth:

```bash
kubectl -n titan scale deployment titan-worker --replicas=10
```

**Important**: Only ONE watchdog/scheduler runs across all workers (leader election).

---

## Resource Recommendations

### API Server

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

### Worker

```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 512Mi
```

Adjust based on job handler requirements.

---

## High Availability

### Redis

Use Redis Sentinel or Redis Cluster for HA:

```yaml
# Example Sentinel configuration
TITAN_REDIS_HOST: redis-sentinel
TITAN_REDIS_PORT: 26379
```

### PostgreSQL

Use a managed service (RDS, Cloud SQL) or run a replicated setup.

### API Servers

Run 3+ replicas behind a load balancer:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: titan-api
spec:
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
  selector:
    app: titan-api
```

### Workers

Use PodDisruptionBudget to ensure minimum workers:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: titan-worker-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: titan-worker
```

---

## Monitoring

### Prometheus Metrics

Add ServiceMonitor for Prometheus Operator:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: titan
spec:
  selector:
    matchLabels:
      app: titan-api
  endpoints:
  - port: http
    path: /metrics
```

### Key Metrics to Watch

- `titan_queue_depth` - Jobs waiting in queue
- `titan_processing_count` - Jobs being processed
- `titan_job_duration_seconds` - Job execution time
- `titan_dlq_count` - Dead letter queue size

---

## Troubleshooting

### Jobs Not Processing

1. Check worker logs: `kubectl logs -l app=titan-worker`
2. Check Redis connectivity: `/ready` endpoint
3. Verify queue has jobs: `/api/v1/queues/stats`

### Jobs Going to DLQ

1. Check last error: `/api/v1/dlq`
2. Review handler implementation
3. Increase `max_retries` if transient failures

### High Latency

1. Check Redis latency: `/ready` shows component latency
2. Review handler performance
3. Scale workers horizontally

### Memory Issues

1. Check for job payload size
2. Tune `TITAN_REDIS_POOL_SIZE`
3. Add memory limits to containers
