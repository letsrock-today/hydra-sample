package handler

import (
	"log"
	"net/http"

	"github.com/labstack/echo"
	"github.com/pkg/errors"

	"github.com/letsrock-today/hydra-sample/authkit/middleware"
	"github.com/letsrock-today/hydra-sample/sample/authkit/backend/service/socialprofile"
	"github.com/letsrock-today/hydra-sample/sample/authkit/backend/service/user"
)

func (h handler) Friends(c echo.Context) error {
	u := c.Get(middleware.DefaultContextKey).(user.User)
	friends := []socialprofile.Profile{}
	// iterate over all available social networks and geather all friends
	for p := range h.config.OAuth2Providers() {
		err := h.withOAuth2TokenDo(u, p, func(client *http.Client) error {
			sp, err := socialprofile.New(p.ID())
			if err != nil {
				// strange, should be implemented for every network
				// skip, if not implemented
				log.Println(err)
				return nil
			}
			fl, err := sp.Friends(client)
			if err != nil {
				log.Println(err)
			} else {
				friends = append(friends, fl...)
			}
			return nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return c.JSON(http.StatusOK, friends)
}
