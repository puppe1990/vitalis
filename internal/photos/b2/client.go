// Package b2 talks to the Backblaze B2 Native API (v2): authorize, upload,
// download-by-name and delete. Tokens live in memory and refresh on 401.
package b2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/puppe1990/vitalis/internal/photos"
)

const defaultEndpoint = "https://api.backblazeb2.com"

// Config is one application key scoped to a single bucket.
type Config struct {
	KeyID    string
	AppKey   string
	Bucket   string
	Endpoint string
	HTTP     *http.Client
}

// Client implements photos.Bucket against B2.
type Client struct {
	cfg  Config
	http *http.Client

	mu   sync.Mutex
	auth *authSession
}

type authSession struct {
	accountID   string
	apiURL      string
	downloadURL string
	token       string
	bucketID    string
	bucketName  string
}

func New(cfg Config) *Client {
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEndpoint
	}
	httpClient := cfg.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	return &Client{cfg: cfg, http: httpClient}
}

// FromEnv builds a client from B2_APPLICATION_KEY_ID, B2_APPLICATION_KEY and
// B2_BUCKET. All three empty disables photos (nil, nil). A partial set is a
// boot error so a typo cannot silently hide the upload form.
func FromEnv() (photos.Bucket, error) {
	id := strings.TrimSpace(os.Getenv("B2_APPLICATION_KEY_ID"))
	key := strings.TrimSpace(os.Getenv("B2_APPLICATION_KEY"))
	bucket := strings.TrimSpace(os.Getenv("B2_BUCKET"))
	switch {
	case id == "" && key == "" && bucket == "":
		return nil, nil
	case id == "" || key == "" || bucket == "":
		return nil, fmt.Errorf("B2_APPLICATION_KEY_ID, B2_APPLICATION_KEY and B2_BUCKET must all be set")
	}
	endpoint := strings.TrimSpace(os.Getenv("B2_ENDPOINT"))
	return New(Config{KeyID: id, AppKey: key, Bucket: bucket, Endpoint: endpoint}), nil
}

func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	sess, err := c.session(ctx)
	if err != nil {
		return err
	}
	var upload struct {
		UploadURL          string `json:"uploadUrl"`
		AuthorizationToken string `json:"authorizationToken"`
	}
	if err := c.post(ctx, "/b2_get_upload_url", map[string]any{"bucketId": sess.bucketID}, &upload); err != nil {
		return err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upload.UploadURL, body)
	if err != nil {
		return fmt.Errorf("b2 upload: %w", err)
	}
	req.ContentLength = size
	req.Header.Set("Authorization", upload.AuthorizationToken)
	req.Header.Set("X-Bz-File-Name", encodeKey(key))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-Bz-Content-Sha1", "do_not_verify")

	var result struct {
		FileID   string `json:"fileId"`
		FileName string `json:"fileName"`
	}
	return c.do(req, &result)
}

func (c *Client) Open(ctx context.Context, key string) (photos.Object, error) {
	sess, err := c.session(ctx)
	if err != nil {
		return photos.Object{}, err
	}
	dest := sess.downloadURL + "/file/" + url.PathEscape(sess.bucketName) + "/" + encodePath(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dest, nil)
	if err != nil {
		return photos.Object{}, fmt.Errorf("b2 download %s: %w", key, err)
	}
	req.Header.Set("Authorization", sess.token)
	res, err := c.http.Do(req)
	if err != nil {
		return photos.Object{}, fmt.Errorf("b2 download %s: %w", key, err)
	}
	if res.StatusCode != http.StatusOK {
		defer func() { _ = res.Body.Close() }()
		return photos.Object{}, fmt.Errorf("b2 download %s: HTTP %d", key, res.StatusCode)
	}
	ctype := res.Header.Get("Content-Type")
	return photos.Object{Body: res.Body, ContentType: ctype}, nil
}

func (c *Client) Remove(ctx context.Context, key string) error {
	file, err := c.findFile(ctx, key)
	if err != nil {
		return err
	}
	return c.post(ctx, "/b2_delete_file_version", map[string]any{
		"fileName": file.FileName, "fileId": file.FileID,
	}, nil)
}

type b2File struct {
	FileID   string `json:"fileId"`
	FileName string `json:"fileName"`
}

func (c *Client) findFile(ctx context.Context, key string) (b2File, error) {
	sess, err := c.session(ctx)
	if err != nil {
		return b2File{}, err
	}
	var payload struct {
		Files []b2File `json:"files"`
	}
	if err := c.post(ctx, "/b2_list_file_names", map[string]any{
		"bucketId": sess.bucketID, "prefix": key, "maxFileCount": 1,
	}, &payload); err != nil {
		return b2File{}, err
	}
	for _, file := range payload.Files {
		if file.FileName == key {
			return file, nil
		}
	}
	return b2File{}, fmt.Errorf("%w: %s", photos.ErrNotFound, key)
}

func (c *Client) session(ctx context.Context) (*authSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.auth != nil {
		return c.auth, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.cfg.Endpoint, "/")+"/b2api/v2/b2_authorize_account", nil)
	if err != nil {
		return nil, fmt.Errorf("b2 authorize: %w", err)
	}
	req.SetBasicAuth(c.cfg.KeyID, c.cfg.AppKey)

	var payload struct {
		AccountID          string `json:"accountId"`
		AuthorizationToken string `json:"authorizationToken"`
		APIURL             string `json:"apiUrl"`
		DownloadURL        string `json:"downloadUrl"`
		Allowed            struct {
			BucketID   string `json:"bucketId"`
			BucketName string `json:"bucketName"`
		} `json:"allowed"`
	}
	if err := c.do(req, &payload); err != nil {
		return nil, err
	}
	if payload.AuthorizationToken == "" || payload.APIURL == "" {
		return nil, fmt.Errorf("b2 authorize: empty response")
	}
	bucketID := payload.Allowed.BucketID
	bucketName := payload.Allowed.BucketName
	if bucketName == "" {
		bucketName = c.cfg.Bucket
	}
	if bucketID == "" {
		id, err := c.lookupBucketID(ctx, payload.APIURL, payload.AuthorizationToken, payload.AccountID)
		if err != nil {
			return nil, err
		}
		bucketID = id
	}
	c.auth = &authSession{
		accountID:   payload.AccountID,
		apiURL:      strings.TrimRight(payload.APIURL, "/"),
		downloadURL: strings.TrimRight(payload.DownloadURL, "/"),
		token:       payload.AuthorizationToken,
		bucketID:    bucketID,
		bucketName:  bucketName,
	}
	return c.auth, nil
}

func (c *Client) lookupBucketID(ctx context.Context, apiURL, token, accountID string) (string, error) {
	body, err := json.Marshal(map[string]any{"accountId": accountID, "bucketName": c.cfg.Bucket})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(apiURL, "/")+"/b2api/v2/b2_list_buckets", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")
	var payload struct {
		Buckets []struct {
			BucketID   string `json:"bucketId"`
			BucketName string `json:"bucketName"`
		} `json:"buckets"`
	}
	if err := c.do(req, &payload); err != nil {
		return "", err
	}
	for _, b := range payload.Buckets {
		if b.BucketName == c.cfg.Bucket {
			return b.BucketID, nil
		}
	}
	return "", fmt.Errorf("b2 bucket %q not found", c.cfg.Bucket)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	sess, err := c.session(ctx)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("b2 %s: %w", path, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sess.apiURL+"/b2api/v2"+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("b2 %s: %w", path, err)
	}
	req.Header.Set("Authorization", sess.token)
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("b2 %s: %w", req.URL.Path, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("b2 %s: HTTP %d: %s", req.URL.Path, res.StatusCode, bytes.TrimSpace(snippet))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func encodeKey(key string) string {
	return strings.ReplaceAll(url.QueryEscape(key), "+", "%20")
}

func encodePath(key string) string {
	parts := strings.Split(key, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}
