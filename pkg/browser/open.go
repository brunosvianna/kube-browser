package browser

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func findBrowser(candidates []string) string {
	for _, c := range candidates {
		if filepath.IsAbs(c) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		} else {
			if p, err := exec.LookPath(c); err == nil {
				return p
			}
		}
	}
	return ""
}

func openAppMode(browserPath, url string) error {
	cmd := exec.Command(browserPath, "--new-window", "--app="+url)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}

	go func() {
		cmd.Wait()
	}()

	time.Sleep(300 * time.Millisecond)

	if cmd.Process != nil {
		return nil
	}
	return fmt.Errorf("browser process exited immediately")
}

func Open(url string) error {
	switch runtime.GOOS {
	case "windows":
		return openWindows(url)
	case "darwin":
		return openDarwin(url)
	default:
		return openLinux(url)
	}
}

func openWindows(url string) error {
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")
	localAppData := os.Getenv("LocalAppData")

	candidates := []string{
		filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
		filepath.Join(programFiles, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
	}

	if browser := findBrowser(candidates); browser != "" {
		log.Printf("Opening in app mode: %s", browser)
		if err := openAppMode(browser, url); err == nil {
			return nil
		}
	}

	log.Printf("Falling back to default browser")
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	return cmd.Start()
}

func openDarwin(url string) error {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}

	if browser := findBrowser(candidates); browser != "" {
		log.Printf("Opening in app mode: %s", browser)
		if err := openAppMode(browser, url); err == nil {
			return nil
		}
	}

	log.Printf("Falling back to 'open' command")
	cmd := exec.Command("open", url)
	return cmd.Start()
}

func openLinux(url string) error {
	candidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"microsoft-edge",
		"microsoft-edge-stable",
		"brave-browser",
	}

	if browser := findBrowser(candidates); browser != "" {
		log.Printf("Opening in app mode: %s", browser)
		if err := openAppMode(browser, url); err == nil {
			return nil
		}
	}

	if p, err := exec.LookPath("xdg-open"); err == nil {
		log.Printf("Falling back to xdg-open: %s", p)
		cmd := exec.Command("xdg-open", url)
		return cmd.Start()
	}

	if p, err := exec.LookPath("sensible-browser"); err == nil {
		log.Printf("Falling back to sensible-browser: %s", p)
		cmd := exec.Command("sensible-browser", url)
		return cmd.Start()
	}

	return fmt.Errorf("no browser found")
}
