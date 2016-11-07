package handler

import (
	"net/http"
	"unicode"

	"github.com/asaskevich/govalidator"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo"
	"github.com/pkg/errors"

	"github.com/letsrock-today/authkit/authkit"
	"github.com/letsrock-today/authkit/authkit/apptoken"
)

type (
	restorePasswordForm struct {
		Email string `form:"email" valid:"required~email-required,email~email-format"`
	}

	changePasswordForm struct {
		Password string `form:"password1" valid:"required~password-required,password~password-format"`
		Token    string `form:"token" valid:"required"`
	}
)

var errEmailInvalid = errors.New("email is invalid or not registered in the app")

func (h handler) RestorePassword(c echo.Context) error {
	var rp restorePasswordForm
	if err := c.Bind(&rp); err != nil {
		return errors.WithStack(err)
	}
	if _, err := govalidator.ValidateStruct(rp); err != nil {
		c.Logger().Debugf("%+v", errors.WithStack(err))
		return c.JSON(
			http.StatusBadRequest,
			h.errorCustomizer.InvalidRequestParameterError(err))
	}

	user, err := h.users.User(rp.Email)
	if err != nil {
		if authkit.IsUserNotFound(err) {
			c.Logger().Debugf("%+v", errors.WithStack(err))
			return c.JSON(
				http.StatusUnauthorized,
				h.errorCustomizer.UserAuthenticationError(err))
		}
		return errors.WithStack(err)
	}

	if err := h.users.RequestPasswordChangeConfirmation(
		user.Email(),
		user.PasswordHash()); err != nil {
		return errors.WithStack(err)
	}
	return c.JSON(http.StatusOK, struct{}{})
}

func (h handler) ChangePassword(c echo.Context) error {
	var cp changePasswordForm
	if err := c.Bind(&cp); err != nil {
		return errors.WithStack(err)
	}
	if _, err := govalidator.ValidateStruct(cp); err != nil {
		c.Logger().Debugf("%+v", errors.WithStack(err))
		return c.JSON(
			http.StatusBadRequest,
			h.errorCustomizer.InvalidRequestParameterError(err))
	}

	s := h.config.OAuth2State
	t, err := apptoken.ParseEmailToken(
		s.TokenIssuer,
		cp.Token,
		s.TokenSignKey)
	if err != nil {
		if err, ok := errors.Cause(err).(*jwt.ValidationError); ok {
			c.Logger().Debugf("%+v", errors.WithStack(err))
			return c.JSON(
				http.StatusUnauthorized,
				h.errorCustomizer.UserAuthenticationError(err))
		}
		return errors.WithStack(err)
	}

	if err = h.users.UpdatePassword(
		t.Login(),
		t.PasswordHash(),
		cp.Password); err != nil {
		if authkit.IsUserNotFound(err) {
			c.Logger().Debugf("%+v", errors.WithStack(err))
			return c.JSON(
				http.StatusUnauthorized,
				h.errorCustomizer.UserAuthenticationError(err))
		}
		return errors.WithStack(err)
	}
	return c.JSON(http.StatusOK, struct{}{})
}

func defaultPasswordValidator(p string) bool {
	l := len(p)
	if l < 5 || l > 50 {
		return false
	}
	lower, upper, digits, other := 0, 0, 0, 0
	for _, r := range p {
		switch {
		case unicode.IsLower(r):
			lower++
		case unicode.IsUpper(r):
			upper++
		case unicode.IsDigit(r):
			digits++
		default:
			other++

		}
		if lower > 0 && upper > 0 && digits > 0 && other > 0 {
			return true
		}
	}
	return false
}
