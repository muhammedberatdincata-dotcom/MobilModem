package embed

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// AssetDir is the directory where embedded assets are extracted.
// We use %TEMP%\MobilModem to avoid polluting the user's filesystem.
var (
	assetDir string
	once     sync.Once
)

// GetAssetDir returns the path to the extracted assets directory.
// Creates it on first call.
func GetAssetDir() string {
	once.Do(func() {
		assetDir = filepath.Join(os.TempDir(), "MobilModem")
		os.MkdirAll(assetDir, 0755)
	})
	return assetDir
}

// ExtractAsset writes raw bytes to a file in the asset directory.
// Skips extraction if file already exists and has the same size.
func ExtractAsset(name string, data []byte) (string, error) {
	dir := GetAssetDir()
	outPath := filepath.Join(dir, name)

	// Skip if already extracted with matching size
	if info, err := os.Stat(outPath); err == nil && info.Size() == int64(len(data)) {
		return outPath, nil
	}

	if err := os.WriteFile(outPath, data, 0755); err != nil {
		return "", fmt.Errorf("failed to extract %s: %v", name, err)
	}
	return outPath, nil
}

// CleanupAssets removes the temporary asset directory.
func CleanupAssets() {
	if assetDir != "" {
		os.RemoveAll(assetDir)
	}
}

// AdbExe is placeholder bytes. In a real build, replace this file with
// go:embed directives pointing to the actual binaries placed in an assets/ folder.
//
// To embed real files, create assets/ directory with the binaries and use:
//
//	//go:embed assets/adb.exe
//	var AdbExeData []byte
//
//	//go:embed assets/AdbWinApi.dll
//	var AdbWinApiData []byte
//
//	//go:embed assets/AdbWinUsbApi.dll
//	var AdbWinUsbApiData []byte
//
//	//go:embed assets/wintun.dll
//	var WintunDllData []byte
//
// For now, we provide stub variables that the build system must populate.
// The adb and wintun modules will look for files in GetAssetDir() first.

var (
	AdbExeData       []byte
	AdbWinApiData    []byte
	AdbWinUsbApiData []byte
	WintunDllData    []byte
)

// ExtractAll extracts all embedded assets to the temp directory.
// Returns a map of asset name -> extracted path.
func ExtractAll() (map[string]string, error) {
	paths := make(map[string]string)

	type asset struct {
		name string
		data []byte
	}

	assets := []asset{
		{"adb.exe", AdbExeData},
		{"AdbWinApi.dll", AdbWinApiData},
		{"AdbWinUsbApi.dll", AdbWinUsbApiData},
		{"wintun.dll", WintunDllData},
	}

	for _, a := range assets {
		if len(a.data) == 0 {
			continue // Skip assets not embedded in this build
		}
		p, err := ExtractAsset(a.name, a.data)
		if err != nil {
			return nil, err
		}
		paths[a.name] = p
	}

	return paths, nil
}
