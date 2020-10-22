package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tb "gopkg.in/tucnak/telebot.v2"
)

type Loggerer struct {
	logger ILogger
	err    error
}

// SetLogger sets objects logger
func (l *Loggerer) SetLogger(logger ILogger) {
	l.logger = logger
}

// Log logs the input text
func (l *Loggerer) Log(loglevel int, text ...interface{}) {
	if l.logger != nil {
		l.logger.Log(loglevel, text...)
	}
}

func (l *Loggerer) Error(text ...interface{}) {
	if l.logger != nil {
		l.logger.Log(Error, "ERROR: "+fmt.Sprint(text...))
	}
	if l.err == nil {
		l.err = errors.New(fmt.Sprint(text...))
	}
}

func (l *Loggerer) ResetError() {
	l.err = nil
}

func (l *Loggerer) GetError() error {
	return l.err
}

type ILogger interface {
	Log(loglevel int, text ...interface{})
}

type Logger struct {
	writers []TWriter
}

type TWriter struct {
	writer   io.Writer
	loglevel int
}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) AddLogger(loglevel int, writer io.Writer) {
	l.writers = append(l.writers, TWriter{writer, loglevel})
}

func (l *Logger) Log(loglevel int, text ...interface{}) {
	for _, writer := range l.writers {
		if loglevel&writer.loglevel != 0 {
			_, err := writer.writer.Write([]byte(time.Now().Format("2006-01-02 15:04:05") + " " + LogLeveltoStr(loglevel) + ": " + fmt.Sprint(text...) + "\n"))
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	if loglevel&Panic != 0 {
		panic(fmt.Sprint(text...))
	}
}

// LogLevel flags
const (
	Quiet = 0
	Panic = 1 << iota
	Error
	Warning
	Notice
	Info
	Debug
)

func LogLevelLeq(loglevel int) int {
	return loglevel - 1 | loglevel
}

func LogLeveltoStr(loglevel int) string {
	s := []string{}
	if loglevel&Panic != 0 {
		s = append(s, "PNC")
	}
	if loglevel&Error != 0 {
		s = append(s, "ERR")
	}
	if loglevel&Warning != 0 {
		s = append(s, "WRN")
	}
	if loglevel&Notice != 0 {
		s = append(s, "NTC")
	}
	if loglevel&Info != 0 {
		s = append(s, "INF")
	}
	if loglevel&Debug != 0 {
		s = append(s, "DBG")
	}
	return strings.Join(s, "|")
}

func NewFileWriter(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return nil, err
	}
	return file, nil
}

type TelegramWriter struct {
	Bot  *tb.Bot
	Chat *tb.Chat
	Msg  []string
}

func NewTelegramWriter(b *tb.Bot, c *tb.Chat) *TelegramWriter {
	return &TelegramWriter{
		Bot:  b,
		Chat: c,
	}
}

func (m *TelegramWriter) Write(p []byte) (n int, err error) {
	m.Msg = append(m.Msg, string(p))
	return 0, nil
}

func (tg *TelegramWriter) Send() error {
	if len(tg.Msg) != 0 {
		var text string
		var isCodeblock bool

		for i, line := range tg.Msg {
			// Split sends if msg length is longer then 4096, leave 3 symbols for "```" to close codeblocks.
			if len(text+line) > 4096-3 {
				if isCodeblock {
					text += "```"
				}

				_, err = tg.Bot.Send(tg.Chat, text, &tb.SendOptions{DisableWebPagePreview: false, ParseMode: tb.ModeMarkdown})
				if err != nil {
					return err
				}

				text = ""
				if isCodeblock {
					text += "```"
				}
			}

			if strings.Count(line, "```")%2 != 0 {
				isCodeblock = !isCodeblock
			}

			text += line

			if i == len(tg.Msg)-1 {
				_, err = tg.Bot.Send(tg.Chat, text, &tb.SendOptions{DisableWebPagePreview: false, ParseMode: tb.ModeMarkdown})
				if err != nil {
					return err
				}
			}
		}

		tg.Msg = []string{}
	}

	return nil
}

// type TMailWriter struct {
// 	msg []string
// }

// func NewMailWriter() *TMailWriter {
// 	return &TMailWriter{}
// }

// func (m *TMailWriter) Write(p []byte) (n int, err error) {
// 	m.msg = append(m.msg, string(p))
// 	return 0, nil
// }

// func (m *TMailWriter) Send() error {
// 	if len(m.msg) != 0 {
// 		body := strings.Join(m.msg, "")
// 		err := pochta.SendMail(smtpserver, auth, from, to, subject, body)
// 		if err != nil {
// 			return err
// 		}
// 		m.msg = []string{}
// 	}
// 	return nil
// }
