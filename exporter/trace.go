// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"log/slog"
)

const (
	// cleanup on add if greater than this threshold
	cleanupThreshold  uint64 = 1_000
	templateDirName   string = "templates"
	proctraceTemplate string = `
<html lang="en">
    <body>
        <table>
            <tr>
                <th> Job Id </th>
                <th> Process Id </th>
                <th> Cpu % </th>
                <th> I/O Wait  </th>
                <th> Memory Usage </th>
                <th> Username </th>
                <th> Hostname </th>
            </tr>
            {{ range . }}
                <tr>
                    <td> {{ .JobId  }} </td>
                    <td> {{ .Pid}} </td>
                    <td> {{ .Cpus }} </td>
                    <td> {{ .WriteBytes }}  </td>
                    <td> {{ .ReadBytes }}  </td>
                    <td> {{ .Mem }} </td>
                    <td> {{ .Username }} </td>
                    <td> {{ .Hostname }} </td>
                </tr>
            {{ end }}
        </table>
    </body>
</html>
`
)

// store a jobs published proc stats
type TraceInfo struct {
	JobId      int64   `json:"job_id"`
	Pid        int64   `json:"pid"`
	Cpus       float64 `json:"cpus"`
	WriteBytes float64 `json:"write_bytes"`
	ReadBytes  float64 `json:"read_bytes"`
	Threads    float64 `json:"threads"`
	Mem        float64 `json:"mem"`
	Username   string  `json:"username"`
	Hostname   string  `json:"hostname"`
	// do not set explicitly, overridden on Add
	uploadAt time.Time
}

type AtomicProcFetcher struct {
	sync.Mutex
	Info             map[int64]*TraceInfo
	sampleRate       uint64
	cleanupThreshold uint64
}

func NewAtomicProFetcher(sampleRate uint64) *AtomicProcFetcher {
	return &AtomicProcFetcher{
		Info:             make(map[int64]*TraceInfo),
		sampleRate:       sampleRate,
		cleanupThreshold: cleanupThreshold,
	}
}

// clean stale entries
func (m *AtomicProcFetcher) cleanup() {
	for jobid, metric := range m.Info {
		if time.Since(metric.uploadAt).Seconds() > float64(m.sampleRate) {
			delete(m.Info, jobid)
		}
	}
}

func (m *AtomicProcFetcher) Add(trace *TraceInfo) error {
	m.Lock()
	defer m.Unlock()
	if trace.JobId == 0 {
		return errors.New("job id unset")
	}
	trace.uploadAt = time.Now()
	m.Info[trace.JobId] = trace
	if len(m.Info) > int(m.cleanupThreshold) {
		m.cleanup()
	}
	return nil
}

func (m *AtomicProcFetcher) Fetch() map[int64]*TraceInfo {
	m.Lock()
	defer m.Unlock()
	m.cleanup()
	cpy := make(map[int64]*TraceInfo)
	for k, v := range m.Info {
		cpy[k] = v
	}
	return cpy
}

type TraceCollector struct {
	ProcessFetcher *AtomicProcFetcher
	squeueFetcher  SlurmMetricFetcher[JobMetric]
	fallback       bool
	// actual proc monitoring
	jobAllocMem  *prometheus.Desc
	jobAllocCpus *prometheus.Desc
	pid          *prometheus.Desc
	cpuUsage     *prometheus.Desc
	memUsage     *prometheus.Desc
	threadCount  *prometheus.Desc
	writeBytes   *prometheus.Desc
	readBytes    *prometheus.Desc
}

func NewTraceCollector(config *Config) *TraceCollector {
	traceConfig := config.TraceConf
	return &TraceCollector{
		ProcessFetcher: NewAtomicProFetcher(traceConfig.rate),
		squeueFetcher:  traceConfig.sharedFetcher,
		fallback:       config.cliOpts.fallback,
		// add for job id correlation
		jobAllocMem:  prometheus.NewDesc("slurm_job_mem_alloc", "running job mem allocated", []string{"jobid"}, nil),
		jobAllocCpus: prometheus.NewDesc("slurm_job_cpu_alloc", "running job cpus allocated", []string{"jobid"}, nil),
		pid:          prometheus.NewDesc("slurm_proc_pid", "pid of running slurm job", []string{"jobid", "hostname"}, nil),
		cpuUsage:     prometheus.NewDesc("slurm_proc_cpu_usage", "actual cpu usage collected from proc monitor", []string{"jobid", "username"}, nil),
		memUsage:     prometheus.NewDesc("slurm_proc_mem_usage", "proc mem usage", []string{"jobid", "username"}, nil),
		threadCount:  prometheus.NewDesc("slurm_proc_threadcount", "threads currently being used", []string{"jobid", "username"}, nil),
		writeBytes:   prometheus.NewDesc("slurm_proc_write_bytes", "proc write bytes", []string{"jobid", "username"}, nil),
		readBytes:    prometheus.NewDesc("slurm_proc_read_bytes", "proc read bytes", []string{"jobid", "username"}, nil),
	}
}

func (c *TraceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.jobAllocMem
	ch <- c.jobAllocCpus
	ch <- c.pid
	ch <- c.cpuUsage
	ch <- c.memUsage
	ch <- c.threadCount
	ch <- c.writeBytes
	ch <- c.readBytes
}

func (c *TraceCollector) Collect(ch chan<- prometheus.Metric) {
	procs := c.ProcessFetcher.Fetch()
	jobMetrics, err := c.squeueFetcher.FetchMetrics()
	if err != nil {
		return
	}
	for _, j := range jobMetrics {
		p, ok := procs[int64(j.JobId)]
		if !ok {
			continue
		}
		jobid := fmt.Sprint(p.JobId)
		ch <- prometheus.MustNewConstMetric(c.jobAllocMem, prometheus.GaugeValue, totalAllocMem(&j.JobResources), jobid)
		ch <- prometheus.MustNewConstMetric(c.jobAllocCpus, prometheus.GaugeValue, j.JobResources.AllocCpus, jobid)
		ch <- prometheus.MustNewConstMetric(c.pid, prometheus.GaugeValue, float64(p.Pid), jobid, p.Hostname)
		ch <- prometheus.MustNewConstMetric(c.cpuUsage, prometheus.GaugeValue, p.Cpus, jobid, p.Username)
		ch <- prometheus.MustNewConstMetric(c.memUsage, prometheus.GaugeValue, p.Mem, jobid, p.Username)
		ch <- prometheus.MustNewConstMetric(c.threadCount, prometheus.GaugeValue, p.Threads, jobid, p.Username)
		ch <- prometheus.MustNewConstMetric(c.writeBytes, prometheus.GaugeValue, p.WriteBytes, jobid, p.Username)
		ch <- prometheus.MustNewConstMetric(c.readBytes, prometheus.GaugeValue, p.ReadBytes, jobid, p.Username)
	}
}

func (c *TraceCollector) uploadTrace(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		defer r.Body.Close()
		var info TraceInfo
		if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
			slog.Error(fmt.Sprintf("unable to decode trace response due to err: %q", err))
			return
		}
		if err := c.ProcessFetcher.Add(&info); err != nil {
			slog.Error(fmt.Sprintf("failed to add to map with: %q", err))
			return
		}
	}
	if r.Method == http.MethodGet {

		tmpl := template.Must(template.New("proc_traces").Parse(proctraceTemplate))
		procs := c.ProcessFetcher.Fetch()
		traces := make([]TraceInfo, 0, len(procs))
		for _, info := range procs {
			traces = append(traces, *info)
		}
		if err := tmpl.Execute(w, traces); err != nil {
			slog.Error(fmt.Sprintf("template failed to render with err: %q", err))
			return
		}
	}
}

// detectTraceTemplatePath returns the trace_root path based on the following criteria:
// 1. If TRACE_ROOT_PATH is specified, search that directory. If we don't find a templates dir, let's panic and crash the program.
// 2. If TRACE_ROOT_PATH isn't specified, we can search cwd and /usr/share/prometheus-slurm-exporter.
// If no templates path is found, returns an empty string
func detectTraceTemplatePath() string {
	if rpath, ok := os.LookupEnv("TRACE_ROOT_PATH"); ok {
		templateP := filepath.Join(rpath, templateDirName)
		if _, err := os.Stat(templateP); err != nil {
			panic("TRACE_ROOT_PATH must include a directory called: templates")
		}
		return templateP
	}
	for _, rpath := range []string{".", "/usr/share/prometheus-slurm-exporter"} {
		templateP := filepath.Join(rpath, templateDirName)
		if _, err := os.Stat(templateP); err == nil {
			return templateP
		}
	}
	return ""
}
