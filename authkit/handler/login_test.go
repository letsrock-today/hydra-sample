package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/labstack/echo"
	"github.com/labstack/echo/engine/standard"
	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {

	as := new(testAuthService)
	as.On(
		"IssueConsentToken",
		"some_client_id",
		[]string{"some_scope"}).Return("valid_token", nil)
	as.On(
		"IssueConsentToken",
		"unknown_client_id",
		[]string{"some_scope"}).Return("", errors.New("unknown_client"))

	us := new(testUserService)
	us.On(
		"Authenticate",
		"valid@login.ok",
		"invalid_password").Return(newTestUserNotFoundError())
	us.On(
		"Authenticate",
		"valid@login.ok",
		"valid_password").Return(nil)
	us.On(
		"Create",
		"new.valid@login.ok",
		"valid_password").Return(newTestAccountDisabledError())
	us.On(
		"Create",
		"broken.valid@login.ok",
		"valid_password").Return(newTestAccountDisabledError())
	us.On(
		"Create",
		"old.valid@login.ok",
		"valid_password").Return(newTestDuplicateUserError())
	us.On(
		"RequestEmailConfirmation",
		"new.valid@login.ok").Return(nil)
	us.On(
		"RequestEmailConfirmation",
		"old.valid@login.ok").Return(nil)
	us.On(
		"RequestEmailConfirmation",
		"broken.valid@login.ok").Return(UserServiceError(errors.New("cannot send email")))

	h := handler{
		errorCustomizer: testErrorCustomizer{},
		auth:            as,
		users:           us,
		config: &testConfig{
			privateOAuth2Provider: testOAuth2Provider{
				id:       "some_id",
				clientID: "some_client_id",
				scopes:   []string{"some_scope"},
			},
		},
	}

	h2 := handler{
		errorCustomizer: testErrorCustomizer{},
		auth:            as,
		users:           us,
		config: &testConfig{
			privateOAuth2Provider: testOAuth2Provider{
				id:       "some_id",
				clientID: "unknown_client_id",
				scopes:   []string{"some_scope"},
			},
		},
	}

	cases := []struct {
		name                  string
		params                url.Values
		expStatusCode         int
		expBody               string
		expBodyRegex          bool
		internalError         bool
		failIssueConsentToken bool
	}{
		{
			name:          "No params",
			params:        make(url.Values),
			expStatusCode: http.StatusBadRequest,
			expBody:       `{"Code":"invalid req param"}`,
		},
		{
			name: "Login: invalid password",
			params: url.Values{
				"action":   []string{"login"},
				"login":    []string{"valid@login.ok"},
				"password": []string{"invalid_password"},
			},
			expStatusCode: http.StatusUnauthorized,
			expBody:       `{"Code":"user auth err"}`,
		},
		{
			name: "Login: OK",
			params: url.Values{
				"action":   []string{"login"},
				"login":    []string{"valid@login.ok"},
				"password": []string{"valid_password"},
			},
			expStatusCode: http.StatusOK,
			expBodyRegex:  true,
			expBody:       `\{"redirUrl":".*consent=valid_token.*"\}`,
		},
		{
			name: "Signup: OK",
			params: url.Values{
				"action":   []string{"signup"},
				"login":    []string{"new.valid@login.ok"},
				"password": []string{"valid_password"},
			},
			// account disabled until user confirms it
			expStatusCode: http.StatusUnauthorized,
			expBody:       `{"Code":"acc disabled"}`,
		},
		{
			name: "Signup: duplicate",
			params: url.Values{
				"action":   []string{"signup"},
				"login":    []string{"old.valid@login.ok"},
				"password": []string{"valid_password"},
			},
			expStatusCode: http.StatusUnauthorized,
			expBody:       `{"Code":"dup user"}`,
		},
		{
			name: "Signup: cannot send email",
			params: url.Values{
				"action":   []string{"signup"},
				"login":    []string{"broken.valid@login.ok"},
				"password": []string{"valid_password"},
			},
			internalError: true,
		},
		{
			name: "Login: fail IssueConsentToken",
			params: url.Values{
				"action":   []string{"login"},
				"login":    []string{"valid@login.ok"},
				"password": []string{"valid_password"},
			},
			internalError:         true,
			failIssueConsentToken: true,
		},
	}

	for _, c := range cases {
		c := c
		for _, enc := range bodyEncoders {
			enc := enc
			t.Run(c.name+", "+enc.name, func(st *testing.T) {
				st.Parallel()
				assert := assert.New(st)
				e := echo.New()

				req, err := http.NewRequest(echo.POST, "", enc.encoder(c.params))
				assert.NoError(err)
				rec := httptest.NewRecorder()
				ctx := e.NewContext(
					standard.NewRequest(req, e.Logger()),
					standard.NewResponse(rec, e.Logger()))
				ctx.Request().Header().Set("Content-Type", enc.contentType)

				if c.failIssueConsentToken {
					err = h2.Login(ctx)
				} else {
					err = h.Login(ctx)
				}
				if enc.invalid || c.internalError {
					assert.Error(err)
				} else {
					assert.NoError(err)
					assert.Equal(c.expStatusCode, rec.Code)
					if c.expBodyRegex {
						assert.Regexp(c.expBody, string(rec.Body.Bytes()))
					} else {
						assert.Equal(c.expBody, string(rec.Body.Bytes()))
					}
				}
			})
		}
	}

}
