package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jnewmano/grpc-json-proxy/codec"
	"github.com/jukeizu/management/internal"
	"github.com/jukeizu/management/pkg/management/discord"
	"github.com/jukeizu/management/pkg/treediagram"
	"github.com/oklog/run"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
)

var Version = ""

const DiscordTokenEnvironmentVariable = "TREEDIAGRAM_DISCORD_TOKEN"

var (
	flagVersion = false
	httpPort    = "10002"
)

func parseConfig() {
	flag.StringVar(&httpPort, "http.port", httpPort, "http port for handler")
	flag.BoolVar(&flagVersion, "v", false, "version")

	flag.Parse()
}

func main() {
	parseConfig()

	if flagVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().
		Str("instance", xid.New().String()).
		Str("component", "management").
		Str("version", Version).
		Logger()

	g := run.Group{}

	token := internal.ReadSecretEnv(DiscordTokenEnvironmentVariable)
	discordManagementServer, err := discord.NewDefaultService(logger, token)
	if err != nil {
		logger.Error().Err(err).Caller().Msg("could not create discord management server")
		os.Exit(1)
	}

	httpAddr := ":" + httpPort

	handler := treediagram.NewHandler(logger, httpAddr, discordManagementServer)

	g.Add(func() error {
		return handler.Start()
	}, func(error) {
		err := handler.Stop()
		if err != nil {
			logger.Error().Err(err).Caller().Msg("couldn't stop handler")
		}
	})

	cancel := make(chan struct{})
	g.Add(func() error {
		return interrupt(cancel)
	}, func(error) {
		close(cancel)
	})

	logger.Info().Err(g.Run()).Msg("stopped")
}

func interrupt(cancel <-chan struct{}) error {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-cancel:
		return errors.New("stopping")
	case sig := <-c:
		return fmt.Errorf("%s", sig)
	}
}
