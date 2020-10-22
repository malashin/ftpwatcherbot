package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/malashin/cpbftpchk/xftp"
)

func Pad(s string, n int) string {
	if n > len(s) {
		return s + strings.Repeat(" ", n-len(s))
	}
	return s
}

func TruncPad(s string, n int, side byte) string {
	if len(s) > n {
		if n >= 1 {
			return "…" + s[len(s)-n+1:]
		}
		return s[len(s)-n:]
	}
	if side == 'r' {
		return strings.Repeat(" ", n-len(s)) + s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func AcceptFileName(fileName string, fileMask *regexp.Regexp) bool {
	if fileMask.MatchString(fileName) {
		return true
	}
	return false
}

func NewFileEntry(entry xftp.TEntry) FileEntry {
	file := FileEntry{}
	file.Name = entry.Name
	file.Size = entry.Size
	file.Time = entry.Time
	file.Found = true
	return file
}

type FileEntry struct {
	Name  string
	Size  int64
	Time  time.Time
	Found bool
}

func (fe *FileEntry) Pack() (string, error) {
	time, err := fe.Time.MarshalText()
	if err != nil {
		return "", err
	}
	return fe.Name + "?|" + fmt.Sprintf("%v", fe.Size) + "?|" + string(time), nil
}

type FoundFiles struct {
	New         []string
	ChangedSize []string
	ChangedDate []string
	Found       bool
}

func (ff *FoundFiles) AddNew(s string) {
	ff.New = append(ff.New, s)
	ff.Found = true
}

func (ff *FoundFiles) AddChangedSize(s string) {
	ff.ChangedSize = append(ff.ChangedSize, s)
	ff.Found = true
}

func (ff *FoundFiles) AddChangedDate(s string) {
	ff.ChangedDate = append(ff.ChangedDate, s)
	ff.Found = true
}

type TFileList struct {
	Loggerer
	files map[string]FileEntry
	path  string
}

func NewFileList(path string) *TFileList {
	return &TFileList{files: map[string]FileEntry{}, path: path}
}

func (fl *TFileList) Pack() (string, error) {
	output := []string{}
	for key, value := range fl.files {
		valueString, err := value.Pack()
		if err != nil {
			return "", err
		}
		output = append(output, "?{"+key+"?}"+valueString+"\n")
	}
	sort.Strings(output)
	return strings.Join(output, ""), nil
}

func (fl *TFileList) Clean() (deletedFiles []string) {
	for key, value := range fl.files {
		if !value.Found {
			delete(fl.files, key)
			fl.Log(Info, "- "+TruncPad(key, 64, 'l')+" deleted")
			deletedFiles = append(deletedFiles, key)
		} else {
			value.Found = false
			fl.files[key] = value
		}
	}

	return deletedFiles
}

func (fl TFileList) String() (string, error) {
	return fl.Pack()
}

func (fl *TFileList) Load(filepath string) {
	var file *os.File

	fl.Log(Debug, "Loading \""+filepath+"\"...")
	if _, err := os.Stat(filepath); err != nil {
		file, err = os.Create(filepath)
		if err != nil {
			fl.Log(Error, err)
			return
		}
	} else {
		file, err = os.Open(filepath)
		if err != nil {
			fl.Log(Error, "\""+filepath+"\" not found")
			return
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, entry := fl.ParseLine(scanner.Text())
		if key != "" {
			fl.files[key] = entry
		}
	}

	err = scanner.Err()
	if err != nil {
		fl.Log(Error, err)
	}
	fl.Log(Debug, "FileList loaded. ", fl.files)
}

func (fl *TFileList) Save() {
	backupPath := fl.path + time.Now().Format("20060102150405")

	// If save file exists, back it up.
	if _, err := os.Stat(fl.path); err == nil {
		input, err := ioutil.ReadFile(fl.path)
		if err != nil {
			fl.Log(Error, err)
			return
		}

		err = ioutil.WriteFile(backupPath, input, 0644)
		if err != nil {
			fl.Log(Error, err)
			return
		}
	}

	file, err := os.Create(fl.path)
	if err != nil {
		fl.Log(Error, err)
		return
	}
	defer file.Close()

	// Get file list ready for writing into file.
	fileListValue, err := fl.Pack()
	if err != nil {
		fl.Log(Error, err)
		return
	}

	// Write file list file.
	_, err = io.Copy(file, strings.NewReader(fileListValue))
	if err != nil {
		fl.Log(Error, err)
		return
	}

	// Delete backup.
	if _, err := os.Stat(fl.path); err == nil {
		err = os.Remove(backupPath)
		if err != nil {
			fl.Log(Error, err)
			return
		}
	}

	// Print sorted file list to debug log.
	list := []string{}
	for key := range fl.files {
		list = append(list, key)
	}

	sort.Strings(list)
	fl.Log(Debug, "FileList saved. ", list)
}

func (fl *TFileList) ParseLine(line string) (string, FileEntry) {
	// "?{/AMEDIATEKA/ANIMALS_2/SER_05620.mxf?}SER_05620.mxf?|13114515508?|2017-03-17 14:39:39 +0000 UTC"
	if !fileListRe.MatchString(line) {
		fl.Log(Error, "Wrong input in file list ("+line+")")
		return "", FileEntry{}
	}
	matches := fileListRe.FindStringSubmatch(line)
	key := matches[1]
	entry := FileEntry{}
	entry.Name = matches[2]
	entrySize, err := strconv.ParseInt(matches[3], 0, 64)
	entry.Size = int64(entrySize)
	err = entry.Time.UnmarshalText([]byte(matches[4]))
	if err != nil {
		fl.Log(Error, "Wrong input in file list ("+line+")")
		return "", FileEntry{}
	}
	if err != nil {
		fl.Log(Error, err)
		return "", FileEntry{}
	}
	return key, entry
}
