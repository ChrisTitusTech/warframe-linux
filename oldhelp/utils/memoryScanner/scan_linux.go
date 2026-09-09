package memoryScanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func findProcess(name string) (int, error) {
	entries, err := filepath.Glob("/proc/[0-9]*/comm")
	if err != nil {
		return 0, err
	}
	for _, commPath := range entries {
		data, err := os.ReadFile(commPath)
		if err != nil {
			continue
		}
		// comm includes a trailing newline.
		if strings.TrimRight(string(data), "\n") == name {
			pidStr := strings.Split(commPath, "/")[2]
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}
			return pid, nil
		}
	}
	return 0, fmt.Errorf("process %q not found", name)
}

type memRegion struct {
	start, end uintptr
}

func readMaps(pid int) ([]memRegion, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	skipNames := map[string]bool{
		"[vdso]":    true,
		"[vvar]":    true,
		"[vsyscall]": true,
	}

	var regions []memRegion
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		perms := fields[1]
		if len(perms) < 1 || perms[0] != 'r' {
			continue
		}

		if len(fields) >= 6 {
			regionName := fields[5]
			if skipNames[regionName] {
				continue
			}
			// Skip device-mapped files (major device != 0).
			// fields[3] is "major:minor", fields[4] is inode.
			// A non-zero inode with a path that looks like a device is unusual,
			// but the simplest heuristic: skip anything under /dev.
			if strings.HasPrefix(regionName, "/dev/") {
				continue
			}
		}

		parts := strings.SplitN(fields[0], "-", 2)
		if len(parts) != 2 {
			continue
		}
		start, err1 := strconv.ParseUint(parts[0], 16, 64)
		end, err2 := strconv.ParseUint(parts[1], 16, 64)
		if err1 != nil || err2 != nil {
			continue
		}

		regions = append(regions, memRegion{
			start: uintptr(start),
			end:   uintptr(end),
		})
	}
	return regions, scanner.Err()
}

func scanAuthz(pid int) (string, error) {
	regions, err := readMaps(pid)
	if err != nil {
		return "", fmt.Errorf("failed to read process maps: %w", err)
	}

	memFile, err := os.Open(fmt.Sprintf("/proc/%d/mem", pid))
	if err != nil {
		return "", fmt.Errorf("failed to open process mem: %w", err)
	}
	defer memFile.Close()

	candidates := make(map[string]int)

	for _, region := range regions {
		if region.end <= region.start {
			continue
		}

		regionCandidates, err := scanRegionChunked(region.end-region.start, func(offset uintptr, dst []byte) (int, error) {
			return memFile.ReadAt(dst, int64(region.start+offset))
		})
		if err != nil {
			continue
		}

		mergeCandidates(candidates, regionCandidates)
		if authz := findConfident(candidates); authz != "" {
			return authz, nil
		}
	}

	return "", fmt.Errorf("authz not found in process memory")
}
