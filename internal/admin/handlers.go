// Package admin — служебные эндпоинты для админ-панели.
//
// /api/admin/uptime — снимок состояния процесса и хостовой машины.
// Только для is_admin=TRUE юзеров (mw RequireAdmin).
package admin

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bogdan/kirpichmusic/internal/httpx"
)

type Handler struct {
	pool      *pgxpool.Pool
	startedAt time.Time
	mediaRoot string
	dbName    string
}

func NewHandler(pool *pgxpool.Pool, mediaRoot, dbName string) *Handler {
	return &Handler{
		pool:      pool,
		startedAt: time.Now().UTC(),
		mediaRoot: mediaRoot,
		dbName:    dbName,
	}
}

type uptimeResp struct {
	Server   serverInfo  `json:"server"`
	Runtime  runtimeInfo `json:"runtime"`
	Memory   memInfo     `json:"memory"`
	System   systemInfo  `json:"system"`
	Database dbInfo      `json:"database"`
	Disks    []diskInfo  `json:"disks"`
	Stats    statsInfo   `json:"stats"`
}

type serverInfo struct {
	StartedAt   string `json:"started_at"`
	UptimeSec   int64  `json:"uptime_sec"`
	GoVersion   string `json:"go_version"`
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Pid         int    `json:"pid"`
	NumCPU      int    `json:"num_cpu"`
	NumGoroutines int  `json:"num_goroutines"`
	Env         string `json:"env"`
}

type runtimeInfo struct {
	GCRuns      uint32  `json:"gc_runs"`
	LastGCAgeMs int64   `json:"last_gc_age_ms"`
	GCPauseMsP99 float64 `json:"gc_pause_ms_p99"`
}

type memInfo struct {
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes    uint64 `json:"heap_sys_bytes"`
	HeapInuseBytes  uint64 `json:"heap_inuse_bytes"`
	HeapObjects     uint64 `json:"heap_objects"`
	StackInuseBytes uint64 `json:"stack_inuse_bytes"`
	NumMallocs      uint64 `json:"num_mallocs"`
	NumFrees        uint64 `json:"num_frees"`
}

type systemInfo struct {
	LoadAvg1   float64 `json:"loadavg_1"`
	LoadAvg5   float64 `json:"loadavg_5"`
	LoadAvg15  float64 `json:"loadavg_15"`
	MemTotalKB uint64  `json:"mem_total_kb"`
	MemFreeKB  uint64  `json:"mem_free_kb"`
	MemAvailKB uint64  `json:"mem_avail_kb"`
	SwapTotalKB uint64 `json:"swap_total_kb"`
	SwapFreeKB  uint64 `json:"swap_free_kb"`
	UptimeSec  int64   `json:"uptime_sec"`
	CPUModel   string  `json:"cpu_model"`
	CPUMHz     float64 `json:"cpu_mhz"`
}

type dbInfo struct {
	Name           string `json:"name"`
	SizeBytes      int64  `json:"size_bytes"`
	ConnectionsTotal int   `json:"connections_total"`
	ConnectionsIdle  int   `json:"connections_idle"`
	ConnectionsBusy  int   `json:"connections_busy"`
	MaxConnections   int32 `json:"max_connections"`
	Version          string `json:"version"`
}

type diskInfo struct {
	Path       string  `json:"path"`
	TotalBytes uint64  `json:"total_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	UsedRatio  float64 `json:"used_ratio"`
}

type statsInfo struct {
	Users     int64 `json:"users"`
	Tracks    int64 `json:"tracks"`
	Playlists int64 `json:"playlists"`
	Likes     int64 `json:"likes"`
	Comments  int64 `json:"comments"`
	Follows   int64 `json:"follows"`
	Sessions  int64 `json:"sessions"`
}

// Uptime — GET /api/admin/uptime
func (h *Handler) Uptime(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	resp := uptimeResp{}

	// --- server / runtime ---
	hostname, _ := os.Hostname()
	resp.Server = serverInfo{
		StartedAt:     h.startedAt.Format(time.RFC3339),
		UptimeSec:     int64(now.Sub(h.startedAt).Seconds()),
		GoVersion:     runtime.Version(),
		Hostname:      hostname,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Pid:           os.Getpid(),
		NumCPU:        runtime.NumCPU(),
		NumGoroutines: runtime.NumGoroutine(),
		Env:           os.Getenv("ENV"),
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	resp.Memory = memInfo{
		HeapAllocBytes:  ms.HeapAlloc,
		HeapSysBytes:    ms.HeapSys,
		HeapInuseBytes:  ms.HeapInuse,
		HeapObjects:     ms.HeapObjects,
		StackInuseBytes: ms.StackInuse,
		NumMallocs:      ms.Mallocs,
		NumFrees:        ms.Frees,
	}
	if ms.LastGC > 0 {
		resp.Runtime.LastGCAgeMs = int64(time.Since(time.Unix(0, int64(ms.LastGC))) / time.Millisecond)
	}
	resp.Runtime.GCRuns = ms.NumGC
	if ms.NumGC > 0 {
		// Грубая оценка p99 из кольцевого буфера PauseNs (256 элементов).
		var max uint64
		for _, p := range ms.PauseNs {
			if p > max {
				max = p
			}
		}
		resp.Runtime.GCPauseMsP99 = float64(max) / 1e6
	}

	// --- системная инфа из /proc (Linux) ---
	resp.System = readLinuxSystem()

	// --- диски ---
	resp.Disks = []diskInfo{}
	if d, ok := statfs("/"); ok {
		d.Path = "/"
		resp.Disks = append(resp.Disks, d)
	}
	if h.mediaRoot != "" {
		if d, ok := statfs(h.mediaRoot); ok {
			d.Path = h.mediaRoot
			// Если совпадает с уже добавленным разделом — пропустим, чтобы не дублировать.
			dup := false
			for _, prev := range resp.Disks {
				if prev.TotalBytes == d.TotalBytes && prev.FreeBytes == d.FreeBytes {
					dup = true
					break
				}
			}
			if !dup {
				resp.Disks = append(resp.Disks, d)
			}
		}
	}

	// --- БД: размер, соединения, версия + статистика по таблицам ---
	resp.Database = h.readDatabase(r.Context())
	resp.Stats = h.readStats(r.Context())

	httpx.WriteJSON(w, http.StatusOK, resp)
}

// readLinuxSystem — читает /proc/loadavg, /proc/meminfo, /proc/uptime, /proc/cpuinfo.
// На macOS/Windows вернёт нули — это нормально, фронт это переживёт.
func readLinuxSystem() systemInfo {
	var s systemInfo

	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 3 {
			s.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
			s.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
			s.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
		}
	}

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 1 {
			if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
				s.UptimeSec = int64(v)
			}
		}
	}

	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			val, _ := strconv.ParseUint(parts[1], 10, 64)
			switch parts[0] {
			case "MemTotal:":
				s.MemTotalKB = val
			case "MemFree:":
				s.MemFreeKB = val
			case "MemAvailable:":
				s.MemAvailKB = val
			case "SwapTotal:":
				s.SwapTotalKB = val
			case "SwapFree:":
				s.SwapFreeKB = val
			}
		}
	}

	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "model name") && s.CPUModel == "" {
				if i := strings.Index(line, ":"); i >= 0 {
					s.CPUModel = strings.TrimSpace(line[i+1:])
				}
			}
			if strings.HasPrefix(line, "cpu MHz") && s.CPUMHz == 0 {
				if i := strings.Index(line, ":"); i >= 0 {
					s.CPUMHz, _ = strconv.ParseFloat(strings.TrimSpace(line[i+1:]), 64)
				}
			}
		}
	}

	return s
}

func statfs(path string) (diskInfo, bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return diskInfo{}, false
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	free := st.Bavail * bsize
	used := uint64(0)
	if total > free {
		used = total - free
	}
	ratio := 0.0
	if total > 0 {
		ratio = float64(used) / float64(total)
	}
	return diskInfo{
		TotalBytes: total,
		FreeBytes:  free,
		UsedBytes:  used,
		UsedRatio:  ratio,
	}, true
}

func (h *Handler) readDatabase(ctx context.Context) dbInfo {
	d := dbInfo{Name: h.dbName}

	stat := h.pool.Stat()
	d.MaxConnections = stat.MaxConns()
	d.ConnectionsTotal = int(stat.TotalConns())
	d.ConnectionsIdle = int(stat.IdleConns())
	d.ConnectionsBusy = int(stat.AcquiredConns())

	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_ = h.pool.QueryRow(c,
		`SELECT pg_database_size(current_database())`,
	).Scan(&d.SizeBytes)

	if d.Name == "" {
		_ = h.pool.QueryRow(c, `SELECT current_database()`).Scan(&d.Name)
	}

	var ver string
	if err := h.pool.QueryRow(c, `SHOW server_version`).Scan(&ver); err == nil {
		d.Version = ver
	}

	return d
}

func (h *Handler) readStats(ctx context.Context) statsInfo {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var s statsInfo

	rows := []struct {
		dst   *int64
		query string
	}{
		{&s.Users, `SELECT COUNT(*) FROM users`},
		{&s.Tracks, `SELECT COUNT(*) FROM tracks WHERE is_published=TRUE`},
		{&s.Playlists, `SELECT COUNT(*) FROM playlists`},
		{&s.Likes, `SELECT COUNT(*) FROM likes`},
		{&s.Comments, `SELECT COUNT(*) FROM comments`},
		{&s.Follows, `SELECT COUNT(*) FROM follows`},
		{&s.Sessions, `SELECT COUNT(*) FROM sessions WHERE expires_at > now()`},
	}
	for _, r := range rows {
		if err := h.pool.QueryRow(c, r.query).Scan(r.dst); err != nil {
			log.Printf("admin stats query (%s): %v", r.query, err)
		}
	}
	return s
}
