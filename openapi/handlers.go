package openapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"gitlab.com/zlyzol/skadi/internal/bot"
)

// Handlers data structure is the api/interface into the policy business logic service
type Handlers struct {
	bot    *bot.Bot
	logger zerolog.Logger
}

// New creates a new service interface with the Datastore of your choise
func New(bot *bot.Bot, logger zerolog.Logger) *Handlers {
	return &Handlers{
		bot:    bot,
		logger: logger,
	}
}

// (GET /v1/stop)
func (h *Handlers) Stop(ctx echo.Context) error {
	err := h.bot.StopHunters()
	var res string
	if err == nil {
		res = "Bot stopped"
	} else {
		res = "Failed to stop bot: " + err.Error()
	}
	return ctx.JSON(http.StatusOK, res)
}

// (GET /v1/start)
func (h *Handlers) Start(ctx echo.Context) error {
	err := h.bot.StartHunters()
	var res string
	if err == nil {
		res = "Bot started"
	} else {
		res = "Failed to start bot: " + err.Error()
	}
	return ctx.JSON(http.StatusOK, res)
}

// This swagger/openapi 3.0 generated documentation// (GET /v1/doc)
func (h *Handlers) GetDocs(ctx echo.Context) error {
	return ctx.File("./public/delivery/http/doc.html")
}

// JSON swagger/openapi 3.0 specification endpoint// (GET /v1/swagger.json)
func (h *Handlers) GetSwagger(ctx echo.Context) error {
	swagger, _ := GetSwagger()
	return ctx.JSONPretty(http.StatusOK, swagger, "   ")
}

// (GET /v1/health)
func (h *Handlers) GetHealth(ctx echo.Context) error {
	health := h.bot.GetHealth()
	return ctx.JSON(http.StatusOK, health)
}

// (GET /v1/start)
func (h *Handlers) GetStart(ctx echo.Context) error {
	response := h.bot.GetStart()
	return ctx.JSON(http.StatusOK, response)
}

// (GET /v1/stop)
func (h *Handlers) GetStop(ctx echo.Context) error {
	response := h.bot.GetStop()
	return ctx.JSON(http.StatusOK, response)
}

// (GET /v1/stats)
func (h *Handlers) GetStats(ctx echo.Context) error {
	stats, err := h.bot.GetStats()
	if err != nil {
		h.logger.Err(err).Msg("failure with GetStats")
		return echo.NewHTTPError(http.StatusInternalServerError, GeneralErrorResponse{Error: err.Error()})
	}
	return ctx.JSON(http.StatusOK, stats)
}

// (GET /v1/trades)
func (h *Handlers) GetTrades(ctx echo.Context) error {
	trades, err := h.bot.GetTrades()
	if err != nil {
		h.logger.Err(err).Msg("failure with GetTrades")
		return echo.NewHTTPError(http.StatusInternalServerError, GeneralErrorResponse{Error: err.Error()})
	}
	return ctx.JSON(http.StatusOK, trades)
}

// (GET /v1/wallet)
func (h *Handlers) GetWallet(ctx echo.Context) error {
	wallet, err := h.bot.GetWallet()
	if err != nil {
		h.logger.Err(err).Msg("failure with GetWallet")
		return echo.NewHTTPError(http.StatusInternalServerError, GeneralErrorResponse{Error: err.Error()})
	}
	return ctx.JSON(http.StatusOK, wallet)
}
