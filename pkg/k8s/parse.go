package k8s

import "strings"

func parseGNUlsOutput(stdout, path string) []FileInfo {
	var files []FileInfo
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		name := strings.Join(fields[7:], " ")
		if name == "." || name == ".." {
			continue
		}

		isDir := strings.HasPrefix(fields[0], "d")
		filePath := buildFilePath(path, name)

		files = append(files, FileInfo{
			Name:    name,
			Size:    fields[4],
			ModTime: fields[5] + " " + fields[6],
			IsDir:   isDir,
			Path:    filePath,
		})
	}
	return files
}

func parseBusyboxOutput(stdout, path string) []FileInfo {
	var files []FileInfo
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		isDir := strings.HasPrefix(fields[0], "d")

		var name, size, modTime string
		if len(fields) >= 9 {
			size = fields[4]
			modTime = fields[5] + " " + fields[6] + " " + fields[7]
			name = strings.Join(fields[8:], " ")
		} else if len(fields) >= 8 {
			size = fields[4]
			modTime = fields[5] + " " + fields[6]
			name = strings.Join(fields[7:], " ")
		} else {
			size = fields[3]
			modTime = fields[4]
			name = strings.Join(fields[5:], " ")
		}

		if name == "." || name == ".." || name == "" {
			continue
		}

		files = append(files, FileInfo{
			Name:    name,
			Size:    size,
			ModTime: modTime,
			IsDir:   isDir,
			Path:    buildFilePath(path, name),
		})
	}
	return files
}

func parseFindOutput(stdout, fullPath, path string) []FileInfo {
	var files []FileInfo
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			name := parts[0]
			if strings.HasPrefix(name, fullPath) {
				name = strings.TrimPrefix(name, fullPath)
				name = strings.TrimPrefix(name, "/")
			}
			if name == "" || name == "." || name == ".." {
				continue
			}
			isDir := strings.Contains(parts[3], "directory")
			files = append(files, FileInfo{
				Name:    name,
				Size:    parts[1],
				ModTime: parts[2],
				IsDir:   isDir,
				Path:    buildFilePath(path, name),
			})
		} else {
			name := line
			if strings.HasPrefix(name, fullPath) {
				name = strings.TrimPrefix(name, fullPath)
				name = strings.TrimPrefix(name, "/")
			}
			if name == "" || name == "." || name == ".." {
				continue
			}
			files = append(files, FileInfo{
				Name:    name,
				Size:    "0",
				ModTime: "-",
				IsDir:   false,
				Path:    buildFilePath(path, name),
			})
		}
	}
	return files
}

func buildFilePath(parent, name string) string {
	if parent == "" || parent == "/" {
		return name
	}
	return parent + "/" + name
}
