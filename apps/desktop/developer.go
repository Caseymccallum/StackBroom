package main

type DockerView struct {
	Available       bool             `json:"available"`
	Reason          string           `json:"reason,omitempty"`
	ServerVersion   string           `json:"serverVersion,omitempty"`
	Context         string           `json:"context,omitempty"`
	Endpoint        string           `json:"endpoint,omitempty"`
	EndpointLocal   bool             `json:"endpointLocal"`
	RemoteApproved  bool             `json:"remoteApproved"`
	Running         int              `json:"running"`
	Stopped         int              `json:"stopped"`
	Containers      []ContainerRow   `json:"containers"`
	DiskUsage       []DockerDiskRow  `json:"diskUsage"`
	Images          []DockerImageRow `json:"images"`
	ImagesTruncated bool             `json:"imagesTruncated"`
	Warnings        []string         `json:"warnings"`
}

type ContainerRow struct {
	Name           string `json:"name"`
	Image          string `json:"image"`
	State          string `json:"state"`
	Status         string `json:"status"`
	ComposeProject string `json:"composeProject,omitempty"`
	ComposeService string `json:"composeService,omitempty"`
	StatsKnown     bool   `json:"statsKnown"`
	CPUPercent     string `json:"cpuPercent,omitempty"`
	MemoryUsage    string `json:"memoryUsage,omitempty"`
	StoppedForText string `json:"stoppedForText,omitempty"`
}

type DockerDiskRow struct {
	Type             string  `json:"type"`
	Total            int     `json:"total"`
	Active           int     `json:"active"`
	SizeText         string  `json:"sizeText,omitempty"`
	SizeBytes        *uint64 `json:"sizeBytes,omitempty"`
	ReclaimableText  string  `json:"reclaimableText,omitempty"`
	ReclaimableBytes *uint64 `json:"reclaimableBytes,omitempty"`
}

type DockerImageRow struct {
	Repository string  `json:"repository"`
	Tag        string  `json:"tag"`
	SizeText   string  `json:"sizeText,omitempty"`
	SizeBytes  *uint64 `json:"sizeBytes,omitempty"`
	InUse      bool    `json:"inUse"`
}

type WSLView struct {
	Available     bool              `json:"available"`
	Reason        string            `json:"reason,omitempty"`
	Distributions []WSLDistribution `json:"distributions"`
	VMMemoryBytes uint64            `json:"vmMemoryBytes"`
	VMMemoryKnown bool              `json:"vmMemoryKnown"`
	VMProcessName string            `json:"vmProcessName,omitempty"`
	VMNote        string            `json:"vmNote,omitempty"`
	Warnings      []string          `json:"warnings"`
}

type WSLDistribution struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	Version   string `json:"version"`
	IsDefault bool   `json:"isDefault"`
}

type ListenerRow struct {
	Protocol    string `json:"protocol"`
	Family      string `json:"family"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	PID         int32  `json:"pid"`
	ProcessName string `json:"processName,omitempty"`
	ProjectName string `json:"projectName,omitempty"`
	Wildcard    bool   `json:"wildcard"`
	Ownership   string `json:"ownership"`
}

type DuplicateRow struct {
	Kind       string        `json:"kind"`
	Key        string        `json:"key"`
	Runtime    string        `json:"runtime"`
	Port       int           `json:"port"`
	PIDs       []int32       `json:"pids"`
	Confidence string        `json:"confidence"`
	Evidence   []string      `json:"evidence"`
	Listeners  []ListenerRow `json:"listeners"`
}

type Opportunity struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Title           string   `json:"title"`
	Detected        string   `json:"detected"`
	Why             string   `json:"why"`
	Evidence        []string `json:"evidence"`
	Resource        string   `json:"resource"`
	ResourceBytes   uint64   `json:"resourceBytes,omitempty"`
	ResourceUnknown bool     `json:"resourceUnknown"`
	Review          []string `json:"review"`
	Risk            string   `json:"risk"`
	RiskDetail      string   `json:"riskDetail"`
	Confidence      string   `json:"confidence"`
}
