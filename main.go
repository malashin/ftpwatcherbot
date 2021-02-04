package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/malashin/cpbftpchk/xftp"
	tb "gopkg.in/tucnak/telebot.v2"
)

type config struct {
	TGChatID            string
	LogPath             string
	FileListPath        string
	FTPLogin            string
	FTPRootPath         string
	FTPFileMask         *regexp.Regexp
	FTPFileMaskIgnore   *regexp.Regexp
	FTPFolderMask       *regexp.Regexp
	FTPFolderMaskIgnore *regexp.Regexp
	LastUpdate          string
}

var c xftp.IFtp
var err error

var fileListRe = regexp.MustCompile(`\?\{(.*)\?\}(.*)\?\|(\d+)\?\|(.*)$`)

var sleepTime = 30 * time.Minute
var lastLine string

// var botToken = ""
// var configs = []config{}

func main() {
	// Get filepath to executable.
	bin, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	binPath := filepath.Dir(bin)

	// Telegram bot.
	pref := tb.Settings{
		Token:  botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tb.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	// Telegram handles.
	b.Handle("/chatid", func(m *tb.Message) {
		b.Send(m.Chat, m.Chat.Recipient())
	})

	b.Handle("/lastupdate", func(m *tb.Message) {
		for _, cfg := range configs {
			if m.Chat.Recipient() == cfg.TGChatID {
				b.Send(m.Chat, cfg.LastUpdate)
			}
		}
	})

	go b.Start()

	for {
		for _, cfg := range configs {
			StartJob(b, cfg, binPath)
		}
		time.Sleep(sleepTime)
	}
}

func StartJob(b *tb.Bot, cfg *config, binPath string) {
	c, err := b.ChatByID(cfg.TGChatID)
	if err != nil {
		log.Fatal(err)
	}

	tgWriter := NewTelegramWriter(b, c)

	// Create watcher objects.
	logger := NewLogger()
	logger.AddLogger(LogLevelLeq(Info), os.Stdout)
	// logger.AddLogger(LogLevelLeq(Debug), os.Stdout)
	logger.AddLogger(LogLevelLeq(Warning), tgWriter)
	ftpConn := NewFtpConn()
	ftpConn.SetLogger(logger)
	fileList := NewFileList(filepath.Join(binPath, cfg.FileListPath))
	fileList.SetLogger(logger)

	// Load file list.
	fileList.Load(filepath.Join(binPath, cfg.FileListPath))

	// Properly close the connection on exit.
	defer ftpConn.Quit()

	// Initialize the connection to the specified ftp server address.
	ftpConn.DialAndLogin(cfg.FTPLogin)

	// Change directory to watcherRootPath.
	ftpConn.Cd(cfg.FTPRootPath)

	// Walk the directory tree.
	if ftpConn.GetError() == nil {
		foundFiles := FoundFiles{}
		logger.Log(Debug, "Looking for new files...")
		ftpConn.Walk(fileList.files, &foundFiles, cfg.FTPFileMask, cfg.FTPFileMaskIgnore, cfg.FTPFolderMask, cfg.FTPFolderMaskIgnore)
		fmt.Print(Pad("", len(lastLine)) + "\r")

		// Terminate the FTP connection.
		ftpConn.Quit()
		if ftpConn.GetError() == nil {
			// Remove deleted files from the fileList.
			deletedFiles := fileList.Clean()

			// Write message for telegram.
			if foundFiles.Found || len(deletedFiles) > 0 {
				tgWriter.Write([]byte(time.Now().Format("*2006-01-02 15:04:05*")))

				if len(foundFiles.New) > 0 {
					tgWriter.Write([]byte("\n*new files:*\n```\n"))
					for _, f := range foundFiles.New {
						tgWriter.Write([]byte(f + "\n"))
					}
					tgWriter.Write([]byte("```\n"))
				}

				if len(foundFiles.ChangedSize) > 0 {
					tgWriter.Write([]byte("\n*changed size:*\n```\n"))
					for _, f := range foundFiles.ChangedSize {
						tgWriter.Write([]byte(f + "\n"))
					}
					tgWriter.Write([]byte("```\n"))
				}

				if len(foundFiles.ChangedDate) > 0 {
					tgWriter.Write([]byte("\n*changed datetime:*\n```\n"))
					for _, f := range foundFiles.ChangedDate {
						tgWriter.Write([]byte(f + "\n"))
					}
					tgWriter.Write([]byte("```\n"))
				}
			}

			if len(deletedFiles) > 0 {
				tgWriter.Write([]byte("\n*deleted:*\n```\n"))
				for _, f := range deletedFiles {
					tgWriter.Write([]byte(f + "\n"))
				}
				tgWriter.Write([]byte("```\n"))
			}

			// Send telegram message.
			err := tgWriter.Send()
			if err != nil {
				logger.Log(Error, err)
			}

			// Save new fileList.
			fileList.Save()

			// Empty found files list.
			foundFiles = FoundFiles{}
		}
	}

	if ftpConn.GetError() == nil {
		cfg.LastUpdate = time.Now().Format("2006-01-02 15:04:05")
		fmt.Print("Updated at " + cfg.LastUpdate + "\r")
	}
}
