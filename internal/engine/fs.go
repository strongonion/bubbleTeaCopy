package engine

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func clearTarget(path string) error {
	info, exists, err := statMaybe(path)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	if !info.IsDir() {
		return os.RemoveAll(path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if err := os.RemoveAll(child); err != nil {
			return err
		}
	}
	return nil
}

func copyPath(src, dst string, srcIsDir bool) error {
	if srcIsDir {
		return copyDirMerge(src, dst)
	}
	return copyFileReplace(src, dst)
}

func movePath(src, dst string, srcIsDir bool) error {
	if srcIsDir {
		return moveDir(src, dst)
	}
	return moveFile(src, dst)
}

func moveDir(src, dst string) error {
	dstInfo, dstExists, err := statMaybe(dst)
	if err != nil {
		return err
	}

	if dstExists && dstInfo.IsDir() {
		if err := copyDirMerge(src, dst); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}

	if dstExists && !dstInfo.IsDir() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}

	if err := ensureParent(dst); err != nil {
		return err
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyDirMerge(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func moveFile(src, dst string) error {
	dstInfo, dstExists, err := statMaybe(dst)
	if err != nil {
		return err
	}

	if dstExists && dstInfo.IsDir() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	} else if dstExists {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}

	if err := ensureParent(dst); err != nil {
		return err
	}

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyFileReplace(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}
	return nil
}

func copyDirMerge(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not directory: %s", src)
	}

	dstInfo, dstExists, err := statMaybe(dst)
	if err != nil {
		return err
	}
	if dstExists && !dstInfo.IsDir() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		dstExists = false
	}
	if !dstExists {
		if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
			return err
		}
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		destPath := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			existing, exists, err := statMaybe(destPath)
			if err != nil {
				return err
			}
			if exists && !existing.IsDir() {
				if err := os.RemoveAll(destPath); err != nil {
					return err
				}
			}
			return os.MkdirAll(destPath, info.Mode().Perm())
		}

		return copyFileReplace(path, destPath)
	})
}

func copyFileReplace(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is directory: %s", src)
	}

	dstInfo, dstExists, err := statMaybe(dst)
	if err != nil {
		return err
	}
	if dstExists && dstInfo.IsDir() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	} else if dstExists {
		if err := os.Remove(dst); err != nil {
			return err
		}
	}

	if err := ensureParent(dst); err != nil {
		return err
	}

	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}

	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(dst, srcInfo.Mode().Perm())
}

func ensureParent(path string) error {
	parent := filepath.Dir(path)
	return os.MkdirAll(parent, 0o755)
}

func statMaybe(path string) (os.FileInfo, bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, err
}
