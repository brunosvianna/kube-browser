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
        cmd.Env = os.Environ()

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

func isWSL() bool {
        data, err := os.ReadFile("/proc/version")
        if err != nil {
                return false
        }
        lower := strings.ToLower(string(data))
        return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func openWSL(url string) error {
        winBrowsers := []string{
                "/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
                "/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
                "/mnt/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
                "/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe",
                "/mnt/c/Program Files/BraveSoftware/Brave-Browser/Application/brave.exe",
        }

        for _, b := range winBrowsers {
                if _, err := os.Stat(b); err == nil {
                        log.Printf("WSL: Opening in app mode: %s", b)
                        if err := openAppMode(b, url); err == nil {
                                return nil
                        }
                        log.Printf("WSL: App mode failed, trying normal: %s", b)
                        if err := runAndCheck(b, url); err == nil {
                                return nil
                        }
                }
        }

        if p, err := exec.LookPath("wslview"); err == nil {
                log.Printf("WSL: Trying wslview: %s", p)
                if err := runAndCheck("wslview", url); err == nil {
                        return nil
                }
        }

        if p, err := exec.LookPath("cmd.exe"); err == nil {
                log.Printf("WSL: Trying cmd.exe /c start")
                if err := runAndCheck(p, "/c", "start", url); err == nil {
                        return nil
                }
        }

        if p, err := exec.LookPath("powershell.exe"); err == nil {
                log.Printf("WSL: Trying powershell.exe Start-Process")
                if err := runAndCheck(p, "-Command", "Start-Process", "'"+url+"'"); err == nil {
                        return nil
                }
        }

        if _, err := os.Stat("/mnt/c/Windows/explorer.exe"); err == nil {
                log.Printf("WSL: Trying explorer.exe")
                if err := runAndCheck("/mnt/c/Windows/explorer.exe", url); err == nil {
                        return nil
                }
        }

        return fmt.Errorf("WSL: could not open browser - please open %s manually", url)
}

func openLinux(url string) error {
        if isWSL() {
                log.Printf("WSL detected, using Windows browser")
                if err := openWSL(url); err == nil {
                        return nil
                }
                log.Printf("WSL browser open failed, trying Linux methods")
        }

        chromiumBrowsers := []string{
                "google-chrome",
                "google-chrome-stable",
                "chromium",
                "chromium-browser",
                "microsoft-edge",
                "microsoft-edge-stable",
                "brave-browser",
        }

        if browser := findBrowser(chromiumBrowsers); browser != "" {
                log.Printf("Opening in app mode: %s", browser)
                if err := openAppMode(browser, url); err == nil {
                        return nil
                }
                log.Printf("App mode failed for %s", browser)
        }

        allBrowsers := []string{
                "google-chrome",
                "google-chrome-stable",
                "chromium",
                "chromium-browser",
                "microsoft-edge",
                "microsoft-edge-stable",
                "brave-browser",
                "firefox",
                "firefox-esr",
                "/snap/bin/firefox",
                "/snap/bin/chromium",
        }

        for _, b := range allBrowsers {
                if found := findBrowser([]string{b}); found != "" {
                        log.Printf("Trying browser: %s", found)
                        if err := runAndCheck(found, url); err == nil {
                                return nil
                        }
                        log.Printf("  failed: %v", found)
                }
        }

        openers := []string{"xdg-open", "sensible-browser", "x-www-browser", "gnome-open", "kde-open"}
        for _, opener := range openers {
                if p, err := exec.LookPath(opener); err == nil {
                        log.Printf("Trying opener: %s", p)
                        if err := runAndCheck(opener, url); err == nil {
                                return nil
                        }
                        log.Printf("  %s returned error: %v", opener, err)
                }
        }

        if p, err := exec.LookPath("python3"); err == nil {
                log.Printf("Trying python3 webbrowser module")
                if err := runAndCheck(p, "-m", "webbrowser", url); err == nil {
                        return nil
                }
        }

        if p, err := exec.LookPath("python"); err == nil {
                log.Printf("Trying python webbrowser module")
                if err := runAndCheck(p, "-m", "webbrowser", url); err == nil {
                        return nil
                }
        }

        return fmt.Errorf("no browser found - please open %s manually", url)
}
