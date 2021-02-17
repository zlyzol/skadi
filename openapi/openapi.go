// Package openapi provides primitives to interact the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen DO NOT EDIT.
package openapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	//"encoding/json"
	"fmt"
	"github.com/deepmap/oapi-codegen/pkg/runtime"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	//"io/ioutil"
	"net/http"
	//"net/url"
	"strings"
)

// StatsData defines model for StatsData.
type StatsData struct {

	// Average PNL of all trade
	AvgPNL *int64 `json:"avgPNL,omitempty"`

	// Total trade amount in RUNE
	AvgTrade *int64 `json:"avgTrade,omitempty"`

	// Running time
	TimeRunning *string `json:"timeRunning,omitempty"`

	// Public IP address 
	IpAddress *string `json:"ipaddress,omitempty"`

	// Total volume of all trade in RUNE
	TotalVolume *int64 `json:"totalVolume,omitempty"`

	// Total trade count
	TradeCount *int64 `json:"tradeCount,omitempty"`
}

// TradesData defines model for TradesData.
type TradesData struct {

	// Amount spent in RUNE
	AmountIn *int64 `json:"amountIn,omitempty"`

	// Amount earned in RUNE
	AmountOut *int64 `json:"amountOut,omitempty"`

	// Asset
	Asset *string `json:"asset,omitempty"`

	// PNL
	Pnl *string `json:"pnl,omitempty"`

	// Trade time
	Time *string `json:"time,omitempty"`
}

// Error defines model for Error.
type Error struct {
	Error string `json:"error"`
}

// GeneralErrorResponse defines model for GeneralErrorResponse.
type GeneralErrorResponse Error

// HealthResponse defines model for HealthResponse.
type HealthResponse struct {
	Database      *bool  `json:"database,omitempty"`
	ScannerHeight *int64 `json:"scannerHeight,omitempty"`
}

// StatsResponse defines model for StatsResponse.
type StatsResponse StatsData

// TradesResponse defines model for TradesResponse.
type TradesResponse TradesData

// RequestEditorFn  is the function signature for the RequestEditor callback function
type RequestEditorFn func(ctx context.Context, req *http.Request) error

// Doer performs HTTP requests.
//
// The standard http.Client implements this interface.
type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ServerInterface represents all server handlers.
type ServerInterface interface {
	// Get Documents
	// (GET /v1/doc)
	GetDocs(ctx echo.Context) error
	// Get Health
	// (GET /v1/health)
	GetHealth(ctx echo.Context) error
	// Get bot statistics
	// (GET /v1/stats)
	GetSwagger(ctx echo.Context) error
	// Get Arbitrage Trades' informations
	// (GET /v1/trades)
	GetStats(ctx echo.Context) error
	// Get Swagger
	// (GET /v1/swagger.json)
	GetTrades(ctx echo.Context) error
}

// ServerInterfaceWrapper converts echo contexts to parameters.
type ServerInterfaceWrapper struct {
	Handler ServerInterface
}

// GetDocs converts echo context to params.
func (w *ServerInterfaceWrapper) GetDocs(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetDocs(ctx)
	return err
}

// GetHealth converts echo context to params.
func (w *ServerInterfaceWrapper) GetHealth(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetHealth(ctx)
	return err
}

// GetStats converts echo context to params.
func (w *ServerInterfaceWrapper) GetStats(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetStats(ctx)
	return err
}

// GetSwagger converts echo context to params.
func (w *ServerInterfaceWrapper) GetSwagger(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetSwagger(ctx)
	return err
}

// GetTrades converts echo context to params.
func (w *ServerInterfaceWrapper) GetTrades(ctx echo.Context) error {
	var err error

	// Invoke the callback with all the unmarshalled arguments
	err = w.Handler.GetTrades(ctx)
	return err
}

// RegisterHandlers adds each server route to the EchoRouter.
func RegisterHandlers(router runtime.EchoRouter, si ServerInterface) {

	wrapper := ServerInterfaceWrapper{
		Handler: si,
	}

	router.GET("/", wrapper.GetStats)
	router.GET("/v1/doc", wrapper.GetDocs)
	router.GET("/v1/health", wrapper.GetHealth)
	router.GET("/v1/stats", wrapper.GetStats)
	router.GET("/v1/swagger.json", wrapper.GetSwagger)
	router.GET("/v1/trades", wrapper.GetTrades)

}

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"H4sIAAAAAAAC/6xWUW/bRgz+KwduwF4My2mHrdDTsqVYA3SZEWd7KfpASbR07enudke5yAL/94EnOaki",
	"uUmMvdm840d+5Hek7qB0rXeWLEfI7yBQ9M5GSn/eERpurgeTWEpnmSzLT/Te6BJZO5t9is6KLZYNtSi/",
	"fHCeAuseqELGAnsIvvUEORTOGUIL+wXEEq2l8I503STorQstMuSgLf/0IywOPtoy1RRgv783ueITlQx7",
	"MVUUy6C9ZAQ5XBN3wUaFVjWJh4qM3EXltuoPXdUYKgm+YeR4EsXvA20hh++yhwpm/WnMEuoFMs5l1ues",
	"JBBqq22tauMKNOrXt+vNF/SqSo4LuAlY0f+eW0J9fm7nodAcsCaVHH+IStu+QdrZmHoxIEvgB+ITEeCu",
	"Xl+9T3IYhTzfUYJfX72X3qAxiiUSLJ4WwkJQU15T3BvHOEApbF1nWWmrrv+6evs8ZNYtXXdWijAFHw6U",
	"XHrwjhzktjhL8L+d6dqjme3S6YjyCxMUl9+E2LfJl+nK4pRnNWjwSENTUS/tTEv7ckdPLy16j/lnx0dB",
	"CYOl6oWoMdIcYjLPdM9bM70t4p3rtJ5tcar8vDrmp5e8qsMbxzJlSy1qAzn864z7paUat47dMn6GybO9",
	"aUhtPmOl1borjC7V+fpS/dNR0BSHA7x/x4VjhbZSPpDHQKMHrbBwHStu+mvsVEEqEFba3CrcoTZYGFJb",
	"F5TvI3WRQlwKS81GSD3OAxawoxD7RM+Wq+VK8neeLHoNObxOpgV45CbpKtudZZUr5Wc917bNF6xrCtkA",
	"oV4vVyK1Um+HgahqshSQqVKVK7tWxp9kKOJNFy4ryOF34gtXRliMl96r1WoaMh4JOY40TMOubTHc9hHU",
	"xSEBqRDWEfIPcLClXOCjOAnnfk0dpf3VQptOaWnYsOYObGSyiPl8fTlLvt/uR+jPLZD7e9mjD4Mp6wH7",
	"wEw2b3yCmPDCEPD2MS3RYUWM2sQ5GmnjnMRivPqnJCSwZK4j66G3iUyvheVh636zWU3XCi9bqRbLRtv+",
	"MaU39FhTIwnPdmwQ/rMUe2rgmTo8hD0IeDPyuBdwWjfxOQKeNFpW4BNfGjMFuekDntL88bfVlPSTnz3i",
	"QUEmG+Qf7qALMqkbZp9n2dmrn2WoLc/yN6s3Mu6+Po8zFz7u/wsAAP//TEw+pIkLAAA=",
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file.
func GetSwagger() (*openapi3.Swagger, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("error loading Swagger: %s", err)
	}
	return swagger, nil
}

