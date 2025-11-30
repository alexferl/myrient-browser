package myrient_browser

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func unzipFile(src, dest string, deleteAfter bool) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			_ = outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		_ = outFile.Close()
		_ = rc.Close()
		if err != nil {
			return err
		}
	}

	if deleteAfter {
		if err := os.Remove(src); err != nil {
			return err
		}
	}

	return nil
}
