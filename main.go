package main

import (
	"bufio"
	"context"
	"flag"
	"github.com/hktalent/go4Hacker/cmd"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/subcommands"
	"github.com/sirupsen/logrus"
)

func main() {
	//os.Args = strings.Split(os.Args[0]+" -level WARN serve -4 192.168.0.107 -domain 51pwn.com -lang zh-CN", " ")
	var logFile, logLevel string

	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(&cmd.ServePwCmd{}, "")
	subcommands.Register(&cmd.ResetPwCmd{}, "")

	//https://github.com/mattn/go-sqlite3/issues/39
	flag.StringVar(&logFile, "log", "logs/logs.log", "set log file, option")
	flag.StringVar(&logLevel, "level", "WARN", "set loglevel, option: DEBUG, WARN,INFO,default: WARN")
	flag.Parse()

	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
	case "WARN":
		logrus.SetLevel(logrus.WarnLevel)
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
	default:
		logrus.SetLevel(logrus.WarnLevel)
	}

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Panicf("Open", logFile, err)
		}
		buf := bufio.NewWriter(f)
		defer func() {
			buf.Flush()
			f.Close()
		}()

		//async flush
		go func() {
			for {
				time.Sleep(60 * time.Second)
				buf.Flush()
			}
		}()
		logrus.SetOutput(buf)
		defer buf.Flush()
	}
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
