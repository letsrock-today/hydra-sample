package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/labstack/echo"

	"github.com/letsrock-today/hydra-sample/authkit/middleware"
	"github.com/letsrock-today/hydra-sample/sample/authkit/backend/config"
	"github.com/letsrock-today/hydra-sample/sample/authkit/backend/service/socialprofile"
)

func Friends(c echo.Context) error {
	login := c.Get(middleware.DefaultContextKey).(string)
	u, err := Users.User(login)
	if err != nil {
		return err
	}
	cfg := config.GetCfg()
	friends := []socialprofile.Profile{}
	// iterate over all available social networks and geather all friends
	for p := range cfg.OAuth2Providers() {
		token, ok := u.Tokens[p.ID()]
		if !ok {
			// no token? skip this provider
			continue
		}
		pa, err := socialprofile.New(p.ID())
		if err != nil {
			// strange, should be implemented for every network
			// skip, if not implemented
			log.Println(err)
			continue
		}
		ctx := context.Background()
		client := p.OAuth2Config().Client(ctx, token)
		fl, err := pa.Friends(client)
		if err != nil {
			// method not implemented, or other error - just skip it
			log.Println(err)
			continue
		}
		friends = append(friends, fl...)
	}
	return c.JSON(http.StatusOK, friends)
}
