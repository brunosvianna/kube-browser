package browser

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func tryAppMode(url string, candidates []string, args ...string) error {
	allArgs := append(args, "--app="+url)
	for _, candidate := range candidates {
		cmd := exec.Command(candidate, allArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no supported browser found for app mode")
}

func Open(url string) error {
	var err error

	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LocalAppData")

		candidates := []string{
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		}
		err = tryAppMode(url, candidates, "--new-window")
		if err != nil {
			cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
			return cmd.Start()
		}
		return nil

	case "darwin":
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		err = tryAppMode(url, candidates, "--new-window")
		if err != nil {
			cmd := exec.Command("open", url)
			return cmd.Start()
		}
		return nil

	default:
		candidates := []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
			"microsoft-edge",
			"brave-browser",
		}
		err = tryAppMode(url, candidates, "--new-window")
		if err != nil {
			cmd := exec.Command("xdg-open", url)
			return cmd.Start()
		}
		return nil
	}
}
