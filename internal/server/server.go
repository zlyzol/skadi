package server

import (
	"context"
	//	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/ziflex/lecho/v2"

	//	rpchttp "github.com/tendermint/tendermint/rpc/client/http"

	"gitlab.com/zlyzol/skadi/internal/bot"
	"gitlab.com/zlyzol/skadi/internal/config"
	"gitlab.com/zlyzol/skadi/internal/store/mongo"

	//"gitlab.com/thorchain/midgard/internal/usecase"
	//"gitlab.com/thorchain/midgard/pkg/clients/thorchain"
	//httpdelivery "gitlab.com/thorchain/midgard/pkg/delivery/http"
	httpdelivery "gitlab.com/zlyzol/skadi/openapi"
)

// Server
type Server struct {
	cfg        config.Configuration
	srv        *http.Server
	logger     zerolog.Logger
	echoEngine *echo.Echo
	bot        *bot.Bot
}

func initLog(level string, pretty bool) zerolog.Logger {
	l, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Warn().Msgf("%s is not a valid log-level, falling back to 'info'", level)
	}
	// logging init
	//logFile, err := os.OpenFile("log_" + time.Now().Format("20060102_150405") + ".txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	logFile, err := os.OpenFile("log.txt", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		panic("failed to open log file")
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	var out io.Writer = mw
	//var out io.Writer = os.Stdout
	if pretty {
		out = zerolog.ConsoleWriter{Out: out}
	}
	//log.Logger = zerolog.New(out)
	zerolog.TimeFieldFormat = "15:04:05"
	log.Logger = log.Output(out)

	if level == "debug" {
	//	log.Logger = log.With().Caller().Logger()
	}
	log.Info().Msg("log started")

	zerolog.SetGlobalLevel(l)
	return log.Output(out).With().Str("service", "skadi").Logger()
}

func NewServer(cfgFile *string) (*Server, error) {
	// Load config
	cfg, err := config.LoadConfiguration(*cfgFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load chain service config")
	}

	log := initLog(cfg.LogLevel, cfg.Pretty)

	mongo, err := mongo.NewClient(cfg.Mongo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create mongodb client instance")
	}

	bot, err := bot.NewBot(mongo, cfg)
	if err != nil {
		if err != nil {
			return nil, errors.Wrap(err, "failed to create bot instance")
		}
	}

	// Setup echo
	echoEngine := echo.New()
	echoEngine.Use(middleware.Recover())

	// CORS default
	// Allows requests from any origin wth GET, HEAD, PUT, POST or DELETE method.
	echoEngine.Use(middleware.CORS())

	logger := log.With().Str("module", "httpServer").Logger()

	// Initialise handlers
	h := httpdelivery.New(bot, logger)

	// Register handlers
	httpdelivery.RegisterHandlers(echoEngine, h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%v", cfg.ListenPort),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	return &Server{
		echoEngine: echoEngine,
		cfg:        *cfg,
		srv:        srv,
		logger:     logger,
		bot:        bot,
	}, nil
}

func (s *Server) Start() error {
	//s.registerWhiteListedProxiedRoutes()
	s.registerEchoWithLogger()
	// Serve HTTP
	go s.startServer()
	return s.bot.Start()
}

func (s *Server) startServer() {
	err := s.echoEngine.StartServer(s.srv)
	s.echoEngine.Logger.Fatal(err)
}

func (s *Server) Stop() error {
	if err := s.bot.Stop(); nil != err {
		s.logger.Error().Err(err).Msg("failed to stop bot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer cancel()
	return s.echoEngine.Shutdown(ctx)
}

func (s *Server) Log() *zerolog.Logger {
	return &s.logger
}

func (s *Server) registerWhiteListedProxiedRoutes() {
	endpoints := []string{"stats", "trades"}
	for _, endpoint := range endpoints {
		endpointParts := strings.Split(endpoint, ":")
		path := fmt.Sprintf("/v1/%s", endpoint)
		log.Info().Str("path", path).Msg("Proxy route created")
		s.echoEngine.GET(path, func(c echo.Context) error {
			return nil
		}, func(handlerFunc echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				req := c.Request()
				res := c.Response()

				// delete duplicate header
				res.Header().Del("Access-Control-Allow-Origin")

				var u *url.URL
				var err error
				// Handle endpoints without any path parameters
				if len(endpointParts) == 1 {
					u, err = url.Parse(s.cfg.Scheme + "://" + s.cfg.Host + "/" + endpointParts[0])
					if err != nil {
						log.Error().Err(err).Msg("Failed to Parse url")
						return err
					}
					// Handle endpoints with path parameters
				} else {
					reqUrlParts := strings.Split(req.URL.EscapedPath(), "/")
					u, err = url.Parse(s.cfg.Scheme + "://" + s.cfg.Host + "/" + endpointParts[0] + reqUrlParts[len(reqUrlParts)-1])
					if err != nil {
						log.Error().Err(err).Msg("Failed to Parse url")
						return err
					}
				}

				log.Info().Str("url", u.String()).Msg("Proxied url")
				proxyHTTP(u).ServeHTTP(res, req)
				return nil
			}
		})
	}
}

func proxyHTTP(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", target.Host)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path
	}
	return proxy
}

func (s *Server) registerEchoWithLogger() {
	l := lecho.New(s.logger)
	s.echoEngine.Use(lecho.Middleware(lecho.Config{Logger: l}))
	s.echoEngine.Use(middleware.RequestID())
}
