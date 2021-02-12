package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	flag "github.com/spf13/pflag"
	"gitlab.com/zlyzol/skadi/internal/server"
)

func main() {
	cfgFile := flag.StringP("cfg", "c", "config", "configuration file with extension")
	flag.Parse()

	s, err := server.NewServer(cfgFile)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create service")
	}

	if err := s.Start(); err != nil {
		log.Fatal().Err(err).Msg("failed to start server")
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	s.Log().Info().Msg("stop signal received")
	if err := s.Stop(); nil != err {
		s.Log().Fatal().Err(err).Msg("failed to stop chain service")
	}
}
