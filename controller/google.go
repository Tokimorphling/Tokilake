package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"one-api/common/config"
	"one-api/common/utils"
	"one-api/model"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	googleIssuer   = "https://accounts.google.com"
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleJWKSURL  = "https://www.googleapis.com/oauth2/v3/certs"
)

var googleKeySet = oidc.NewRemoteKeySet(context.Background(), googleJWKSURL)

type GoogleUser struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func getGoogleOAuthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     config.GoogleClientId,
		ClientSecret: config.GoogleClientSecret,
		RedirectURL:  config.ServerAddress + "/oauth/google",
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  googleAuthURL,
			TokenURL: googleTokenURL,
		},
	}
}

func getGoogleUserInfoByCode(code string) (*GoogleUser, error) {
	if code == "" {
		return nil, errors.New("无效的参数")
	}
	if config.GoogleClientId == "" || config.GoogleClientSecret == "" {
		return nil, errors.New("Google OAuth 配置不完整")
	}

	oauthConfig := getGoogleOAuthConfig()
	ctx := context.Background()
	token, err := oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, errors.New("Google 授权失败，请稍后重试")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("Google 返回的身份令牌无效")
	}

	verifier := oidc.NewVerifier(googleIssuer, googleKeySet, &oidc.Config{ClientID: config.GoogleClientId})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, errors.New("Google 身份令牌校验失败")
	}

	var googleUser GoogleUser
	if err := idToken.Claims(&googleUser); err != nil {
		return nil, errors.New("Google 用户信息解析失败")
	}
	if googleUser.Sub == "" {
		return nil, errors.New("Google 用户标识为空")
	}

	return &googleUser, nil
}

func getUserByGoogle(googleUser *GoogleUser) (*model.User, error) {
	if model.IsGoogleIdAlreadyTaken(googleUser.Sub) {
		return model.FindUserByField("google_id", googleUser.Sub)
	}

	if googleUser.EmailVerified && googleUser.Email != "" && model.IsEmailAlreadyTaken(googleUser.Email) {
		return model.FindUserByField("email", googleUser.Email)
	}

	return nil, nil
}

func sanitizeGoogleUsername(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
		case r == '_', r == '-', r == '.':
			builder.WriteRune('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func buildGoogleUsername(googleUser *GoogleUser) string {
	candidate := ""
	if googleUser.Email != "" {
		parts := strings.SplitN(googleUser.Email, "@", 2)
		candidate = sanitizeGoogleUsername(parts[0])
	}
	if candidate == "" {
		candidate = sanitizeGoogleUsername(googleUser.Name)
	}
	if candidate == "" || model.IsUsernameAlreadyTaken(candidate) {
		return "google_" + strconv.Itoa(model.GetMaxUserId()+1)
	}
	return candidate
}

func applyGoogleProfile(user *model.User, googleUser *GoogleUser) {
	user.GoogleId = googleUser.Sub

	if user.DisplayName == "" {
		if googleUser.Name != "" {
			user.DisplayName = googleUser.Name
		} else if user.Username != "" {
			user.DisplayName = user.Username
		}
	}

	if user.AvatarUrl == "" && googleUser.Picture != "" {
		user.AvatarUrl = googleUser.Picture
	}

	if user.Email == "" && googleUser.EmailVerified && googleUser.Email != "" && !model.IsEmailAlreadyTaken(googleUser.Email) {
		user.Email = googleUser.Email
	}
}

func GoogleEndpoint(c *gin.Context) {
	if !config.GoogleOAuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启通过 Google 登录",
		})
		return
	}
	if config.GoogleClientId == "" || config.GoogleClientSecret == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Google OAuth 配置不完整",
		})
		return
	}

	session := sessions.Default(c)
	state := utils.GetRandomString(12)
	session.Set("oauth_state", state)
	if err := session.Save(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    getGoogleOAuthConfig().AuthCodeURL(state),
	})
}

func GoogleOAuth(c *gin.Context) {
	if !config.GoogleOAuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启通过 Google 登录以及注册",
		})
		return
	}
	if config.GoogleClientId == "" || config.GoogleClientSecret == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Google OAuth 配置不完整",
		})
		return
	}

	session := sessions.Default(c)
	state := c.Query("state")
	if state == "" || session.Get("oauth_state") == nil || state != session.Get("oauth_state").(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "state is empty or not same",
		})
		return
	}

	if session.Get("username") != nil {
		GoogleBind(c)
		return
	}

	code := c.Query("code")
	affCode := c.Query("aff")

	googleUser, err := getGoogleUserInfoByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	user, err := getUserByGoogle(googleUser)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if user == nil {
		if err := validateNewUserRegistration("google"); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		var inviterId int
		if affCode != "" {
			inviterId, _ = model.GetUserIdByAffCode(affCode)
		}

		user = &model.User{
			Username: buildGoogleUsername(googleUser),
			Role:     config.RoleCommonUser,
			Status:   config.UserStatusEnabled,
		}
		if inviterId > 0 {
			user.InviterId = inviterId
		}
		applyGoogleProfile(user, googleUser)

		if err := user.Insert(inviterId); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	} else {
		applyGoogleProfile(user, googleUser)
		if err := user.Update(false); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	if user.Status != config.UserStatusEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "用户已被封禁",
		})
		return
	}

	setupLogin(user, c)
}

func GoogleBind(c *gin.Context) {
	if !config.GoogleOAuthEnabled {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "管理员未开启通过 Google 登录以及注册",
		})
		return
	}
	if config.GoogleClientId == "" || config.GoogleClientSecret == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Google OAuth 配置不完整",
		})
		return
	}

	code := c.Query("code")
	googleUser, err := getGoogleUserInfoByCode(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if model.IsGoogleIdAlreadyTaken(googleUser.Sub) {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该 Google 账户已被绑定",
		})
		return
	}

	session := sessions.Default(c)
	id := session.Get("id")
	user := model.User{Id: id.(int)}
	if err := user.FillUserById(); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	applyGoogleProfile(&user, googleUser)
	if err := user.Update(false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "bind",
	})
}
