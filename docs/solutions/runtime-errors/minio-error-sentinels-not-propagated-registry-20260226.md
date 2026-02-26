---
module: Skillbox
date: 2026-02-26
problem_type: runtime_error
component: service_object
symptoms:
  - "DELETE /v1/skills/:name/:version returns 204 for non-existent skills instead of 404"
  - "GET /v1/skills/:name/:version/download returns 500 for missing skills instead of 404"
  - "POST /v1/executions returns 500 when skill not found instead of 404"
root_cause: wrong_api
resolution_type: code_fix
severity: high
tags: [minio, error-handling, sentinel-errors, s3, go, docker]
---

# Troubleshooting: MinIO Error Sentinels Not Propagated Through Registry/Runner

## Problem

Three related bugs caused incorrect HTTP status codes when operating on non-existent skills. MinIO's S3-compatible API has silent-success behaviors and non-standard error types that weren't being detected or translated into the application's sentinel error types (`registry.ErrSkillNotFound`, `runner.ErrSkillNotFound`).

## Environment

- Module: Skillbox (registry + runner packages)
- Go Version: 1.22+
- Affected Components: `internal/registry/registry.go`, `internal/runner/runner.go`
- Dependencies: `github.com/minio/minio-go/v7`
- Date: 2026-02-26

## Symptoms

1. **DELETE returning 204 for non-existent skills**: `registry.Delete` silently succeeded because MinIO's `RemoveObject` does not return an error when the key doesn't exist — it's a no-op by S3 design.
2. **Download returning 500 instead of 404**: `registry.Download` called `GetObject` which returns a lazy reader (no immediate error). The subsequent `obj.Stat()` returned a `minio.ErrorResponse` with `Code: "NoSuchKey"`, but this wasn't being matched to `ErrSkillNotFound`.
3. **Execute returning 500 instead of 404**: `runner.Run` received `registry.ErrSkillNotFound` from the registry layer but didn't translate it to `runner.ErrSkillNotFound` — these are different sentinel values in different packages, and the HTTP handler only checks for `runner.ErrSkillNotFound`.

## What Didn't Work

**Direct solution:** The three bugs were identified through comprehensive E2E testing and fixed on the first attempt once root causes were understood. The non-obvious part was understanding MinIO's behavior:

- `RemoveObject` silently succeeds for missing keys (S3 spec behavior)
- `GetObject` returns a lazy reader, not an immediate error
- `minio.ErrorResponse` is a struct type requiring `errors.As`, not `errors.Is`

## Solution

### Bug 1: `registry.Delete` — Add existence check before removal

```go
// Before (broken):
func (r *Registry) Delete(ctx context.Context, tenantID, name, version string) error {
    key := objectKey(tenantID, name, version)
    return r.client.RemoveObject(ctx, r.bucket, key, minio.RemoveObjectOptions{})
}

// After (fixed):
func (r *Registry) Delete(ctx context.Context, tenantID, name, version string) error {
    key := objectKey(tenantID, name, version)
    _, err := r.client.StatObject(ctx, r.bucket, key, minio.StatObjectOptions{})
    if err != nil {
        errResp := minio.ErrorResponse{}
        if errors.As(err, &errResp) && errResp.Code == "NoSuchKey" {
            return ErrSkillNotFound
        }
        return fmt.Errorf("checking skill archive %q: %w", key, err)
    }
    return r.client.RemoveObject(ctx, r.bucket, key, minio.RemoveObjectOptions{})
}
```

### Bug 2: `registry.Download` — Detect NoSuchKey on Stat

```go
// Before (broken):
func (r *Registry) Download(ctx context.Context, tenantID, name, version string) (io.ReadCloser, error) {
    key := objectKey(tenantID, name, version)
    obj, err := r.client.GetObject(ctx, r.bucket, key, minio.GetObjectOptions{})
    if err != nil {
        return nil, fmt.Errorf("downloading skill archive %q: %w", key, err)
    }
    return obj, nil  // No existence check — GetObject is lazy!
}

// After (fixed):
func (r *Registry) Download(ctx context.Context, tenantID, name, version string) (io.ReadCloser, error) {
    key := objectKey(tenantID, name, version)
    obj, err := r.client.GetObject(ctx, r.bucket, key, minio.GetObjectOptions{})
    if err != nil {
        return nil, fmt.Errorf("downloading skill archive %q: %w", key, err)
    }
    // Force an immediate check — GetObject is lazy and won't error on missing keys.
    if _, err := obj.Stat(); err != nil {
        obj.Close()
        errResp := minio.ErrorResponse{}
        if errors.As(err, &errResp) && errResp.Code == "NoSuchKey" {
            return nil, ErrSkillNotFound
        }
        return nil, fmt.Errorf("skill archive %q not found or inaccessible: %w", key, err)
    }
    return obj, nil
}
```

### Bug 3: `runner.Run` — Translate registry sentinel to runner sentinel

```go
// Before (broken):
if req.Version == "" || req.Version == "latest" {
    resolved, resolveErr := r.registry.ResolveLatest(ctx, req.TenantID, req.Skill)
    if resolveErr != nil {
        return nil, fmt.Errorf("resolving latest version for %s: %w", req.Skill, resolveErr)
    }
    req.Version = resolved
}

// After (fixed):
if req.Version == "" || req.Version == "latest" {
    resolved, resolveErr := r.registry.ResolveLatest(ctx, req.TenantID, req.Skill)
    if resolveErr != nil {
        if errors.Is(resolveErr, registry.ErrSkillNotFound) {
            return nil, ErrSkillNotFound  // Translate to runner's sentinel
        }
        return nil, fmt.Errorf("resolving latest version for %s: %w", req.Skill, resolveErr)
    }
    req.Version = resolved
}
```

## Why This Works

**Root Cause:** Three failures in the error propagation chain:

1. **S3 API semantics mismatch**: MinIO follows the S3 spec where `RemoveObject` is idempotent (deleting a non-existent key is not an error). Go developers often assume "delete non-existent = error". The fix adds an explicit `StatObject` guard.

2. **Lazy reader pattern**: MinIO's `GetObject` returns a reader object immediately without checking if the key exists. The actual S3 GET only happens when you read or stat the object. The fix calls `obj.Stat()` eagerly to force the existence check.

3. **Sentinel error boundary crossing**: Go's sentinel errors (`var ErrSkillNotFound = errors.New(...)`) are package-scoped values. `registry.ErrSkillNotFound` and `runner.ErrSkillNotFound` are distinct values. The HTTP handler only checks `errors.Is(err, runner.ErrSkillNotFound)`, so if the registry's sentinel leaks through unwrapped, the handler falls through to a generic 500. The fix translates at the package boundary using `errors.Is`.

**Key insight about `minio.ErrorResponse`**: It's a struct type, not a sentinel. You must use `errors.As(err, &errResp)` to extract it, then check `errResp.Code == "NoSuchKey"`. Using `errors.Is` won't work because each error instance is unique.

## Prevention

- **Always `Stat()` after `GetObject()`** in MinIO/S3 code — the reader is lazy and won't surface missing-key errors until you read from it.
- **Never trust `RemoveObject` return values** for existence checks — S3 deletes are idempotent by design. Use `StatObject` first if you need to verify existence.
- **Translate sentinel errors at package boundaries** — when one package wraps another, explicitly check for the inner package's sentinels and map them to the outer package's equivalent.
- **Use `errors.As` for MinIO errors, not `errors.Is`** — `minio.ErrorResponse` is a struct carrying an error code string, not a comparable sentinel.
- **E2E tests should cover error paths** — the bugs were invisible to unit tests but immediately caught by E2E tests checking HTTP status codes for non-existent resources.

## Related Issues

No related issues documented yet.
