package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/malashin/cpbftpchk/xftp"
)

type FtpConn struct {
	Loggerer
	conn      xftp.IFtp
	connected bool
}

func NewFtpConn() *FtpConn {
	return &FtpConn{}
}

func (f *FtpConn) Quit() {
	if !f.connected {
		return
	}
	f.conn.Quit()
	f.connected = false
	f.Log(Debug, "QUIT: Connection closed correctly")
}

func (f *FtpConn) DialAndLogin(addr string) {
	f.ResetError()
	f.Log(Debug, "DIAL_AND_LOGIN: Connecting to "+addr)
	conn, err := xftp.New(addr)
	if err != nil {
		f.Error(err)
		return
	}
	f.conn = conn
	f.connected = true
	f.Log(Debug, "DIAL_AND_LOGIN: Connected to "+addr)
}

func (f *FtpConn) Pwd() string {
	if f.GetError() != nil {
		return ""
	}
	cwd, err := f.conn.CurrentDir()
	if err != nil {
		f.Error(err)
		return ""
	}
	return cwd
}

func (f *FtpConn) Cd(path string) error {
	if f.GetError() != nil {
		return f.GetError()
	}
	f.Log(Debug, "CD: "+path)
	err := f.conn.ChangeDir(path)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "550") {
			f.Error(err)
			return err
		}
		f.Log(Warning, "CD: "+f.Pwd()+"/"+path+": "+err.Error())
		return err
	}
	// f.Log(Debug, "PWD: "+f.Pwd())
	return nil
}

func (f *FtpConn) CdUp() error {
	if f.GetError() != nil {
		return f.GetError()
	}
	f.Log(Debug, "CDUP: "+f.Pwd())
	err := f.conn.ChangeDirToParent()
	if err != nil {
		if !strings.HasPrefix(err.Error(), "550") {
			f.Error(err)
			return err
		}
		f.Log(Warning, "CDUP: "+err.Error())
		return err
	}
	// f.Log(Debug, "PWD: "+f.Pwd())
	return nil
}

func (f *FtpConn) Ls(path string) (entries []xftp.TEntry) {
	if f.GetError() != nil {
		return
	}
	entries, err := f.conn.List(path)
	if err != nil {
		f.Error(err)
		return
	}
	list := []string{}
	for _, file := range entries {
		if !(file.Name == "." || file.Name == "..") {
			list = append(list, file.Name)
		}
	}
	sort.Strings(list)
	f.Log(Debug, "LIST: ", list)
	return entries
}

func (f *FtpConn) Walk(fl map[string]FileEntry, foundFiles *FoundFiles, fileMask *regexp.Regexp, fileMaskIgnore *regexp.Regexp, folderMask *regexp.Regexp, folderMaskIgnore *regexp.Regexp) {
	if f.GetError() != nil {
		return
	}
	entries := f.Ls("")
	cwd := f.Pwd()

	// Add "/" to cwd path
	if cwd != "/" {
		cwd = cwd + "/"
	}

	newLine := Pad(cwd, len(lastLine))
	fmt.Print(newLine + "\r")
	lastLine = cwd

	for _, element := range entries {
		switch element.Type {
		case xftp.File:
			if AcceptFileName(element.Name, fileMask) && !AcceptFileName(element.Name, fileMaskIgnore) {
				key := cwd + element.Name
				entry, fileExists := fl[key]
				if fileExists {
					if entry.Size != element.Size {
						// Old file with new size
						f.Log(Notice, "~ "+TruncPad(key, 64, 'l')+" size changed")
						fl[key] = NewFileEntry(element)
						foundFiles.AddChangedSize(key)
					} else if !entry.Time.Equal(element.Time) {
						// Old file with new date
						f.Log(Notice, "~ "+TruncPad(key, 64, 'l')+" datetime changed")
						fl[key] = NewFileEntry(element)
						foundFiles.AddChangedDate(key)
					} else {
						// Old file
						entry.Found = true
						fl[key] = entry
					}
				} else {
					// New file
					f.Log(Notice, "+ "+TruncPad(key, 64, 'l')+" new file")
					fl[key] = NewFileEntry(element)
					foundFiles.AddNew(key)
				}
			}
		case xftp.Folder:
			if !(element.Name == "." || element.Name == "..") && AcceptFileName(cwd+element.Name, folderMask) && !AcceptFileName(cwd+element.Name, folderMaskIgnore) {
				err := f.Cd(element.Name)
				if err != nil {
					continue
				}
				f.Walk(fl, foundFiles, fileMask, fileMaskIgnore, folderMask, folderMaskIgnore)
				err = f.CdUp()
				if err != nil {
					continue
				}
			}
		}
	}
}
