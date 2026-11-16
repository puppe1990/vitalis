package b2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/puppe1990/vitalis/internal/photos"
)

func TestClient_PutOpenRemove(t *testing.T) {
	srv := fakeB2(t)
	client := New(Config{
		KeyID:    "key-id",
		AppKey:   "key-secret",
		Bucket:   "vitalis-photos",
		Endpoint: srv.URL,
		HTTP:     srv.Client(),
	})

	ctx := context.Background()
	body := []byte{0xff, 0xd8, 0xff, 0xe0}
	if err := client.Put(ctx, "users/1/2026-11-16/almoco/a.jpg", bytes.NewReader(body), int64(len(body)), "image/jpeg"); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if srv.lastUpload.fileName != "users/1/2026-11-16/almoco/a.jpg" {
		t.Errorf("uploaded name = %q", srv.lastUpload.fileName)
	}
	if srv.lastUpload.contentType != "image/jpeg" {
		t.Errorf("uploaded type = %q", srv.lastUpload.contentType)
	}
	if !bytes.Equal(srv.lastUpload.body, body) {
		t.Errorf("uploaded body = %v", srv.lastUpload.body)
	}

	obj, err := client.Open(ctx, "users/1/2026-11-16/almoco/a.jpg")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = obj.Body.Close() }()
	got, _ := io.ReadAll(obj.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("downloaded = %v", got)
	}
	if obj.ContentType != "image/jpeg" {
		t.Errorf("download type = %q", obj.ContentType)
	}

	if err := client.Remove(ctx, "users/1/2026-11-16/almoco/a.jpg"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if srv.deletedFileID != "file-1" {
		t.Errorf("deleted file id = %q", srv.deletedFileID)
	}
}

func TestClient_PutRejectsBadAuth(t *testing.T) {
	srv := fakeB2(t)
	client := New(Config{
		KeyID: "wrong", AppKey: "nope", Bucket: "vitalis-photos",
		Endpoint: srv.URL, HTTP: srv.Client(),
	})
	err := client.Put(context.Background(), "a.jpg", bytes.NewReader([]byte{0xff, 0xd8, 0xff}), 3, "image/jpeg")
	if err == nil {
		t.Fatal("Put with bad credentials must fail")
	}
}

func TestFromEnv_disabledWhenUnset(t *testing.T) {
	t.Setenv("B2_APPLICATION_KEY_ID", "")
	t.Setenv("B2_APPLICATION_KEY", "")
	t.Setenv("B2_BUCKET", "")
	bucket, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if bucket != nil {
		t.Error("unset env must return a nil bucket")
	}
}

func TestFromEnv_errorsWhenPartial(t *testing.T) {
	t.Setenv("B2_APPLICATION_KEY_ID", "id")
	t.Setenv("B2_APPLICATION_KEY", "")
	t.Setenv("B2_BUCKET", "vitalis-photos")
	_, err := FromEnv()
	if err == nil {
		t.Fatal("partial B2 env must error")
	}
}

type fakeB2Server struct {
	*httptest.Server
	files         map[string]storedFile
	lastUpload    storedFile
	deletedFileID string
}

type storedFile struct {
	fileName    string
	fileID      string
	contentType string
	body        []byte
}

func fakeB2(t *testing.T) *fakeB2Server {
	t.Helper()
	s := &fakeB2Server{files: map[string]storedFile{}}
	mux := http.NewServeMux()

	mux.HandleFunc("/b2api/v2/b2_authorize_account", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "key-id" || pass != "key-secret" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accountId":          "acc-1",
			"authorizationToken": "token-1",
			"apiUrl":             s.URL,
			"downloadUrl":        s.URL,
			"allowed":            map[string]any{"bucketId": "bucket-1", "bucketName": "vitalis-photos"},
		})
	})
	mux.HandleFunc("/b2api/v2/b2_get_upload_url", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token-1" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"uploadUrl":          s.URL + "/b2upload",
			"authorizationToken": "upload-token",
		})
	})
	mux.HandleFunc("/b2upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "upload-token" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		name, _ := url.QueryUnescape(r.Header.Get("X-Bz-File-Name"))
		file := storedFile{
			fileName:    name,
			fileID:      "file-1",
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		}
		s.lastUpload = file
		s.files[name] = file
		_ = json.NewEncoder(w).Encode(map[string]any{"fileId": "file-1", "fileName": name})
	})
	mux.HandleFunc("/b2api/v2/b2_list_file_names", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Prefix string `json:"prefix"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var files []map[string]any
		for name, file := range s.files {
			if strings.HasPrefix(name, req.Prefix) {
				files = append(files, map[string]any{
					"fileName": file.fileName, "fileId": file.fileID,
				})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
	})
	mux.HandleFunc("/b2api/v2/b2_delete_file_version", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			FileID   string `json:"fileId"`
			FileName string `json:"fileName"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		s.deletedFileID = req.FileID
		delete(s.files, req.FileName)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/file/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token-1" {
			http.Error(w, `{"code":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/file/vitalis-photos/")
		file, ok := s.files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", file.contentType)
		_, _ = w.Write(file.body)
	})

	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

var _ photos.Bucket = (*Client)(nil)
