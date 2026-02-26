#!/usr/bin/env bash
#
# End-to-end Helm chart test using kind.
#
# Creates a kind cluster, deploys postgres + minio as test fixtures,
# installs the Skillbox Helm chart, and validates the rollout.
#
# Usage:
#   bash scripts/helm-test-kind.sh              # run and clean up
#   bash scripts/helm-test-kind.sh --no-cleanup  # keep cluster after test
#
set -euo pipefail

CLUSTER="skillbox-helm-test"
NS="skillbox-test"
RELEASE="skillbox"

log()  { echo -e "\033[0;32m[OK]\033[0m $*"; }
fail() { echo -e "\033[0;31m[FAIL]\033[0m $*"; exit 1; }

# --- Cleanup ---
SKIP_CLEANUP=false
[[ "${1:-}" == "--no-cleanup" ]] && SKIP_CLEANUP=true

cleanup() { kind delete cluster --name "$CLUSTER" 2>/dev/null || true; }
[[ "$SKIP_CLEANUP" == false ]] && trap cleanup EXIT

# --- Pre-flight ---
for cmd in kind kubectl helm docker; do
    command -v "$cmd" &>/dev/null || fail "$cmd is required"
done

# --- 1. Kind cluster ---
log "Creating kind cluster..."
cat <<EOF | kind create cluster --name "$CLUSTER" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    extraMounts:
      - hostPath: /var/run/docker.sock
        containerPath: /var/run/docker.sock
EOF

# --- 2. Build and load image ---
log "Building skillbox image..."
docker build -f deploy/docker/Dockerfile -t ghcr.io/devs-group/skillbox:test .

log "Loading image into kind..."
kind load docker-image ghcr.io/devs-group/skillbox:test --name "$CLUSTER"

# --- 3. Deploy test fixtures (postgres + minio) ---
log "Deploying test fixtures..."
kubectl create namespace "$NS"

kubectl apply -n "$NS" -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: postgres
  labels: { app: postgres }
spec:
  containers:
    - name: postgres
      image: postgres:16-alpine
      env:
        - { name: POSTGRES_USER,     value: skillbox }
        - { name: POSTGRES_PASSWORD, value: skillbox }
        - { name: POSTGRES_DB,       value: skillbox }
      ports:
        - containerPort: 5432
      readinessProbe:
        exec:
          command: [pg_isready, -U, skillbox]
        initialDelaySeconds: 5
        periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
spec:
  selector: { app: postgres }
  ports:
    - port: 5432
---
apiVersion: v1
kind: Pod
metadata:
  name: minio
  labels: { app: minio }
spec:
  containers:
    - name: minio
      image: minio/minio:latest
      args: [server, /data, --console-address, ":9001"]
      env:
        - { name: MINIO_ROOT_USER,     value: minioadmin }
        - { name: MINIO_ROOT_PASSWORD, value: minioadmin }
      ports:
        - containerPort: 9000
      readinessProbe:
        httpGet: { path: /minio/health/live, port: 9000 }
        initialDelaySeconds: 5
        periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: minio
spec:
  selector: { app: minio }
  ports:
    - port: 9000
EOF

log "Waiting for postgres..."
kubectl wait --for=condition=Ready pod/postgres -n "$NS" --timeout=120s

log "Waiting for minio..."
kubectl wait --for=condition=Ready pod/minio -n "$NS" --timeout=120s

log "Creating minio buckets..."
kubectl run minio-init --rm -i --restart=Never -n "$NS" \
    --image=minio/mc:latest --command -- /bin/sh -c \
    'mc alias set local http://minio:9000 minioadmin minioadmin && mc mb --ignore-existing local/skills && mc mb --ignore-existing local/executions'

# --- 4. Install Helm chart ---
log "Installing Helm chart..."
helm install "$RELEASE" deploy/helm/skillbox/ \
    --namespace "$NS" \
    --set image.tag=test \
    --set image.pullPolicy=Never \
    --set replicaCount=1 \
    --set postgresql.dsn="postgres://skillbox:skillbox@postgres:5432/skillbox?sslmode=disable" \
    --set minio.endpoint="minio:9000" \
    --set minio.accessKey="minioadmin" \
    --set minio.secretKey="minioadmin" \
    --wait \
    --timeout 180s

# --- 5. Validate ---
log "Checking rollout..."
kubectl rollout status deployment/"$RELEASE" -n "$NS" --timeout=120s
kubectl get pods -n "$NS"

log "Testing /health endpoint..."
kubectl port-forward -n "$NS" "deployment/$RELEASE" 8080:8080 &
PF_PID=$!
sleep 3

HEALTH=$(curl -sf http://localhost:8080/health || echo "FAIL")
kill $PF_PID 2>/dev/null || true

if echo "$HEALTH" | grep -q '"status"'; then
    log "Health check passed: $HEALTH"
else
    fail "Health check failed: $HEALTH"
fi

echo ""
log "All checks passed!"
