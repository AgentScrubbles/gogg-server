# GOGG Server - Kubernetes Deployment Guide

This document provides instructions for deploying the GOGG (GOG Game Downloader) service in a Kubernetes environment.

## Architecture Overview

```
                    ┌─────────────────┐
                    │     Ingress     │
                    │  (nginx/traefik)│
                    └────────┬────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼                             ▼
    ┌─────────────────┐           ┌─────────────────┐
    │    Frontend     │           │     Server      │
    │  (nginx:alpine) │           │   (Go API)      │
    │    Port 80      │           │   Port 8080     │
    └─────────────────┘           └────────┬────────┘
                                           │
                         ┌─────────────────┼─────────────────┐
                         │                 │                 │
                         ▼                 ▼                 ▼
               ┌─────────────────┐ ┌─────────────┐ ┌─────────────────┐
               │   PostgreSQL    │ │  Downloads  │ │   GOG API       │
               │   (Database)    │ │ (NFS/PVC)   │ │  (External)     │
               └─────────────────┘ └─────────────┘ └─────────────────┘
```

## Container Images

| Image | Registry | Description |
|-------|----------|-------------|
| `ghcr.io/agentscrubbles/gogg-server-server` | GHCR | Go API server with embedded frontend option |
| `ghcr.io/agentscrubbles/gogg-server-frontend` | GHCR | Nginx-based static frontend |

### Image Tags

- `latest` - Latest build from main branch
- `main` - Same as latest
- `<sha>` - Specific commit SHA
- `<version>` - Semantic version tags (when released)

---

## Server Container

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `JWT_SECRET` | Yes | - | Secret key for JWT token signing (min 32 chars recommended) |
| `PORT` | No | `8080` | HTTP server listen port |
| `GOGG_DOWNLOAD_PATH` | No | `/downloads/shared` | Path where game files are downloaded |
| `GOGG_STATIC_PATH` | No | `/app/frontend/dist` | Path to frontend static files (set empty to disable) |
| `ALLOWED_ORIGINS` | No | `http://localhost:3000,http://localhost:5173` | Comma-separated list of allowed CORS origins |
| `DEBUG` | No | `false` | Enable debug logging (`true`/`false`) |

### Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 8080 | TCP/HTTP | REST API and WebSocket |

### Volumes

| Mount Path | Type | Description |
|------------|------|-------------|
| `/downloads` | PVC (ReadWriteMany) | Game file storage - requires shared storage (NFS/CephFS/etc.) |

### Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

### Resource Recommendations

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "1000m"
```

Note: During active downloads, CPU usage may spike. Adjust limits based on `MaxConcurrentDownloads` (hardcoded to 3).

---

## Frontend Container

### Environment Variables

The frontend is a static nginx container. API URL is determined at runtime via relative paths (`/api/*`).

No environment variables required.

### Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 80 | TCP/HTTP | Static file server |

### Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /
    port: 80
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /
    port: 80
  initialDelaySeconds: 2
  periodSeconds: 5
```

### Resource Recommendations

```yaml
resources:
  requests:
    memory: "32Mi"
    cpu: "10m"
  limits:
    memory: "128Mi"
    cpu: "100m"
```

---

## Database Requirements

PostgreSQL 14+ is required. The server automatically runs migrations on startup.

### Connection String Format

```
postgres://<user>:<password>@<host>:<port>/<database>?sslmode=<mode>
```

Example:
```
postgres://gogg:secretpassword@postgres-service:5432/gogg?sslmode=require
```

### Database Schema

Tables are auto-created on first run:
- `users` - User accounts
- `user_tokens` - GOG OAuth tokens (encrypted at rest recommended)
- `user_games` - Game catalogue cache per user
- `download_jobs` - Download queue and history

---

## Kubernetes Manifests

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: gogg
```

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gogg-secrets
  namespace: gogg
type: Opaque
stringData:
  database-url: "postgres://gogg:CHANGE_ME@postgres:5432/gogg?sslmode=require"
  jwt-secret: "CHANGE_ME_MIN_32_CHARACTERS_LONG"
```

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gogg-config
  namespace: gogg
data:
  ALLOWED_ORIGINS: "https://gogg.example.com"
  GOGG_DOWNLOAD_PATH: "/downloads/shared"
  GOGG_STATIC_PATH: ""  # Empty = frontend served separately
```

### PersistentVolumeClaim (Downloads)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: gogg-downloads
  namespace: gogg
spec:
  accessModes:
    - ReadWriteMany  # Required for shared access
  storageClassName: nfs-client  # Adjust to your storage class
  resources:
    requests:
      storage: 500Gi  # Adjust based on expected game library size
```

### Server Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gogg-server
  namespace: gogg
spec:
  replicas: 1  # Single replica recommended due to download job processing
  selector:
    matchLabels:
      app: gogg-server
  template:
    metadata:
      labels:
        app: gogg-server
    spec:
      containers:
        - name: server
          image: ghcr.io/agentscrubbles/gogg-server-server:latest
          ports:
            - containerPort: 8080
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: gogg-secrets
                  key: database-url
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: gogg-secrets
                  key: jwt-secret
          envFrom:
            - configMapRef:
                name: gogg-config
          volumeMounts:
            - name: downloads
              mountPath: /downloads
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "1000m"
      volumes:
        - name: downloads
          persistentVolumeClaim:
            claimName: gogg-downloads
```

### Server Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: gogg-server
  namespace: gogg
spec:
  selector:
    app: gogg-server
  ports:
    - port: 8080
      targetPort: 8080
```

### Frontend Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gogg-frontend
  namespace: gogg
spec:
  replicas: 2
  selector:
    matchLabels:
      app: gogg-frontend
  template:
    metadata:
      labels:
        app: gogg-frontend
    spec:
      containers:
        - name: frontend
          image: ghcr.io/agentscrubbles/gogg-server-frontend:latest
          ports:
            - containerPort: 80
          livenessProbe:
            httpGet:
              path: /
              port: 80
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: 80
            initialDelaySeconds: 2
            periodSeconds: 5
          resources:
            requests:
              memory: "32Mi"
              cpu: "10m"
            limits:
              memory: "128Mi"
              cpu: "100m"
```

### Frontend Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: gogg-frontend
  namespace: gogg
spec:
  selector:
    app: gogg-frontend
  ports:
    - port: 80
      targetPort: 80
```

### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gogg-ingress
  namespace: gogg
  annotations:
    # Adjust annotations for your ingress controller
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    # WebSocket support
    nginx.ingress.kubernetes.io/proxy-http-version: "1.1"
    nginx.ingress.kubernetes.io/upstream-hash-by: "$remote_addr"
spec:
  ingressClassName: nginx  # Adjust to your ingress class
  rules:
    - host: gogg.example.com  # Change to your domain
      http:
        paths:
          # API routes to server
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: gogg-server
                port:
                  number: 8080
          - path: /health
            pathType: Exact
            backend:
              service:
                name: gogg-server
                port:
                  number: 8080
          # All other routes to frontend
          - path: /
            pathType: Prefix
            backend:
              service:
                name: gogg-frontend
                port:
                  number: 80
  tls:
    - hosts:
        - gogg.example.com
      secretName: gogg-tls  # Your TLS certificate secret
```

---

## Deployment Options

### Option A: Separate Frontend (Recommended)

Deploy frontend and server as separate services with ingress routing.

**Pros:**
- Independent scaling
- Frontend can have multiple replicas
- Clear separation of concerns

**Cons:**
- More resources (two deployments)
- Slightly more complex ingress config

### Option B: Embedded Frontend

Use only the server image with `GOGG_STATIC_PATH=/app/frontend/dist`.

**Pros:**
- Single deployment
- Simpler setup

**Cons:**
- Server handles static file serving
- Can't scale frontend independently

---

## Security Considerations

1. **JWT Secret**: Use a strong, randomly generated secret (minimum 32 characters)
   ```bash
   openssl rand -base64 32
   ```

2. **Database Credentials**: Store in Kubernetes Secrets, consider using external secret management (Vault, AWS Secrets Manager, etc.)

3. **GOG Tokens**: User OAuth tokens are stored in the database. Consider:
   - Database encryption at rest
   - Network policies to restrict database access
   - Regular token rotation

4. **CORS Origins**: Set `ALLOWED_ORIGINS` to your actual frontend domain(s)

5. **TLS**: Always use HTTPS in production via ingress TLS termination

6. **Network Policies**: Restrict pod-to-pod communication:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: gogg-server-policy
     namespace: gogg
   spec:
     podSelector:
       matchLabels:
         app: gogg-server
     ingress:
       - from:
           - podSelector:
               matchLabels:
                 app: gogg-frontend
         ports:
           - port: 8080
       - from:
           - namespaceSelector:
               matchLabels:
                 name: ingress-nginx
         ports:
           - port: 8080
   ```

---

## Monitoring

### Logs

Both containers log to stdout/stderr. Use your cluster's logging solution (Loki, ELK, CloudWatch, etc.).

Server log levels:
- Default: INFO level
- Set `DEBUG=true` for DEBUG level (verbose)

### Metrics

No built-in Prometheus metrics currently. Monitor via:
- Container resource metrics (CPU, memory)
- HTTP response codes from ingress
- Database connection pool stats

### Alerts Recommendations

- Server pod restarts
- High memory usage (approaching limits)
- Database connection failures (check logs for "Failed to connect to database")
- Download job failures (check logs for "Download job failed")

---

## Troubleshooting

### Server won't start

1. Check database connectivity:
   ```bash
   kubectl exec -it <pod> -n gogg -- sh -c 'nc -zv postgres 5432'
   ```

2. Verify secrets are mounted:
   ```bash
   kubectl exec -it <pod> -n gogg -- env | grep -E 'DATABASE|JWT'
   ```

3. Check logs:
   ```bash
   kubectl logs -f deployment/gogg-server -n gogg
   ```

### Downloads not working

1. Verify PVC is mounted and writable:
   ```bash
   kubectl exec -it <pod> -n gogg -- ls -la /downloads
   kubectl exec -it <pod> -n gogg -- touch /downloads/test && rm /downloads/test
   ```

2. Check download job status in logs (search for "Download job")

### WebSocket connections failing

1. Ensure ingress has WebSocket support annotations
2. Check timeout settings (downloads can be long-running)
3. Verify the path `/api/ws/downloads` routes to the server

### Frontend can't reach API

1. Verify ingress routing (`/api/*` → server, `/*` → frontend)
2. Check CORS settings match your domain
3. Test API directly:
   ```bash
   curl -v https://gogg.example.com/health
   ```

---

## Maintenance

### Database Backups

Implement regular PostgreSQL backups. The schema supports pg_dump/pg_restore.

### Updating Images

```bash
kubectl set image deployment/gogg-server server=ghcr.io/agentscrubbles/gogg-server-server:new-tag -n gogg
kubectl set image deployment/gogg-frontend frontend=ghcr.io/agentscrubbles/gogg-server-frontend:new-tag -n gogg
```

Or update the deployment manifests and apply:
```bash
kubectl apply -f deployment.yaml
```

### Storage Management

Downloaded games accumulate in the PVC. Implement a cleanup policy or expand storage as needed. Games are stored at:
```
/downloads/shared/<game-title>/<platform>/
```
