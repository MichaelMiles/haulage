package main

import (
	"github.com/uw-ictd/haulage/tdf"
	"github.com/uw-ictd/haulage/pcrf"
	log "github.com/sirupsen/logrus"
	"github.com/jessevdk/go-flags"
)

const opts struct {
	ConfigPath string `short:"c" long:"config" description:"The path to the configuration file" required:"true" default:"/etc/haulage/config.yml"`
}

func parseConfig(path string) config {
	var config Config
	configBytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.WithField("path", path).WithError(err).Fatal("Failed to load configuration")
	}

	log.Debug("Parsing" + path)
	var config config
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		log.WithError(err).Fatal("Failed to parse configuration")
	}
	return config
}

func setupSigintHandler(trafficDetector TrafficDetector, ctx Context) {s
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, os.Interrupt)

	go func() {
		<-sigintChan
		trafficDetector.Close()
		OnStop(&ctx)
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

	params := Parameters{
		config.Custom.DBLocation,
		config.Custom.DBUser,
		config.Custom.DBPass,
		config.FlowLogInterval,
		config.UserLogInterval,
		config.Custom.ReenableUserPollInterval,
	}
	log.WithField("Parameters", config).Info("Parsed parameters")


	var processingGroup sync.WaitGroup
	trafficDetector := tdf.createTDF(config, processingGroup)
	defer trafficDetector.Close()

	log.Info("Initializing context")
	OnStart(&ctx, params)
	log.Info("Context initialization complete")
	start_radius_server(ctx.db)
	defer Cleanup(&ctx)
	log.Info("Context initialization complete")


	trafficDetector.StartDetection();
	//StartPCRF();

	processingGroup.Wait()
}
