package main

import (
	"io/ioutil"
	"sync"
	"os"
	"os/signal"

	"github.com/uw-ictd/haulage/tdf"
	log "github.com/sirupsen/logrus"
	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v2"
)

var opts struct {
	ConfigPath string `short:"c" long:"config" description:"The path to the configuration file" required:"true" default:"/etc/haulage/config.yml"`
}

func parseConfig(path string) tdf.Config {
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.WithField("path", path).WithError(err).Fatal("Failed to load configuration")
	}

	log.Debug("Parsing" + path)
	var config tdf.Config
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.WithError(err).Fatal("Failed to parse configuration")
	}
	return config
}

func setupSigintHandler(trafficDetector tdf.TrafficDetector) {
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, os.Interrupt)

	go func() {
		<-sigintChan
		trafficDetector.Close()
		<-sigintChan
		log.Fatal("Terminating Uncleanly! Connections may be orphaned.")
	}()
}

func main() {
	log.Info("Starting haulage")

	// Setup flags
	_, err := flags.Parse(&opts)
	if err != nil {
		log.Fatal(err)
	}

	config := parseConfig(opts.ConfigPath)

	var processingGroup sync.WaitGroup
	trafficDetector, err := tdf.CreateTDF(config, processingGroup)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer trafficDetector.Close()

	trafficDetector.StartDetection();
	//StartPCRF();

	processingGroup.Wait()
}
