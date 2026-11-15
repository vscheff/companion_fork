package companion

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	SourceID      string    `json:"sourceId"`
	Edition       string    `json:"edition"`
	FileName      string    `json:"fileName"`
	SHA256        string    `json:"sha256"`
	Bytes         int64     `json:"bytes"`
	FileModTime   time.Time `json:"fileModTime"`
	CreatedAt     time.Time `json:"createdAt"`
	AppVersion    string    `json:"appVersion"`
	Kind          string    `json:"kind"`
	Mode          string    `json:"mode"`
	FileSHA256    string    `json:"fileSha256,omitempty"`
	Rows          int       `json:"rows,omitempty"`
	Addon         string    `json:"addon,omitempty"`
	Realm         string    `json:"realm,omitempty"`
	Character     string    `json:"character,omitempty"`
	AccountID     string    `json:"accountId,omitempty"`
}

const (
	KindCensus  = "wowcensus"
	KindAddon   = "addon"
	ModeFull    = "full"
	ModeSession = "session"
)

type Bundle struct {
	Manifest Manifest
	Zip      []byte
	ZipSHA   string
}

func NewManifest(snap *Snapshot, sourceID string) Manifest {
	m := Manifest{
		SchemaVersion: 1,
		SourceID:      sourceID,
		Edition:       snap.File.Edition,
		FileName:      SavedVariablesName,
		SHA256:        snap.SHA256,
		Bytes:         int64(len(snap.Data)),
		FileModTime:   snap.Sig.ModTime.UTC(),
		CreatedAt:     snap.TakenAt.UTC(),
		AppVersion:    AppVersion,
		Kind:          KindCensus,
		Mode:          ModeFull,
		FileSHA256:    snap.SHA256,
	}
	return m
}

func (m *Manifest) SetData(data []byte) {
	sum := sha256.Sum256(data)
	m.SHA256 = hex.EncodeToString(sum[:])
	m.Bytes = int64(len(data))
}

func BuildBundle(snap *Snapshot, m Manifest) (*Bundle, error) {
	return BuildBundleData(m, snap.Data)
}

func BuildBundleData(m Manifest, data []byte) (*Bundle, error) {
	if m.FileName == "" {
		m.FileName = SavedVariablesName
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mj, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	stamp := m.CreatedAt
	for _, f := range []struct {
		name string
		data []byte
	}{{"manifest.json", mj}, {m.FileName, data}} {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: stamp})
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(buf.Bytes())
	return &Bundle{Manifest: m, Zip: buf.Bytes(), ZipSHA: hex.EncodeToString(sum[:])}, nil
}
