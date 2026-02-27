package registry

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrSkillNotFound is returned when a skill cannot be located in storage.
var ErrSkillNotFound = errors.New("registry: skill not found")

// SkillMeta contains summary information about a skill version stored in
// the registry. It is returned by List.
type SkillMeta struct {
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// Registry provides skill archive storage backed by MinIO / S3-compatible
// object storage. Archives are stored as zip files keyed by tenant, skill
// name, and version.
type Registry struct {
	client *minio.Client
	bucket string
}

// New creates a Registry connected to the given S3/MinIO endpoint. It
// ensures the target bucket exists, creating it if necessary.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Registry, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating minio client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("checking bucket %q existence: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("creating bucket %q: %w", bucket, err)
		}
	}

	return &Registry{
		client: client,
		bucket: bucket,
	}, nil
}

// objectPath returns the S3 key for a skill archive.
func objectPath(tenantID, skillName, version string) string {
	return path.Join(tenantID, skillName, version, "skill.zip")
}

// Upload stores a skill zip archive in the registry after validating that
// the provided data is a valid zip file (by checking zip headers). The
// archive is stored at {tenantID}/{skillName}/{version}/skill.zip.
func (r *Registry) Upload(ctx context.Context, tenantID, skillName, version string, zipData io.Reader, size int64) error {
	if tenantID == "" || skillName == "" || version == "" {
		return fmt.Errorf("tenantID, skillName, and version are required")
	}

	// Read the full zip into memory so we can validate the zip headers
	// before uploading. This prevents storing corrupt archives.
	buf, err := io.ReadAll(io.LimitReader(zipData, size+1))
	if err != nil {
		return fmt.Errorf("reading zip data: %w", err)
	}
	if int64(len(buf)) > size {
		return fmt.Errorf("zip data exceeds declared size of %d bytes", size)
	}

	// Validate zip headers by attempting to open the archive.
	if _, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf))); err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}

	key := objectPath(tenantID, skillName, version)
	_, err = r.client.PutObject(ctx, r.bucket, key, bytes.NewReader(buf), int64(len(buf)), minio.PutObjectOptions{
		ContentType: "application/zip",
	})
	if err != nil {
		return fmt.Errorf("uploading skill archive to %q: %w", key, err)
	}

	return nil
}

// Download returns a reader for the skill zip archive. The caller is
// responsible for closing the returned ReadCloser.
func (r *Registry) Download(ctx context.Context, tenantID, skillName, version string) (io.ReadCloser, error) {
	if tenantID == "" || skillName == "" || version == "" {
		return nil, fmt.Errorf("tenantID, skillName, and version are required")
	}

	key := objectPath(tenantID, skillName, version)
	obj, err := r.client.GetObject(ctx, r.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("downloading skill archive %q: %w", key, err)
	}

	// Verify the object is actually accessible by reading its stat.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		// MinIO returns an ErrorResponse with Code "NoSuchKey" for missing objects.
		// Map this to our sentinel so callers can distinguish "not found" from other errors.
		errResp := minio.ErrorResponse{}
		if errors.As(err, &errResp) && errResp.Code == "NoSuchKey" {
			return nil, ErrSkillNotFound
		}
		return nil, fmt.Errorf("skill archive %q not found or inaccessible: %w", key, err)
	}

	return obj, nil
}

// Delete removes a skill archive from the registry.
// It returns ErrSkillNotFound if the archive does not exist.
func (r *Registry) Delete(ctx context.Context, tenantID, skillName, version string) error {
	if tenantID == "" || skillName == "" || version == "" {
		return fmt.Errorf("tenantID, skillName, and version are required")
	}

	key := objectPath(tenantID, skillName, version)

	// Verify the object exists before attempting removal.
	// MinIO's RemoveObject succeeds silently for non-existent keys.
	_, err := r.client.StatObject(ctx, r.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ErrorResponse{}
		if errors.As(err, &errResp) && errResp.Code == "NoSuchKey" {
			return ErrSkillNotFound
		}
		return fmt.Errorf("checking skill archive %q: %w", key, err)
	}

	if err := r.client.RemoveObject(ctx, r.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("deleting skill archive %q: %w", key, err)
	}

	return nil
}

// ResolveLatest returns the version string of the most recently uploaded
// version of a skill. If no versions are found, it returns ErrSkillNotFound.
func (r *Registry) ResolveLatest(ctx context.Context, tenantID, skillName string) (string, error) {
	if tenantID == "" || skillName == "" {
		return "", fmt.Errorf("tenantID and skillName are required")
	}

	prefix := path.Join(tenantID, skillName) + "/"
	var latest string
	var latestTime time.Time

	for obj := range r.client.ListObjects(ctx, r.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return "", fmt.Errorf("listing objects with prefix %q: %w", prefix, obj.Err)
		}
		if !strings.HasSuffix(obj.Key, "/skill.zip") {
			continue
		}

		// Extract version from path: {tenantID}/{skillName}/{version}/skill.zip
		relative := strings.TrimPrefix(obj.Key, prefix)
		version := strings.TrimSuffix(relative, "/skill.zip")

		if latest == "" || obj.LastModified.After(latestTime) {
			latest = version
			latestTime = obj.LastModified
		}
	}

	if latest == "" {
		return "", ErrSkillNotFound
	}
	return latest, nil
}

// List returns metadata for all skill versions belonging to a tenant.
// It works by iterating over object prefixes under the tenant's namespace
// and extracting skill name, version, and upload timestamp from each
// matching object.
func (r *Registry) List(ctx context.Context, tenantID string) ([]SkillMeta, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	prefix := tenantID + "/"
	var skills []SkillMeta

	for obj := range r.client.ListObjects(ctx, r.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("listing objects with prefix %q: %w", prefix, obj.Err)
		}

		// Expected key format: {tenantID}/{skillName}/{version}/skill.zip
		if !strings.HasSuffix(obj.Key, "/skill.zip") {
			continue
		}

		// Strip the tenant prefix and the trailing "/skill.zip".
		relative := strings.TrimPrefix(obj.Key, prefix)
		relative = strings.TrimSuffix(relative, "/skill.zip")

		parts := strings.SplitN(relative, "/", 2)
		if len(parts) != 2 {
			continue
		}

		skills = append(skills, SkillMeta{
			Name:       parts[0],
			Version:    parts[1],
			UploadedAt: obj.LastModified,
		})
	}

	return skills, nil
}
