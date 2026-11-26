package main

import (
	"fmt"
	"os"
	"syscall"
)

const maxInt64 = int64(^uint64(0) >> 1)

func availableBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("inspect free disk space at %s: %w", path, err)
	}
	if stat.Bsize <= 0 {
		return 0, fmt.Errorf("filesystem returned invalid block size %d", stat.Bsize)
	}
	blockSize := uint64(stat.Bsize)
	if stat.Bavail > ^uint64(0)/blockSize {
		return ^uint64(0), nil
	}
	return stat.Bavail * blockSize, nil
}

func formatBytes(n uint64) string {
	const (
		KiB = uint64(1 << 10)
		MiB = uint64(1 << 20)
		GiB = uint64(1 << 30)
	)
	switch {
	case n >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(n)/float64(GiB))
	case n >= MiB:
		return fmt.Sprintf("%.2f MiB", float64(n)/float64(MiB))
	case n >= KiB:
		return fmt.Sprintf("%.2f KiB", float64(n)/float64(KiB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func checkAvailableSpace(required, available uint64, operation string) error {
	if required > available {
		return fmt.Errorf("insufficient free disk space for %s: need %s, available %s",
			operation, formatBytes(required), formatBytes(available))
	}
	return nil
}

func requireFreeSpace(path string, required int64, operation string) error {
	if required < 0 {
		return fmt.Errorf("invalid negative space requirement for %s: %d", operation, required)
	}
	available, err := availableBytes(path)
	if err != nil {
		return err
	}
	if err := checkAvailableSpace(uint64(required), available, operation); err != nil {
		return err
	}
	logf("SPACE preflight (%s): need %s; available %s", operation, formatBytes(uint64(required)), formatBytes(available))
	return nil
}

func combinedSpaceRequirement(downloadBytes, reserveBytes int64) (int64, error) {
	if downloadBytes < 0 || reserveBytes < 0 {
		return 0, fmt.Errorf("negative disk-space requirement: download=%d reserve=%d", downloadBytes, reserveBytes)
	}
	if reserveBytes > maxInt64-downloadBytes {
		return 0, fmt.Errorf("disk-space requirement overflows: download=%d reserve=%d", downloadBytes, reserveBytes)
	}
	return downloadBytes + reserveBytes, nil
}

// preflightFiles hashes existing files once, then estimates the additional bytes
// needed for every missing or invalid resource. The returned map lets workers
// skip a second hash of files already proven valid.
func preflightFiles(root string, files []Entry) (map[string]bool, int64, error) {
	return preflightFilesWithReserve(root, files, 0, "download")
}

func preflightFilesWithReserve(root string, files []Entry, reserveBytes int64, operation string) (map[string]bool, int64, error) {
	if reserveBytes < 0 {
		return nil, 0, fmt.Errorf("invalid negative reserve for %s: %d", operation, reserveBytes)
	}
	if st, err := os.Lstat(root); err == nil && st.Mode()&os.ModeSymlink != 0 {
		return nil, 0, fmt.Errorf("symlink refused for download root: %s", root)
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, 0, err
	}

	valid := make(map[string]bool, len(files))
	var required int64
	for _, entry := range files {
		if entry.Size < 0 || entry.Size >= maxInt64 || !validMD5(entry.MD5) {
			return nil, 0, fmt.Errorf("invalid size or MD5 in download manifest: %s", entry.Name)
		}
		p, err := safe(root, entry.Name)
		if err != nil {
			return nil, 0, err
		}
		if st, statErr := os.Lstat(p); statErr == nil && !st.Mode().IsRegular() {
			return nil, 0, fmt.Errorf("download destination exists but is not a regular file: %s", entry.Name)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return nil, 0, fmt.Errorf("inspect download destination %s: %w", entry.Name, statErr)
		}
		part := p + ".part"
		if st, statErr := os.Lstat(part); statErr == nil && !st.Mode().IsRegular() {
			return nil, 0, fmt.Errorf("partial download path exists but is not a regular file: %s", entry.Name)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return nil, 0, fmt.Errorf("inspect partial download %s: %w", entry.Name, statErr)
		}
		if verify(p, entry) {
			valid[entry.Name] = true
			continue
		}
		// A complete, checksum-verified .part can be atomically promoted without
		// downloading another copy or requiring additional free space.
		if verify(part, entry) {
			continue
		}
		if entry.Size > maxInt64-required {
			return nil, 0, fmt.Errorf("download size estimate overflows for %s", entry.Name)
		}
		required += entry.Size
	}
	totalRequired, err := combinedSpaceRequirement(required, reserveBytes)
	if err != nil {
		return nil, 0, err
	}
	if err := requireFreeSpace(root, totalRequired, operation); err != nil {
		return nil, 0, err
	}
	return valid, required, nil
}
