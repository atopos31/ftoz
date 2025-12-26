package service

import (
	"os"
	"path/filepath"
)

// Scanner 目录扫描器
type Scanner struct{}

// NewScanner 创建扫描器
func NewScanner() *Scanner {
	return &Scanner{}
}

// Scan 递归扫描目录，返回文件列表和目录列表（相对路径）
func (s *Scanner) Scan(rootDir string) (files []string, dirs []string, err error) {
	stack := []string{""}

	for len(stack) > 0 {
		// Pop
		relDir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		absDir := filepath.Join(rootDir, relDir)
		entries, err := os.ReadDir(absDir)
		if err != nil {
			return nil, nil, err
		}

		for _, entry := range entries {
			relPath := filepath.Join(relDir, entry.Name())

			if entry.IsDir() {
				dirs = append(dirs, relPath)
				stack = append(stack, relPath)
			} else if entry.Type().IsRegular() {
				files = append(files, relPath)
			}
		}
	}

	return files, dirs, nil
}
