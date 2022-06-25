package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"plugin"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sony/sonyflake"
)

var ApplicationDescription string = "TaskQ Redis Subscriber"
var BuildVersion string = "0.0.0"

var Debug bool = false
var DebugMetricsNotifierPeriod time.Duration = 60

type PluginStruct struct {
	Command  string `json:"command,omitempty"`
	Filename string `json:"filename,omitempty"`
	Symbol   plugin.Symbol
}

type RedisStruct struct {
	Channel string       `json:"channel,omitempty"`
	Address *net.TCPAddr `json:"address,omitempty"`
}

type ConfigurationStruct struct {
	Plugins []PluginStruct `json:"plugins,omitempty"`
	Redis   RedisStruct    `json:"redis,omitempty"`
}

type handleSignalParamsStruct struct {
	httpServer http.Server
}

type MetricsStruct struct {
	Index     int32
	Warnings  int32
	Errors    int32
	Success   int32
	Incomings int32
	Started   time.Time
}

var Configuration = ConfigurationStruct{}
var handleSignalParams = handleSignalParamsStruct{}

var MetricsNotifierPeriod int = 60
var Metrics = MetricsStruct{
	Index:     0,
	Warnings:  0,
	Errors:    0,
	Success:   0,
	Incomings: 0,
	Started:   time.Now(),
}

var ctx = context.Background()
var flake = sonyflake.NewSonyflake(sonyflake.Settings{})

var rdb *redis.Client

func ReadConfigurationFile(configPtr string, configuration *ConfigurationStruct) {

	log.Info().Msgf("Reading configuration file")

	configFile, _ := os.Open(configPtr)
	defer configFile.Close()

	JSONDecoder := json.NewDecoder(configFile)

	err := JSONDecoder.Decode(&configuration)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error while reading config file")
	}

	log.Info().Msgf("Configuration: %+v", *configuration)

}

func SetupPlugins(configuration *ConfigurationStruct) {

	for item := range configuration.Plugins {

		filename := configuration.Plugins[item].Filename
		log.Info().Msgf("Plugin filename: %+v", filename)

		pluginHandler, err := plugin.Open(filename)
		if err != nil {
			log.Fatal().Err(err).Msgf("Error while opening plugin file:", err)
		}

		symbol, err := pluginHandler.Lookup("ExecCommand")
		if err != nil {
			log.Fatal().Err(err).Msgf("Error while looking up a symbol:", err)
		}

		configuration.Plugins[item].Symbol = symbol

		log.Info().Msgf("symbol: '%v'", symbol)
		log.Info().Msgf("configuration.Plugins[item]: '%v'", configuration.Plugins[item])
	}

	log.Info().Msgf("Configuration: %+v", *configuration)

}

func MetricsNotifier() {
	go func() {
		for {
			time.Sleep(DebugMetricsNotifierPeriod * time.Second)
			log.Debug().
				Int32("Index", Metrics.Index).
				Int32("Incomings", Metrics.Incomings).
				Int32("Warnings", Metrics.Warnings).
				Int32("Errors", Metrics.Errors).
				Int32("Success", Metrics.Success).
				Msg("Metrics")
		}
	}()
}

func handleSignal() {

	log.Debug().Msg("Initialising signal handling function")

	signalChannel := make(chan os.Signal)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	go func() {

		<-signalChannel

		log.Warn().Msg("SIGINT")
		os.Exit(0)

	}()
}

func init() {

	configPtr := flag.String("config", "subscriber.conf", "Path to configuration file")
	redisAddressPtr := flag.String("redis-address", "127.0.0.1:6379", "Address and port of the Redis server")
	verbosePtr := flag.Bool("verbose", false, "Verbose output")
	showVersionPtr := flag.Bool("version", false, "Show version")

	flag.Parse()

	ReadConfigurationFile(*configPtr, &Configuration)
	SetupPlugins(&Configuration)

	if *showVersionPtr {
		fmt.Printf("%s\n", ApplicationDescription)
		fmt.Printf("Version: %s\n", BuildVersion)
		os.Exit(0)
	}

	if *verbosePtr {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		MetricsNotifier()
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Debug().Msg("Logger initialised")

	redis_address, err := net.ResolveTCPAddr("tcp4", *redisAddressPtr)
	if err != nil {
		log.Fatal().Err(err).Msgf("Error while resolving Redis server address")
	}

	Configuration.Redis.Address = redis_address

	if Configuration.Redis.Channel == "" {
		log.Debug().Msg("Setting Redis channel to default value 'junk'")
		Configuration.Redis.Channel = "junk"
	}

	handleSignal()

}

func main() {

	log.Info().Msgf("Preparing Redis connection")
	log.Info().Msgf("Redis server address %s", Configuration.Redis.Address.String())
	log.Info().Msgf("Redis channel %s", Configuration.Redis.Channel)

	rdb = redis.NewClient(&redis.Options{
		Addr:     Configuration.Redis.Address.String(),
		PoolSize: 2000,
	})

	log.Info().Msgf("Subscribing to a channel")

	for {

		result, err := rdb.BLPop(ctx, 0*time.Second, Configuration.Redis.Channel).Result()
		if err != nil {
			log.Error().Err(err).Msgf("Couldn't fetch message")
			continue
		}

		_ = atomic.AddInt32(&Metrics.Incomings, 1)

		log.Info().
			Str("channel", result[0]).
			Int("payload_size", len(result[1])).
			Msgf("Recieved a message: %+v", result[1])

		go func() {
			var symbol_param string = result[1]

			for _, plugin := range Configuration.Plugins {
				log.Info().Msgf("Processing payload with plugin %v", plugin.Command)

				symbol_result, err := plugin.Symbol.(func(string) (string, error))(symbol_param)

				if err != nil {
					log.Error().Err(err).Msgf("Couldn't call plugin")
					continue
				}

				symbol_param = symbol_result

				log.Info().Msgf("Plugin call result %v", symbol_result)

			}
		}()

	}
}
