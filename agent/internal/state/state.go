// Package state owns the status and DNS snapshot files that the plugin reads.
// Everything is written atomically (temp file + rename) so the web UI can
// poll at any time and never observe a partially written document.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DNSEntry struct {
	Domain string `json:"domain"`
	IP     string `json:"ip"`
	Type   string `json:"type"`
	SiteID int    `json:"siteId"`
}

type SubnetRouterStatus struct {
	Enabled       bool     `json:"enabled"`
	Backend       string   `json:"backend"`
	TunnelIP      string   `json:"tunnelIp,omitempty"`
	LocalNetworks []string `json:"localNetworks,omitempty"`
	Applied       bool     `json:"applied"`
	LastError     string   `json:"lastError,omitempty"`
}

type Status struct {
	Version   string `json:"version"`
	Mode      string `json:"mode"`
	PID       int    `json:"pid"`
	StartedAt string `json:"startedAt"`
	UpdatedAt string `json:"updatedAt"`

	Connected  bool     `json:"connected"`
	Registered bool     `json:"registered"`
	TunnelIP   string   `json:"tunnelIp,omitempty"`
	Interface  string   `json:"interface,omitempty"`
	DNSProxyIP string   `json:"dnsProxyIp,omitempty"`
	Routes     []string `json:"routes,omitempty"`

	DNSRecords []DNSEntry `json:"dnsRecords,omitempty"`

	LastError   string `json:"lastError,omitempty"`
	LastErrorAt string `json:"lastErrorAt,omitempty"`

	HAActive bool   `json:"haActive"`
	HANote   string `json:"haNote,omitempty"`

	SubnetRouter *SubnetRouterStatus `json:"subnetRouter,omitempty"`
}

// Store serialises writes for one agent process.
type Store struct {
	dir       string
	mode      string
	dnsPath   string
	mu        sync.Mutex
	status    Status
	dirty     bool
	lastWrite time.Time
}

func New(dir, mode string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create state directory: %w", err)
	}
	return &Store{
		dir:     dir,
		mode:    mode,
		dnsPath: filepath.Join(dir, DefaultDNSOutput),
	}, nil
}

const DefaultDNSOutput = "dns-records.json"

// SetDNSRecordsPath overrides the DNS snapshot location. It must be called
// before the status writer starts.
func (s *Store) SetDNSRecordsPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if path != "" {
		s.dnsPath = path
	}
}

func (s *Store) StatusPath() string {
	return filepath.Join(s.dir, s.mode+"-status.json")
}

func (s *Store) DNSRecordsPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dnsPath
}

func (s *Store) HealthPath() string {
	return filepath.Join(s.dir, s.mode+".health")
}

func (s *Store) Init(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	s.status = Status{
		Version:   version,
		Mode:      s.mode,
		PID:       os.Getpid(),
		StartedAt: now,
		UpdatedAt: now,
		HAActive:  true,
	}
	s.dirty = true
	_ = s.writeStatusLocked()
}

func (s *Store) Snapshot() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneStatus(s.status)
}

// Update applies fn to the current status and writes it if the revision
// changed. The callback must not retain the status pointer.
func (s *Store) Update(fn func(*Status)) (Status, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, err := json.Marshal(s.status)
	if err != nil {
		return cloneStatus(s.status), false, err
	}
	fn(&s.status)
	after, err := json.Marshal(s.status)
	if err != nil {
		return cloneStatus(s.status), false, err
	}
	if string(before) == string(after) {
		if !s.dirty {
			return cloneStatus(s.status), false, nil
		}
		/* A previous write failed. Retry even when the in-memory status did
		 * not change so the file eventually catches up. */
		return cloneStatus(s.status), false, s.writeStatusLocked()
	}
	s.status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.dirty = true
	return cloneStatus(s.status), true, s.writeStatusLocked()
}

// Touch refreshes UpdatedAt at most once per interval. The plugin uses this
// timestamp to decide whether a status file still belongs to a live agent.
// Passing zero forces a write. Dirty state is always retried.
func (s *Store) Touch(interval time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !s.dirty && interval > 0 && !s.lastWrite.IsZero() && now.Sub(s.lastWrite) < interval {
		return nil
	}
	s.status.UpdatedAt = now.UTC().Format(time.RFC3339)
	s.dirty = true
	return s.writeStatusLocked()
}

func (s *Store) SetConnected(connected bool, tunnelIP, iface string, routes []string) (Status, bool, error) {
	return s.Update(func(st *Status) {
		st.Connected = connected
		if tunnelIP != "" {
			st.TunnelIP = tunnelIP
		}
		if iface != "" {
			st.Interface = iface
		}
		if routes != nil {
			st.Routes = append([]string(nil), routes...)
		}
		if !connected {
			st.Registered = false
		}
	})
}

func (s *Store) SetError(message string) (Status, bool, error) {
	return s.Update(func(st *Status) {
		if message == "" {
			st.LastError = ""
			st.LastErrorAt = ""
			return
		}
		st.LastError = message
		st.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
	})
}

func (s *Store) SetDNSProxyIP(ip string) (Status, bool, error) {
	return s.Update(func(st *Status) {
		st.DNSProxyIP = ip
	})
}

func (s *Store) SetHA(active bool, note string) (Status, bool, error) {
	return s.Update(func(st *Status) {
		st.HAActive = active
		st.HANote = note
	})
}

func (s *Store) SetSubnetRouter(sr *SubnetRouterStatus) (Status, bool, error) {
	var copy *SubnetRouterStatus
	if sr != nil {
		c := *sr
		c.LocalNetworks = append([]string(nil), sr.LocalNetworks...)
		copy = &c
	}
	return s.Update(func(st *Status) {
		st.SubnetRouter = copy
	})
}

// WriteDNSRecords atomically replaces the DNS snapshot. The snapshot is kept
// separate from status so a large alias set does not make every status poll
// expensive. The file is written first: a failed status write must not leave a
// new snapshot in memory that the plugin cannot see on disk.
func (s *Store) WriteDNSRecords(records []DNSEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if records == nil {
		records = []DNSEntry{}
	}
	if err := writeAtomic(s.dnsPath, records, 0o644); err != nil {
		return err
	}
	s.status.DNSRecords = append([]DNSEntry(nil), records...)
	s.dirty = true
	return s.writeStatusLocked()
}

func (s *Store) ReadDNSRecords() ([]DNSEntry, error) {
	data, err := os.ReadFile(s.DNSRecordsPath())
	if err != nil {
		return nil, err
	}
	var out []DNSEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) writeStatusLocked() error {
	if err := writeAtomic(s.StatusPath(), s.status, 0o644); err != nil {
		s.dirty = true
		return err
	}
	s.dirty = false
	s.lastWrite = time.Now()
	return nil
}

func writeAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".pangolin-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func cloneStatus(st Status) Status {
	out := st
	if st.Routes != nil {
		out.Routes = append([]string(nil), st.Routes...)
	}
	if st.DNSRecords != nil {
		out.DNSRecords = append([]DNSEntry(nil), st.DNSRecords...)
	}
	if st.SubnetRouter != nil {
		sr := *st.SubnetRouter
		if sr.LocalNetworks != nil {
			sr.LocalNetworks = append([]string(nil), sr.LocalNetworks...)
		}
		out.SubnetRouter = &sr
	}
	return out
}

// ReadStatus loads a status file written by a (possibly different) agent
// process. It is used by the `status` subcommand and tests.
func ReadStatus(path string) (Status, error) {
	var st Status
	data, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(data, &st)
	return st, err
}

// Healthy reports whether the tunnel's health file is fresh. Newt rewrites it
// on every successful ping and removes it on failure. A missing file is not
// treated as healthy.
func (s *Store) Healthy(maxAge time.Duration) bool {
	info, err := os.Stat(s.HealthPath())
	if err != nil {
		return false
	}
	if maxAge <= 0 {
		maxAge = 90 * time.Second
	}
	return time.Since(info.ModTime()) <= maxAge
}
