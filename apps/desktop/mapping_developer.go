package main

import "github.com/Caseymccallum/StackBroom/internal/model"

func mapDeveloper(snapshot model.Snapshot) Developer {
	return Developer{
		Docker:     mapDocker(snapshot.Docker),
		WSL:        mapWSL(snapshot.WSL),
		Listeners:  mapListeners(snapshot.Network.Listeners),
		Duplicates: mapDuplicates(snapshot.Duplicates),
	}
}

func mapDocker(info model.DockerInfo) DockerView {
	view := DockerView{
		Available:       info.Status.Available,
		Reason:          info.Status.Reason,
		ServerVersion:   info.Status.ServerVersion,
		Context:         info.Status.Context,
		Endpoint:        info.Status.Endpoint,
		EndpointLocal:   info.Status.EndpointLocal,
		RemoteApproved:  info.Status.RemoteApproved,
		Running:         info.RunningContainers(),
		Stopped:         info.StoppedContainers(),
		Containers:      make([]ContainerRow, 0, len(info.Containers)),
		DiskUsage:       make([]DockerDiskRow, 0, len(info.DiskUsage)),
		Images:          make([]DockerImageRow, 0, len(info.Images)),
		ImagesTruncated: info.ImagesTruncated,
		Warnings:        append([]string(nil), info.Warnings...),
	}
	for _, container := range info.Containers {
		view.Containers = append(view.Containers, ContainerRow{
			Name:           container.Name,
			Image:          container.Image,
			State:          container.State,
			Status:         container.Status,
			ComposeProject: container.ComposeProject,
			ComposeService: container.ComposeService,
			StatsKnown:     container.StatsKnown,
			CPUPercent:     container.CPUPercent,
			MemoryUsage:    container.MemoryUsage,
			StoppedForText: container.StoppedForText,
		})
	}
	for _, row := range info.DiskUsage {
		view.DiskUsage = append(view.DiskUsage, DockerDiskRow{
			Type:             row.Type,
			Total:            row.Total,
			Active:           row.Active,
			SizeText:         row.SizeText,
			SizeBytes:        cloneUint64(row.Size),
			ReclaimableText:  row.ReclaimableText,
			ReclaimableBytes: cloneUint64(row.Reclaimable),
		})
	}
	for _, image := range info.Images {
		view.Images = append(view.Images, DockerImageRow{
			Repository: image.Repository,
			Tag:        image.Tag,
			SizeText:   image.SizeText,
			SizeBytes:  cloneUint64(image.Size),
			InUse:      image.InUse,
		})
	}
	return view
}

func mapWSL(info model.WSLInfo) WSLView {
	view := WSLView{
		Available:     info.Available,
		Reason:        info.Reason,
		Distributions: make([]WSLDistribution, 0, len(info.Distributions)),
		VMMemoryBytes: info.VMMemoryBytes,
		VMMemoryKnown: info.VMMemoryKnown,
		VMProcessName: info.VMProcessName,
		VMNote:        info.VMNote,
		Warnings:      append([]string(nil), info.Warnings...),
	}
	for _, distro := range info.Distributions {
		view.Distributions = append(view.Distributions, WSLDistribution{
			Name:      distro.Name,
			State:     distro.State,
			Version:   distro.Version,
			IsDefault: distro.IsDefault,
		})
	}
	return view
}

func mapListeners(listeners []model.TCPListener) []ListenerRow {
	out := make([]ListenerRow, 0, len(listeners))
	for _, listener := range listeners {
		out = append(out, ListenerRow{
			Protocol:    listener.Protocol,
			Family:      listener.Family,
			Address:     listener.Address,
			Port:        listener.Port,
			PID:         listener.PID,
			ProcessName: listener.ProcessName,
			ProjectName: listener.ProjectName,
			Wildcard:    listener.Wildcard,
			Ownership:   string(listener.Ownership),
		})
	}
	return out
}

func mapDuplicates(groups []model.DuplicateGroup) []DuplicateRow {
	out := make([]DuplicateRow, 0, len(groups))
	for _, group := range groups {
		row := DuplicateRow{
			Kind:       group.Kind,
			Key:        group.Key,
			Runtime:    string(group.Runtime),
			Port:       group.Port,
			PIDs:       append([]int32(nil), group.PIDs...),
			Confidence: string(group.Confidence),
			Evidence:   append([]string(nil), group.Evidence...),
			Listeners:  make([]ListenerRow, 0, len(group.Listeners)),
		}
		for _, listener := range group.Listeners {
			row.Listeners = append(row.Listeners, ListenerRow{
				Protocol:  "tcp",
				Family:    listener.Family,
				Address:   listener.Address,
				Port:      listener.Port,
				PID:       listener.PID,
				Wildcard:  listener.Address == "0.0.0.0" || listener.Address == "::",
				Ownership: string(listener.Ownership),
			})
		}
		out = append(out, row)
	}
	return out
}

func mapOpportunities(recommendations []model.Recommendation) []Opportunity {
	out := make([]Opportunity, 0, len(recommendations))
	for _, rec := range recommendations {
		out = append(out, Opportunity{
			ID:              rec.ID,
			Kind:            rec.Kind,
			Title:           rec.Title,
			Detected:        rec.Detected,
			Why:             rec.WhyUnnecessary,
			Evidence:        append([]string(nil), rec.Evidence...),
			Resource:        rec.Resource,
			ResourceBytes:   rec.ResourceBytes,
			ResourceUnknown: rec.ResourceUnknown,
			Review:          append([]string(nil), rec.Review...),
			Risk:            string(rec.Risk),
			RiskDetail:      rec.RiskDetail,
			Confidence:      string(rec.Confidence),
		})
	}
	return out
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
