// SPDX-FileCopyrightText: 2023 Rivos Inc.
//
// SPDX-License-Identifier: Apache-2.0

package main

import "encoding/json"

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

func parseSlurmctldMetrics(sdiagResp []byte) (*SdiagResponse, error) {
	sdiag := new(SdiagResponse)
	err := json.Unmarshal(sdiagResp, sdiag)
	return sdiag, err
}
