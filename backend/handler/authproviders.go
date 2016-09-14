package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo"

	"github.com/letsrock-today/hydra-sample/backend/config"
	"github.com/letsrock-today/hydra-sample/backend/util/seekingbuffer"
)

type providersReply struct {
	Providers []config.OAuth2Provider `json:"providers"`
}

func AuthProviders(c echo.Context) error {
	c.Response().Header().Set("Expires", time.Now().UTC().Format(http.TimeFormat))
	c.ServeContent(
		seekingbuffer.New(
			func() ([]byte, error) {
				p := providersReply{}
				p.Providers = config.GetConfig().OAuth2Providers

				b, err := json.Marshal(p)
				if err != nil {
					log.Println("Error at AuthProviders, json.Marshal():", err)
				}
				return b, err
			}),
		".json", // mapped to correct type via /etc/mime.types (if not, register it manually)
		config.ModTime())
	return nil
}
