package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"

	embedpkg "github.com/mobilmodem/windows-client/internal/embed"
	"github.com/mobilmodem/windows-client/internal/gui"
)

func isElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

func runElevated() error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}

	exePtr, err := syscall.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}

	cwd := filepath.Dir(exe)
	cwdPtr, err := syscall.UTF16PtrFromString(cwd)
	if err != nil {
		return err
	}

	var showCmd int32 = 1 // SW_NORMAL
	err = windows.ShellExecute(0, verb, exePtr, nil, cwdPtr, showCmd)
	if err != nil {
		return fmt.Errorf("ShellExecute başarısız: %v", err)
	}

	return nil
}

func main() {
	// 1. Ensure Administrator Privileges (Required for Wintun, Netsh & Route)
	if !isElevated() {
		err := runElevated()
		if err != nil {
			windows.MessageBox(
				0,
				windows.StringToUTF16Ptr("Uygulama sanal ağ adaptörü ve yönlendirme oluşturabilmek için Yönetici İzni gerektirir."),
				windows.StringToUTF16Ptr("Yönetici Yetkisi Gerekli"),
				windows.MB_ICONERROR|windows.MB_OK,
			)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// 2. Extract Embedded Assets to %TEMP%\MobilModem (adb.exe, wintun.dll, etc.)
	_, _ = embedpkg.ExtractAll()

	// 3. Initialize GUI App
	appInstance := gui.NewApp()

	// 4. Listen for OS Shutdown Signals for clean routing rollback
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		appInstance.GracefulShutdown()
		os.Exit(0)
	}()

	// 5. Run Application
	appInstance.Run()

	// 6. Cleanup upon regular exit
	appInstance.GracefulShutdown()
}
