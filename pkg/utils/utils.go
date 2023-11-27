package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Host struct {
	IP     string
	Suffix string
}

func ReadFiltersFromUser(path string) (string, error) {
	reader, err := resolveReader(path, false)
	if err != nil {
		return "", err
	}

	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ReadInventoryFromUser(path string) ([]Host, error) {
	reader, err := resolveReader(path, true)
	if err != nil {
		return nil, err
	}

	var hosts []Host
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		host := createHost(scanner.Text())
		if host != nil {
			hosts = append(hosts, *host)
		}
	}
	return hosts, scanner.Err()
}

func ReadFilesFromUser(path string) ([][]byte, error) {
	var files [][]byte
	switch {
	case path != "":
		if info, err := os.Stat(path); err == nil {
			if info.IsDir() {
				entries, err := os.ReadDir(path)
				if err != nil {
					return nil, err
				}

				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					file, err := readFile(filepath.Join(path, entry.Name()))
					if err != nil {
						return nil, err
					}
					files = append(files, file)
				}
			} else {
				file, err := readFile(path)
				if err != nil {
					return nil, err
				}
				files = append(files, file)
			}
			return files, nil
		} else {
			return nil, fmt.Errorf("failed to read file, %v", err)
		}
	case fileExistInStdin():
		stdin, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		files = append(files, stdin)
		return files, nil
	default:
		return nil, fmt.Errorf("no value, file or stdin available")
	}
}

func TimeStamp() string {
	ts := time.Now().Format(time.DateOnly)
	return strings.Replace(ts, "-", "_", -1)
}

func createHost(text string) *Host {
	hostWithFileName := strings.Split(text, " ")
	if len(hostWithFileName) < 1 {
		return nil
	}

	var suffix string
	if len(hostWithFileName) == 2 {
		suffix = hostWithFileName[1]
	}
	return &Host{
		IP:     hostWithFileName[0],
		Suffix: suffix,
	}
}

func resolveReader(path string, skipStdin bool) (io.Reader, error) {
	switch {
	case path != "":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return f, nil
	case !skipStdin && fileExistInStdin():
		return os.Stdin, nil
	default:
		return nil, fmt.Errorf("no file or stdin available")
	}
}

func readFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, _ = bytes.CutSuffix(b, []byte("\n"))
	return b, nil
}

func fileExistInStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}
