// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

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
	Statistics struct {
		ServerThreadCount int              `json:"server_thread_count"`
		RpcByUser         []UserRpcInfo    `json:"rpcs_by_user"`
		RpcByMessageType  []MessageRpcInfo `json:"rpcs_by_message_type"`
	}
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

func parseDiagMetrics(sdiagResp []byte) (*SdiagResponse, error) {
	sdiag := new(SdiagResponse)
	err := json.Unmarshal(sdiagResp, sdiag)
	return sdiag, err
}

type DiagnosticsCollector struct {
	// collector state
	fetcher            SlurmFetcher
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
	slurmCtlThreadCount *prometheus.Desc
}

func NewDiagsCollector(config *Config) *DiagnosticsCollector {
	cliOpts := config.cliOpts
	return &DiagnosticsCollector{
		fetcher:               NewCliFetcher(config.cliOpts.sdiag...),
		slurmUserRpcCount:     prometheus.NewDesc("slurm_rpc_user_count", "slurm rpc count per user", []string{"user"}, nil),
		slurmUserRpcTotalTime: prometheus.NewDesc("slurm_rpc_user_total_time", "slurm rpc avg time per user", []string{"user"}, nil),
		slurmTypeRpcCount:     prometheus.NewDesc("slurm_rpc_msg_type_count", "slurm rpc count per message type", []string{"type"}, nil),
		slurmTypeRpcAvgTime:   prometheus.NewDesc("slurm_rpc_msg_type_avg_time", "slurm rpc total time consumed per message type", []string{"type"}, nil),
		slurmTypeRpcTotalTime: prometheus.NewDesc("slurm_rpc_msg_type_total_time", "slurm rpc avg time per message type", []string{"type"}, nil),
		slurmCtlThreadCount:   prometheus.NewDesc("slurm_daemon_thread_count", "slurm daemon thread count", nil, nil),
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
	ch <- sc.diagScrapeError.Desc()
}

func (sc *DiagnosticsCollector) Collect(ch chan<- prometheus.Metric) {
	defer func() {
		ch <- sc.diagScrapeError
	}()
	sdiag, err := sc.fetcher.Fetch()
	if err != nil {
		sc.diagScrapeError.Inc()
		return
	}
	ch <- prometheus.MustNewConstMetric(sc.diagScrapeDuration, prometheus.GaugeValue, float64(sc.fetcher.Duration().Abs().Milliseconds()))
	sdiagResponse, err := parseDiagMetrics(sdiag)
	if err != nil {
		sc.diagScrapeError.Inc()
		return
	}
	if _, ok := sdiagResponse.Meta.Plugins["data_parser"]; !ok {
		slog.Error("only the data_parser plugin is supported")
		sc.diagScrapeError.Inc()
		return
	}
	emitNonZero := func(desc *prometheus.Desc, val float64, label string) {
		if val > 0 {
			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, val, label)
		}
	}
	ch <- prometheus.MustNewConstMetric(sc.slurmCtlThreadCount, prometheus.GaugeValue, float64(sdiagResponse.Statistics.ServerThreadCount))
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
