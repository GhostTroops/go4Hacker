package hacker

import (
	_ "github.com/projectdiscovery/fdmax/autofdmax"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/naabu/v2/pkg/runner"
	"log"
	"os"
	"os/signal"
)

func PortScan(options *runner.Options) {
	//var options *runner.Options
	//options = &runner.Options{}
	naabuRunner, err := runner.NewRunner(options)
	if err != nil {
		log.Println(err)
		return
	}
	// Setup graceful exits
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			naabuRunner.ShowScanResultOnExit()
			gologger.Info().Msgf("CTRL+C pressed: Exiting\n")
			if options.ResumeCfg.ShouldSaveResume() {
				gologger.Info().Msgf("Creating resume file: %s\n", runner.DefaultResumeFilePath())
				err := options.ResumeCfg.SaveResumeConfig()
				if err != nil {
					gologger.Error().Msgf("Couldn't create resume file: %s\n", err)
				}
			}
			naabuRunner.Close()
			//os.Exit(1)
			break
		}
	}()

	err = naabuRunner.RunEnumeration()
	if err != nil {
		gologger.Fatal().Msgf("Could not run enumeration: %s\n", err)
	}
	// on successful execution remove the resume file in case it exists
	options.ResumeCfg.CleanupResumeConfig()
}

//func main() {
//
//}
