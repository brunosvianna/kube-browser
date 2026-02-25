package browser

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("browser exited with error: %w", err)
		}
		return nil
	case <-time.After(2 * time.Second):
		return nil
	}
}

func runAndCheck(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s: %w", name, err)
	}
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s failed: %w (stderr: %s)", name, err, strings.TrimSpace(stderr.String()))
		}
		return nil
	case <-time.After(5 * time.Second):
		return nil
	}
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
	return runAndCheck("rundll32", "url.dll,FileProtocolHandler", url)
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
	return runAndCheck("open", url)
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
		"firefox",
	}

	if browser := findBrowser(candidates); browser != "" {
		log.Printf("Opening in app mode: %s", browser)
		if err := openAppMode(browser, url); err == nil {
			return nil
		}
		log.Printf("App mode failed, trying normal mode: %s", browser)
		if err := runAndCheck(browser, url); err == nil {
			return nil
		}
	}

	openers := []string{"xdg-open", "sensible-browser", "x-www-browser", "gnome-open"}
	for _, opener := range openers {
		if p, err := exec.LookPath(opener); err == nil {
			log.Printf("Trying %s: %s", opener, p)
			if err := runAndCheck(opener, url); err == nil {
				return nil
			}
			log.Printf("%s failed: %v", opener, err)
		}
	}

	browsers := []string{"firefox", "chromium", "google-chrome", "brave-browser"}
	for _, b := range browsers {
		if p, err := exec.LookPath(b); err == nil {
			log.Printf("Trying direct browser: %s", p)
			if err := runAndCheck(b, url); err == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("no browser found - please open %s manually", url)
}
