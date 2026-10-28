package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

const partialHashBytes = 64 * 1024

type hashedFile struct {
	file *FileRecord
	hash string
}

type hashJob struct {
	index int
	file  *FileRecord
}

type hashResult struct {
	index int
	hash  string
	ok    bool
}

// findDuplicates stages candidates by size, partial content and complete
// SHA-256. It never opens placeholders and never treats hard links as copies.
func (s *Scanner) findDuplicates(ctx context.Context, files []*FileRecord, request Request, progress func(done, total int)) []DuplicateGroup {
	bySize := make(map[uint64][]*FileRecord)
	candidates := 0
	for _, file := range files {
		if file.Placeholder || file.Reparse || file.LogicalBytes < request.MinDuplicateSize || file.LogicalBytes > request.MaxHashBytes {
			continue
		}
		if candidates >= request.MaxDuplicateFiles {
			break
		}
		bySize[file.LogicalBytes] = append(bySize[file.LogicalBytes], file)
		candidates++
	}

	sizeGroups := make([][]*FileRecord, 0)
	for _, group := range bySize {
		if len(group) > 1 {
			sort.Slice(group, func(i, j int) bool { return group[i].RelativePath < group[j].RelativePath })
			sizeGroups = append(sizeGroups, group)
		}
	}
	if len(sizeGroups) == 0 {
		return nil
	}

	partial := make(map[string][]*FileRecord)
	for _, group := range sizeGroups {
		results := hashFiles(ctx, group, partialHashBytes, request.Workers, progress)
		for _, result := range results {
			key := fmt.Sprintf("%d:%s", group[0].LogicalBytes, result.hash)
			partial[key] = append(partial[key], group[result.index])
		}
	}

	fullGroups := make([][]*FileRecord, 0)
	for _, group := range partial {
		if len(group) > 1 {
			fullGroups = append(fullGroups, group)
		}
	}
	if len(fullGroups) == 0 {
		return nil
	}

	verified := make(map[string][]*FileRecord)
	for _, group := range fullGroups {
		results := hashFiles(ctx, group, 0, request.Workers, progress)
		for _, result := range results {
			if !fileStillMatches(group[result.index]) {
				continue
			}
			verified[result.hash] = append(verified[result.hash], group[result.index])
		}
	}

	groups := make([]DuplicateGroup, 0)
	index := 0
	for hash, group := range verified {
		independent := make(map[string]bool)
		members := make([]DuplicateFile, 0)
		linked := false
		for _, file := range group {
			if file.LinkCount > 1 {
				linked = true
			}
			identity := file.FileID
			if identity == "" {
				identity = file.Path
			}
			if independent[identity] {
				continue
			}
			independent[identity] = true
			members = append(members, DuplicateFile{
				NodeID:       file.NodeID,
				RelativePath: file.RelativePath,
				Name:         file.Name,
				LinkCount:    file.LinkCount,
				FileID:       file.FileID,
			})
		}
		if len(members) < 2 {
			continue
		}
		sort.Slice(members, func(i, j int) bool { return members[i].RelativePath < members[j].RelativePath })
		index++
		note := "Different file IDs: independently allocated duplicate candidates."
		if linked {
			note = "Hard-linked names are present; verify link semantics before acting."
		}
		groups = append(groups, DuplicateGroup{
			ID:    fmt.Sprintf("dup-%d", index),
			Size:  group[0].LogicalBytes,
			Hash:  hash,
			Files: members,
			Note:  note,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Size != groups[j].Size {
			return groups[i].Size > groups[j].Size
		}
		return groups[i].Hash < groups[j].Hash
	})
	return groups
}

func hashFiles(ctx context.Context, files []*FileRecord, limit uint64, workers int, progress func(done, total int)) []hashResult {
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan hashJob)
	results := make(chan hashResult, len(files))
	var wait sync.WaitGroup
	wait.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			defer wait.Done()
			for job := range jobs {
				hash, ok := hashFile(ctx, job.file.Path, limit)
				results <- hashResult{index: job.index, hash: hash, ok: ok}
			}
		}()
	}
	go func() {
		for index, file := range files {
			if ctx.Err() != nil {
				break
			}
			jobs <- hashJob{index: index, file: file}
		}
		close(jobs)
		wait.Wait()
		close(results)
	}()
	output := make([]hashResult, 0, len(files))
	done := 0
	for result := range results {
		done++
		if progress != nil {
			progress(done, len(files))
		}
		if result.ok {
			output = append(output, result)
		}
	}
	return output
}

func hashFile(ctx context.Context, path string, limit uint64) (string, bool) {
	handle, err := os.Open(platformPath(path))
	if err != nil {
		return "", false
	}
	defer handle.Close()
	reader := io.Reader(handle)
	if limit > 0 {
		reader = io.LimitReader(handle, int64(limit))
	}
	hasher := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if ctx.Err() != nil {
			return "", false
		}
		read, readErr := reader.Read(buffer)
		if read > 0 {
			_, _ = hasher.Write(buffer[:read])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", false
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), true
}

func fileStillMatches(file *FileRecord) bool {
	info, err := os.Lstat(platformPath(file.Path))
	if err != nil || info.IsDir() || info.Size() < 0 {
		return false
	}
	if uint64(info.Size()) != file.LogicalBytes || !info.ModTime().Equal(file.ModTime) {
		return false
	}
	meta := inspectFile(file.Path, info)
	if meta.Placeholder || meta.Reparse {
		return false
	}
	if file.FileID != "" && meta.FileID != "" && file.FileID != meta.FileID {
		return false
	}
	return true
}
