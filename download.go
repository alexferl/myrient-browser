package myrient_browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func getFileInfo(fileURL string) (size int64, resumable bool, err error) {
	req, err := http.NewRequest("HEAD", fileURL, nil)
	if err != nil {
		return 0, false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, false, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	size = resp.ContentLength
	resumable = resp.Header.Get("Accept-Ranges") == "bytes"
	return size, resumable, nil
}

func scanAndDownload(basePath string, files []fileEntry, stats *downloadStats, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		decodedBasePath, err := url.QueryUnescape(basePath)
		if err != nil {
			decodedBasePath = basePath
		}

		outputDir := filepath.Join("./downloads", decodedBasePath)
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return errMsg{err: fmt.Errorf("failed to create directory: %w", err)}
		}

		var fileInfos []fileInfo
		var totalBytes int64
		var mu sync.Mutex

		jobs := make(chan fileEntry, len(files))
		results := make(chan fileInfo, len(files))
		var wg sync.WaitGroup

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for file := range jobs {
					select {
					case <-ctx.Done():
						return
					default:
					}

					decodedFilename, err := url.QueryUnescape(file.Path)
					if err != nil {
						decodedFilename = file.Path
					}

					fileURL := baseURL + basePath + file.Path
					outputPath := filepath.Join(outputDir, decodedFilename)
					size, resumable, err := getFileInfo(fileURL)

					existingSize := int64(0)
					if stat, err := os.Stat(outputPath); err == nil {
						existingSize = stat.Size()
					} else if stat, err := os.Stat(outputPath + ".part"); err == nil {
						existingSize = stat.Size()
					}

					info := fileInfo{
						url:       fileURL,
						filename:  decodedFilename,
						path:      outputPath,
						size:      size,
						resumable: resumable,
					}

					if err == nil && size > 0 {
						mu.Lock()
						totalBytes += size - existingSize
						mu.Unlock()
					}

					results <- info
					atomic.AddInt32(&stats.scanProgress, 1)
				}
			}()
		}

		for _, file := range files {
			jobs <- file
		}
		close(jobs)

		go func() {
			wg.Wait()
			close(results)
		}()

		for info := range results {
			fileInfos = append(fileInfos, info)
		}

		return scanCompleteMsg{
			totalBytes: totalBytes,
			files:      fileInfos,
		}
	}
}

func startDownloadWithFiles(files []fileInfo, stats *downloadStats, ctx context.Context, autoExtract bool, extractToFolder bool, deleteZip bool) tea.Cmd {
	return func() tea.Msg {
		jobs := make(chan fileInfo, len(files))
		var wg sync.WaitGroup
		var extractFiles []string
		var extractMu sync.Mutex

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					select {
					case <-ctx.Done():
						return
					default:
					}

					for atomic.LoadInt32(&stats.paused) == 1 {
						time.Sleep(100 * time.Millisecond)
						select {
						case <-ctx.Done():
							return
						default:
						}
					}

					downloadFileWithResume(ctx, job, stats)
					atomic.AddInt32(&stats.completed, 1)

					if autoExtract && strings.HasSuffix(strings.ToLower(job.path), ".zip") {
						extractMu.Lock()
						extractFiles = append(extractFiles, job.path)
						extractMu.Unlock()
					}
				}
			}()
		}

		for _, file := range files {
			jobs <- file
		}
		close(jobs)
		wg.Wait()

		if autoExtract && len(extractFiles) > 0 {
			for _, zipPath := range extractFiles {
				select {
				case <-ctx.Done():
					return downloadCompleteMsg{}
				default:
				}

				var extractDir string
				if extractToFolder {
					extractDir = strings.TrimSuffix(zipPath, filepath.Ext(zipPath))
				} else {
					extractDir = filepath.Dir(zipPath)
				}

				if err := unzipFile(zipPath, extractDir, deleteZip); err != nil {
					return errMsg{err: fmt.Errorf("failed to extract %s: %w", filepath.Base(zipPath), err)}
				}
				atomic.AddInt32(&stats.extracted, 1)
			}
		}

		return downloadCompleteMsg{}
	}
}

func downloadAllFiles(basePath string, files []fileEntry, stats *downloadStats, ctx context.Context, autoExtract bool, extractToFolder bool, deleteZip bool) tea.Cmd {
	return func() tea.Msg {
		decodedBasePath, err := url.QueryUnescape(basePath)
		if err != nil {
			decodedBasePath = basePath
		}

		outputDir := filepath.Join("./downloads", decodedBasePath)
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			return errMsg{err: fmt.Errorf("failed to create directory: %w", err)}
		}

		var fileInfos []fileInfo
		for _, file := range files {
			decodedFilename, err := url.QueryUnescape(file.Path)
			if err != nil {
				decodedFilename = file.Path
			}

			fileInfos = append(fileInfos, fileInfo{
				url:       baseURL + basePath + file.Path,
				filename:  decodedFilename,
				path:      filepath.Join(outputDir, decodedFilename),
				size:      0,
				resumable: true,
			})
		}

		jobs := make(chan fileInfo, len(fileInfos))
		var wg sync.WaitGroup
		var extractFiles []string
		var extractMu sync.Mutex

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					select {
					case <-ctx.Done():
						return
					default:
					}

					for atomic.LoadInt32(&stats.paused) == 1 {
						time.Sleep(100 * time.Millisecond)
						select {
						case <-ctx.Done():
							return
						default:
						}
					}

					downloadFileWithResume(ctx, job, stats)
					atomic.AddInt32(&stats.completed, 1)

					if autoExtract && strings.HasSuffix(strings.ToLower(job.path), ".zip") {
						extractMu.Lock()
						extractFiles = append(extractFiles, job.path)
						extractMu.Unlock()
					}
				}
			}()
		}

		for _, file := range fileInfos {
			jobs <- file
		}
		close(jobs)
		wg.Wait()

		if autoExtract && len(extractFiles) > 0 {
			for _, zipPath := range extractFiles {
				select {
				case <-ctx.Done():
					return downloadCompleteMsg{}
				default:
				}

				var extractDir string
				if extractToFolder {
					extractDir = strings.TrimSuffix(zipPath, filepath.Ext(zipPath))
				} else {
					extractDir = filepath.Dir(zipPath)
				}

				if err := unzipFile(zipPath, extractDir, deleteZip); err != nil {
					return errMsg{err: fmt.Errorf("failed to extract %s: %w", filepath.Base(zipPath), err)}
				}
				atomic.AddInt32(&stats.extracted, 1)
			}
		}

		return downloadCompleteMsg{}
	}
}

func downloadFileWithResume(ctx context.Context, file fileInfo, stats *downloadStats) int64 {
	partFile := file.path + ".part"
	existingSize := int64(0)

	if stat, err := os.Stat(file.path); err == nil {
		if file.size > 0 && stat.Size() == file.size {
			return 0
		}
	}

	if stat, err := os.Stat(partFile); err == nil {
		existingSize = stat.Size()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", file.url, nil)
	if err != nil {
		return 0
	}

	if existingSize > 0 && file.resumable {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer func() { _ = resp.Body.Close() }()

	flag := os.O_CREATE | os.O_WRONLY
	if existingSize > 0 && (resp.StatusCode == 206 || resp.StatusCode == 200) {
		flag |= os.O_APPEND
	}

	out, err := os.OpenFile(partFile, flag, 0o644)
	if err != nil {
		return 0
	}
	defer func() { _ = out.Close() }()

	reader := &progressReader{
		reader: resp.Body,
		stats:  stats,
	}

	n, _ := io.Copy(out, reader)

	select {
	case <-ctx.Done():
		return n
	default:
	}

	_ = os.Rename(partFile, file.path)
	return n
}

type progressReader struct {
	reader io.Reader
	stats  *downloadStats
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	atomic.AddInt64(&pr.stats.bytesDownload, int64(n))
	return n, err
}
