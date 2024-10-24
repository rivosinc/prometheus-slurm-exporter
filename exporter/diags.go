// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package exporter

import (
	"encoding/json"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slog"
)

type UserRpcInfo struct {
	User      string `json:"user"`
	UserId    int    `json:"user_id"`
	Count     int    `json:"count"`
	AvgTime   int    `json:"average_time"`
	TotalTime int    `json:"total_time"`
}

type MessageRpcInfo struct {
	MessageType string `json:"message_type"`
	TypeId      int    `json:"type_id"`
	Count       int    `json:"count"`
	AvgTime     int    `json:"average_time"`
	TotalTime   int    `json:"total_time"`
}

type DiagMetric struct {
	ServerThreadCount     int              `json:"server_thread_count"`
	DBDAgentQueueSize     int              `json:"dbd_agent_queue_size"`
	RpcByUser             []UserRpcInfo    `json:"rpcs_by_user"`
	RpcByMessageType      []MessageRpcInfo `json:"rpcs_by_message_type"`
	BackfillJobCount      int              `json:"bf_backfilled_jobs"`
	BackfillCycleCountSum int              `json:"bf_cycle_sum"`
	BackfillCycleCounter  int              `json:"bf_cycle_counter"`
	BackfillLastDepth     int              `json:"bf_last_depth"`
	BackfillLastDepthTry  int              `json:"bf_last_depth_try"`
}

type SdiagResponse struct {
	Meta struct {
		SlurmVersion struct {
			Version struct {
				Major int `json:"major"`
				Micro int `json:"micro"`
				Minor int `json:"minor"`
			} `json:"version"`
			Release string `json:"release"`
		} `json:"Slurm"`
		Plugins map[string]string
	} `json:"meta"`
	Statistics DiagMetric
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
}

func parseDiagMetrics(sdiagResp []byte) (*SdiagResponse, error) {
	sdiag := new(SdiagResponse)
	err := json.Unmarshal(sdiagResp, sdiag)
	return sdiag, err
}

type DiagnosticsCollector struct {
	// collector state
	fetcher            SlurmByteScraper
	diagScrapeError    prometheus.Counter
	diagScrapeDuration *prometheus.Desc
	// user rpc metrics
	slurmUserRpcCount     *prometheus.Desc
	slurmUserRpcTotalTime *prometheus.Desc
	// type rpc metrics
	slurmTypeRpcCount     *prometheus.Desc
	slurmTypeRpcAvgTime   *prometheus.Desc
	slurmTypeRpcTotalTime *prometheus.Desc
	// daemon metrics
	slurmCtlThreadCount            *prometheus.Desc
	slurmDbdAgentQueueSize         *prometheus.Desc
	slurmBackfillJobCount          *prometheus.Desc
	slurmBackfillCycleCount        *prometheus.Desc
	slurmBackfillLastDepth         *prometheus.Desc
	slurmBackfillLastDepthTrySched *prometheus.Desc
	slurmBackfillCycleCounter      *prometheus.Desc
}

func NewDiagsCollector(config *Config) *DiagnosticsCollector {
	cliOpts := config.cliOpts
	return &DiagnosticsCollector{
		fetcher:                        NewCliScraper(config.cliOpts.sdiag...),
		slurmUserRpcCount:              prometheus.NewDesc("slurm_rpc_user_count", "slurm rpc count per user", []string{"user"}, nil),
		slurmUserRpcTotalTime:          prometheus.NewDesc("slurm_rpc_user_total_time", "slurm rpc avg time per user", []string{"user"}, nil),
		slurmTypeRpcCount:              prometheus.NewDesc("slurm_rpc_msg_type_count", "slurm rpc count per message type", []string{"type"}, nil),
		slurmTypeRpcAvgTime:            prometheus.NewDesc("slurm_rpc_msg_type_avg_time", "slurm rpc total time consumed per message type", []string{"type"}, nil),
		slurmTypeRpcTotalTime:          prometheus.NewDesc("slurm_rpc_msg_type_total_time", "slurm rpc avg time per message type", []string{"type"}, nil),
		slurmCtlThreadCount:            prometheus.NewDesc("slurm_daemon_thread_count", "slurm daemon thread count", nil, nil),
		slurmDbdAgentQueueSize:         prometheus.NewDesc("slurm_dbd_agent_queue_size", "slurmDbd queue size. Number of threads interacting with SlrumDBD. Will grow rapidly if DB is down or under stress", nil, nil),
		slurmBackfillJobCount:          prometheus.NewDesc("slurm_backfill_job_count", "slurm number of jobs started thanks to backfilling since last slurm start", nil, nil),
		slurmBackfillCycleCount:        prometheus.NewDesc("slurm_backfill_cycle_count", "slurm number of Number of backfill scheduling cycles since last reset", nil, nil),
		slurmBackfillLastDepth:         prometheus.NewDesc("slurm_backfill_last_depth", "slurm number of processed jobs during last backfilling scheduling cycle. It counts every job even if that job can not be started due to dependencies or limits", nil, nil),
		slurmBackfillLastDepthTrySched: prometheus.NewDesc("slurm_backfill_last_depth_try_sched", "slurm number of processed jobs during last backfilling scheduling cycle. It counts only jobs with a chance to start using available resources", nil, nil),
		slurmBackfillCycleCounter:      prometheus.NewDesc("slurm_backfill_cycle_counter", "slurm number of backfill scheduling cycles since last reset", nil, nil),
		diagScrapeError: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "slurm_diag_scrape_error",
			Help: "slurm diag scrape erro",
		}),
		diagScrapeDuration: prometheus.NewDesc("slurm_diag_scrape_duration", fmt.Sprintf("how long the cmd %v took (ms)", cliOpts.sdiag), nil, nil),
	}
}

func (sc *DiagnosticsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- sc.slurmUserRpcCount
	ch <- sc.slurmUserRpcTotalTime
	ch <- sc.slurmTypeRpcCount
	ch <- sc.slurmTypeRpcAvgTime
	ch <- sc.slurmTypeRpcTotalTime
	ch <- sc.slurmCtlThreadCount
	ch <- sc.diagScrapeDuration
	ch <- sc.slurmDbdAgentQueueSize
	ch <- sc.slurmBackfillJobCount
	ch <- sc.slurmBackfillCycleCount
	ch <- sc.slurmBackfillLastDepth
	ch <- sc.slurmBackfillLastDepthTrySched
	ch <- sc.slurmBackfillCycleCounter
	ch <- sc.diagScrapeError.Desc()
}

func (sc *DiagnosticsCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- sc.diagScrapeError
	}()
	sdiag, err := sc.fetcher.FetchRawBytes()
	if err != nil {
		sc.diagScrapeError.Inc()
		slog.Error(fmt.Sprintf("sdiag fetch error %q", err))
		return
	}
	ch <- prometheus.MustNewConstMetric(sc.diagScrapeDuration, prometheus.GaugeValue, float64(sc.fetcher.Duration().Abs().Milliseconds()))
	sdiagResponse, err := parseDiagMetrics(sdiag)
	if _, ok := sdiagResponse.Meta.Plugins["data_parser"]; !ok {
		sc.diagScrapeError.Inc()
		slog.Error("only the data_parser plugin is supported")
		return
	}
	if err != nil {
		sc.diagScrapeError.Inc()
		slog.Error(fmt.Sprintf("diag parse error: %q", err))
		return
	}
	emitNonZero := func(desc *prometheus.Desc, val float64, label string) {
		if val > 0 {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, label)
		}
	}
	ch <- prometheus.MustNewConstMetric(sc.slurmCtlThreadCount, prometheus.GaugeValue, float64(sdiagResponse.Statistics.ServerThreadCount))
	ch <- prometheus.MustNewConstMetric(sc.slurmDbdAgentQueueSize, prometheus.GaugeValue, float64(sdiagResponse.Statistics.DBDAgentQueueSize))
	ch <- prometheus.MustNewConstMetric(sc.slurmBackfillJobCount, prometheus.GaugeValue, float64(sdiagResponse.Statistics.BackfillJobCount))
	ch <- prometheus.MustNewConstMetric(sc.slurmBackfillCycleCount, prometheus.GaugeValue, float64(sdiagResponse.Statistics.BackfillCycleCountSum))
	ch <- prometheus.MustNewConstMetric(sc.slurmBackfillLastDepth, prometheus.GaugeValue, float64(sdiagResponse.Statistics.BackfillLastDepth))
	ch <- prometheus.MustNewConstMetric(sc.slurmBackfillLastDepthTrySched, prometheus.GaugeValue, float64(sdiagResponse.Statistics.BackfillLastDepthTry))
	ch <- prometheus.MustNewConstMetric(sc.slurmBackfillCycleCounter, prometheus.GaugeValue, float64(sdiagResponse.Statistics.BackfillCycleCounter))
	for _, userRpcInfo := range sdiagResponse.Statistics.RpcByUser {
		emitNonZero(sc.slurmUserRpcCount, float64(userRpcInfo.Count), userRpcInfo.User)
		emitNonZero(sc.slurmUserRpcTotalTime, float64(userRpcInfo.TotalTime), userRpcInfo.User)
	}
	for _, typeRpcInfo := range sdiagResponse.Statistics.RpcByMessageType {
		emitNonZero(sc.slurmTypeRpcAvgTime, float64(typeRpcInfo.AvgTime), typeRpcInfo.MessageType)
		emitNonZero(sc.slurmTypeRpcCount, float64(typeRpcInfo.Count), typeRpcInfo.MessageType)
		emitNonZero(sc.slurmTypeRpcTotalTime, float64(typeRpcInfo.TotalTime), typeRpcInfo.MessageType)
	}
}
