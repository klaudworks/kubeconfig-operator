package test

import (
	"path/filepath"
	"runtime"
)

// RootDir returns the root directory of the project
func RootDir() string {
	_, b, _, _ := runtime.Caller(0)
	// this file is located at internal/test, which is two directories from the root
	return filepath.Join(filepath.Dir(b), "../..")
}

// CRDPaths returns the paths to this project's CRD manifests
func CRDPaths() []string {
	return []string{
		filepath.Join(RootDir(), "manifests", "crd", "bases"),
	}
}
