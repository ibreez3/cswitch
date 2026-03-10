package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteFileAtomic 原子写入文件
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}

// BackupFile 创建备份文件
func BackupFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return CopyFile(path, bakPath)
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return CopyFile(path, candidate)
		}
	}
}

// LatestBackup 获取最新的备份文件路径
func LatestBackup(path string) (string, error) {
	bakPath := path + ".bak"
	if _, err := os.Stat(bakPath); os.IsNotExist(err) {
		return "", fmt.Errorf("没有找到 %s 的备份文件", filepath.Base(path))
	}

	latest := bakPath
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.bak.%d", path, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			break
		}
		latest = candidate
	}
	return latest, nil
}

// RestoreFromBackup 从备份恢复文件
func RestoreFromBackup(path string) (string, error) {
	bakPath, err := LatestBackup(path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(bakPath)
	if err != nil {
		return "", fmt.Errorf("无法读取备份 %s: %w", bakPath, err)
	}
	info, err := os.Stat(bakPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, info.Mode()); err != nil {
		return "", fmt.Errorf("无法恢复到 %s: %w", path, err)
	}
	if err := os.Remove(bakPath); err != nil {
		fmt.Fprintf(os.Stderr, "警告：无法删除已恢复的备份 %s: %v\n", bakPath, err)
	}
	return bakPath, nil
}

// BackupFiles 批量备份文件
func BackupFiles(files []string) error {
	for _, f := range files {
		if err := BackupFile(f); err != nil {
			return fmt.Errorf("备份 %s 失败: %w", f, err)
		}
	}
	return nil
}
