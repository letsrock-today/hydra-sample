package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"

	"github.com/letsrock-today/authkit/authkit"
	"github.com/letsrock-today/authkit/authkit/mocks"
)

func TestConfigValidation(t *testing.T) {
	assert := assert.New(t)

	us := new(mocks.UserService)

	// No config at all
	// No obligatory settings
	assert.Panics(func() {
		AccessTokenWithConfig(AccessTokenConfig{})
	})

	// Ensure that defaults are used if non-obligatory settings are not set
	var effcfg *AccessTokenConfig
	reportEffectiveConfig = func(c AccessTokenConfig) {
		effcfg = &c
	}
	AccessTokenWithConfig(AccessTokenConfig{
		UserService:    us,
		TokenValidator: testTokenValidator{},
		AuthHeaderName: "xxx-auth",
	})
	assert.NotNil(effcfg)
	assert.Equal(DefaultContextKey, effcfg.ContextKey)
	assert.Equal(DefaultPermissionMapper{}, effcfg.PermissionMapper)

	// Ensure that defaults are used in AccessToken except fields set explicitly
	effcfg = nil
	AccessToken(
		"xxx-provider",
		us,
		testTokenValidator{})
	assert.NotNil(effcfg)
	assert.Equal(DefaultContextKey, effcfg.ContextKey)
	assert.Equal(DefaultPermissionMapper{}, effcfg.PermissionMapper)
	assert.NotNil(effcfg.UserService)
	assert.NotNil(effcfg.TokenValidator)
}

func TestAccessTokenWithConfig(t *testing.T) {
	user := testUser{"valid@login.ok", "name"}
	us := new(mocks.UserService)
	us.On(
		"User",
		"valid@login.ok").Return(user, nil)
	us.On(
		"User",
		"unknown@login.ok").Return(nil, authkit.NewUserNotFoundError(nil))
	us.On(
		"OAuth2TokenAndLoginByAccessToken",
		"xxx",
		"xxx-provider").Return(&oauth2.Token{
		AccessToken:  "xxx",
		RefreshToken: "rrr",
	}, "valid@login.ok", nil)
	us.On(
		"OAuth2TokenAndLoginByAccessToken",
		"zzz",
		"xxx-provider").Return(nil, "", authkit.NewUserNotFoundError(nil))
	us.On(
		"Principal",
		user).Return(user)

	accessTokenConfig := AccessTokenConfig{
		PrivateProviderID: "xxx-provider",
		UserService:       us,
		PermissionMapper:  testPermMapper{},
		TokenValidator: testTokenValidator{
			allowed: map[string]bool{
				"GET:/permitted:xxx":     true,
				"GET:/permitted:yyy":     true,
				"POST:/permitted:xxx":    true,
				"POST:/permitted:yyy":    true,
				"GET:/permitted-get:xxx": true,
				"GET:/permitted-get:yyy": true,
			},
		},
		OAuth2Config:   testOAuth2Config{},
		ContextCreator: authkit.DefaultContextCreator{},
		AuthHeaderName: "xxx-auth",
	}

	invalidHeaderFormatMsg := errInvalidAuthHeader.Error()
	notPermittedMsg := errAccessDenied.Error()

	cases := []struct {
		name              string
		w                 *httptest.ResponseRecorder
		r                 *http.Request
		accessTokenConfig AccessTokenConfig
		accessTokenHeader string
		expStatusCode     int
		expRespBody       string
		unprotected       bool
	}{
		{
			name:              "Unprotected resource, header empty",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetUnprotected(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "",
			expStatusCode:     http.StatusOK,
			expRespBody:       testNextHandlerMsg,
			unprotected:       true,
		},
		{
			name:              "Protected resource, header empty",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetRestricted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "",
			expStatusCode:     http.StatusForbidden,
			expRespBody:       invalidHeaderFormatMsg,
		},
		{
			name:              "Protected resource, invalid header format (no 'bearer')",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetRestricted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "zzz",
			expStatusCode:     http.StatusForbidden,
			expRespBody:       invalidHeaderFormatMsg,
		},
		{
			name:              "Protected resource, invalid header format (no token after 'bearer')",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetRestricted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer   ",
			expStatusCode:     http.StatusForbidden,
			expRespBody:       invalidHeaderFormatMsg,
		},
		{
			name:              "Restricted resource, valid token",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetRestricted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer xxx",
			expStatusCode:     http.StatusForbidden,
			expRespBody:       notPermittedMsg,
		},
		{
			name:              "Permitted resource, valid token, GET",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetPermitted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer xxx",
			expStatusCode:     http.StatusOK,
			expRespBody:       testNextHandlerMsg,
		},
		{
			name:              "Permitted resource, valid token, POST",
			w:                 httptest.NewRecorder(),
			r:                 testNewPostPermitted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer xxx",
			expStatusCode:     http.StatusOK,
			expRespBody:       testNextHandlerMsg,
		},
		{
			name:              "Permitted resource, invalid token",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetPermitted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer zzz",
			expStatusCode:     http.StatusForbidden,
			expRespBody:       notPermittedMsg,
		},
		{
			name:              "Permitted resource, unknown token",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetPermitted(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer yyy", // token with permission, but without user
			expStatusCode:     http.StatusForbidden,
			expRespBody:       notPermittedMsg,
		},
		{
			name:              "Permitted only get resource, valid token, GET",
			w:                 httptest.NewRecorder(),
			r:                 testNewGetPermittedOnlyGet(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer xxx",
			expStatusCode:     http.StatusOK,
			expRespBody:       testNextHandlerMsg,
		},
		{
			name:              "Permitted only get resource, valid token, POST",
			w:                 httptest.NewRecorder(),
			r:                 testNewPostPermittedOnlyGet(t),
			accessTokenConfig: accessTokenConfig,
			accessTokenHeader: "bearer xxx",
			expStatusCode:     http.StatusForbidden,
			expRespBody:       notPermittedMsg,
		},
	}

	for _, cs := range cases {
		cs := cs
		// e.Any(...) and brothers should not be used in parallel
		next := testNextHandler{
			checkPrincipal: !cs.unprotected,
		}
		e := echo.New()
		e.Any("/unprotected", next.next)
		e.Any(
			"/permitted",
			next.next,
			AccessTokenWithConfig(cs.accessTokenConfig))
		e.Any(
			"/permitted-get",
			next.next,
			AccessTokenWithConfig(cs.accessTokenConfig))
		e.Any(
			"/restricted",
			next.next,
			AccessTokenWithConfig(cs.accessTokenConfig))
		t.Run(cs.name, func(st *testing.T) {
			st.Parallel()
			assert := assert.New(st)
			r := cs.r
			w := cs.w
			cs.r.Header.Set(echo.HeaderAuthorization, cs.accessTokenHeader)
			e.ServeHTTP(echo.NewResponse(w, e), r)
			assert.Equal(cs.expStatusCode, w.Code)
			assert.Equal(cs.expRespBody, string(w.Body.Bytes()))
		})
	}
}

func testNewGetUnprotected(t *testing.T) *http.Request {
	r, err := http.NewRequest(echo.GET, "/unprotected", nil)
	assert.NoError(t, err)
	return r
}

func testNewGetPermitted(t *testing.T) *http.Request {
	r, err := http.NewRequest(echo.GET, "/permitted", nil)
	assert.NoError(t, err)
	return r
}

func testNewPostPermitted(t *testing.T) *http.Request {
	r, err := http.NewRequest(echo.POST, "/permitted", nil)
	assert.NoError(t, err)
	return r
}

func testNewGetPermittedOnlyGet(t *testing.T) *http.Request {
	r, err := http.NewRequest(echo.GET, "/permitted-get", nil)
	assert.NoError(t, err)
	return r
}

func testNewPostPermittedOnlyGet(t *testing.T) *http.Request {
	r, err := http.NewRequest(echo.POST, "/permitted-get", nil)
	assert.NoError(t, err)
	return r
}

func testNewGetRestricted(t *testing.T) *http.Request {
	r, err := http.NewRequest(echo.GET, "/restricted", nil)
	assert.NoError(t, err)
	return r
}

const testNextHandlerMsg = "Result from next handler"

type testNextHandler struct {
	hasRun         bool
	checkPrincipal bool
}

func (n *testNextHandler) next(c echo.Context) error {
	n.hasRun = true
	// check if user data is available in context
	if !n.checkPrincipal {
		return c.String(http.StatusOK, testNextHandlerMsg)
	}
	u := c.Get(DefaultContextKey)
	user, ok := u.(testUser)
	if !ok {
		return errors.New("no user in context")
	}
	if user.Name != "name" {
		return errors.New("invalid user in context")
	}
	return c.String(http.StatusOK, testNextHandlerMsg)
}

type testUser struct {
	login string
	Name  string
}

func (u testUser) Login() string {
	return u.login
}

func (u testUser) Email() string {
	return u.login
}

func (u testUser) PasswordHash() string {
	return "some-hash"
}

type testPermMapper struct{}

func (testPermMapper) RequiredPermissioin(
	method, path string) (interface{}, error) {
	return method + ":" + path, nil
}

type testTokenValidator struct {
	allowed map[string]bool
}

func (v testTokenValidator) Validate(
	token string,
	perm interface{}) (string, error) {
	if b, ok := v.allowed[perm.(string)+":"+token]; !ok || !b {
		return "", errors.New("forbidden")
	}
	login := ""
	switch token {
	case "xxx":
		login = "valid@login.ok"
	case "yyy":
		login = "unknown@login.ok"
	}
	return login, nil
}

type testOAuth2Config struct{}

func (testOAuth2Config) TokenSource(
	ctx context.Context, t *oauth2.Token) oauth2.TokenSource {
	return testTokenSource{t}
}

func (testOAuth2Config) AuthCodeURL(
	state string,
	opts ...oauth2.AuthCodeOption) string {
	panic("not implemented")
}

func (testOAuth2Config) PasswordCredentialsToken(
	ctx context.Context,
	username, password string) (*oauth2.Token, error) {
	panic("not implemented")
}

func (testOAuth2Config) Exchange(
	ctx context.Context,
	code string) (*oauth2.Token, error) {
	panic("not implemented")
}

func (testOAuth2Config) Client(
	ctx context.Context,
	t *oauth2.Token) *http.Client {
	panic("not implemented")
}

type testTokenSource struct {
	t *oauth2.Token
}

func (s testTokenSource) Token() (*oauth2.Token, error) {
	if s.t.Valid() {
		return s.t, nil
	}
	return &oauth2.Token{
		AccessToken:  "xxx",
		RefreshToken: "rrr",
	}, nil
}
