package artifacts

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Collector handles the packaging and uploading of file artifacts produced
// by skill executions. Output files are tar-gzipped and stored in MinIO
// with a presigned download URL.
type Collector struct {
	client *minio.Client
	bucket string
}

// New creates a Collector connected to the given S3/MinIO endpoint. It
// ensures the target bucket exists, creating it if necessary.
func New(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Collector, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating minio client for artifacts: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("checking artifacts bucket %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("creating artifacts bucket %q: %w", bucket, err)
		}
	}

	return &Collector{
		client: client,
		bucket: bucket,
	}, nil
}

// Collect packages all files found in filesDir into a tar.gz archive,
// uploads it to MinIO at {tenantID}/{executionID}/files.tar.gz, and
// returns a presigned GET URL (1 hour TTL) along with the list of
// relative file paths. If filesDir is empty or contains no files, it
// returns empty values with no error.
func (c *Collector) Collect(ctx context.Context, tenantID, executionID, filesDir string) (filesURL string, filesList []string, err error) {
	// Walk the files directory and collect all regular files.
	filesList, err = listFiles(filesDir)
	if err != nil {
		return "", nil, fmt.Errorf("listing artifact files: %w", err)
	}
	if len(filesList) == 0 {
		return "", nil, nil
	}

	// Create tar.gz archive in memory.
	archive, err := createTarGz(filesDir, filesList)
	if err != nil {
		return "", nil, fmt.Errorf("creating tar.gz archive: %w", err)
	}

	// Upload to MinIO.
	key := fmt.Sprintf("%s/%s/files.tar.gz", tenantID, executionID)
	_, err = c.client.PutObject(ctx, c.bucket, key, archive, int64(archive.Len()), minio.PutObjectOptions{
		ContentType: "application/gzip",
	})
	if err != nil {
		return "", nil, fmt.Errorf("uploading artifact archive to %q: %w", key, err)
	}

	// Generate presigned URL with 1 hour TTL.
	reqParams := make(url.Values)
	presignedURL, err := c.client.PresignedGetObject(ctx, c.bucket, key, 1*time.Hour, reqParams)
	if err != nil {
		return "", nil, fmt.Errorf("generating presigned URL for %q: %w", key, err)
	}

	return presignedURL.String(), filesList, nil
}

// listFiles walks the given directory and returns relative paths of all
// regular files found. It skips directories, symlinks, and other non-regular
// entries.
func listFiles(dir string) ([]string, error) {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dir)
	}

	var files []string
	err = filepath.Walk(dir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return files, nil
}

// createTarGz builds a tar.gz archive from the listed files rooted at baseDir.
// It returns the archive as a bytes.Buffer ready for upload.
func createTarGz(baseDir string, files []string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, rel := range files {
		absPath := filepath.Join(baseDir, rel)

		// Sanitize: reject any path that escapes the base directory.
		cleaned := filepath.Clean(absPath)
		if !strings.HasPrefix(cleaned, filepath.Clean(baseDir)+string(filepath.Separator)) &&
			cleaned != filepath.Clean(baseDir) {
			return nil, fmt.Errorf("file path %q escapes base directory", rel)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", absPath, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return nil, fmt.Errorf("creating tar header for %q: %w", rel, err)
		}
		// Use the relative path in the archive so extraction is clean.
		header.Name = filepath.ToSlash(rel)

		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("writing tar header for %q: %w", rel, err)
		}

		f, err := os.Open(absPath)
		if err != nil {
			return nil, fmt.Errorf("opening %q: %w", absPath, err)
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return nil, fmt.Errorf("writing %q to tar: %w", rel, err)
		}
		f.Close()
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("closing gzip writer: %w", err)
	}

	return &buf, nil
}
