package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/malashin/ffinfo"
)

var watchlistMap = make(map[string]File)
var watchlistFilePath = "ftpWatcherWatchlist.txt"

type File struct {
	FileName string
	Probe    ffinfo.File
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func isFileNeeded(fileName string) bool {
	if _, ok := watchlistMap[strings.ToLower(fileName)]; ok {
		return true
	}
	return false
}

// func waitForFullUpload(ftpConn *FtpConn, filePath string) error {
// 	ftpConn.DialAndLogin(ftpLogin)
// 	defer ftpConn.Quit()

// 	size1, err := ftpConn.conn.FileSize(filePath)
// 	if err != nil {
// 		return err
// 	}

// 	time.Sleep(5 * time.Minute)

// 	for {
// 		size2, err := ftpConn.conn.FileSize(filePath)
// 		if err != nil {
// 			return err
// 		}

// 		if size1 == size2 {
// 			return nil
// 		}

// 		size1 = size2
// 		time.Sleep(5 * time.Minute)
// 	}
// }
