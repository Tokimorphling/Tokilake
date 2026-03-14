package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"one-api/common"
	"one-api/common/config"
	"one-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupGoogleControllerTestDB(t *testing.T) {
	t.Helper()

	oldDB := model.DB
	oldUsingSQLite := common.UsingSQLite

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	model.DB = db
	common.UsingSQLite = true

	t.Cleanup(func() {
		model.DB = oldDB
		common.UsingSQLite = oldUsingSQLite
	})
}

func withGoogleRegisterConfig(t *testing.T, registerEnabled, googleOnly bool) {
	t.Helper()

	oldRegisterEnabled := config.RegisterEnabled
	oldGoogleOnlyRegisterEnabled := config.GoogleOnlyRegisterEnabled

	config.RegisterEnabled = registerEnabled
	config.GoogleOnlyRegisterEnabled = googleOnly

	t.Cleanup(func() {
		config.RegisterEnabled = oldRegisterEnabled
		config.GoogleOnlyRegisterEnabled = oldGoogleOnlyRegisterEnabled
	})
}

func createGoogleTestUser(t *testing.T, username, email, googleID string) *model.User {
	t.Helper()

	user := &model.User{
		Username:    username,
		DisplayName: username,
		Email:       email,
		GoogleId:    googleID,
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))
	return user
}

func TestValidateNewUserRegistration(t *testing.T) {
	withGoogleRegisterConfig(t, false, false)
	require.EqualError(t, validateNewUserRegistration("google"), "管理员关闭了新用户注册")

	withGoogleRegisterConfig(t, true, true)
	require.NoError(t, validateNewUserRegistration("google"))
	require.EqualError(t, validateNewUserRegistration("github"), googleOnlyRegisterMessage)

	withGoogleRegisterConfig(t, true, false)
	require.NoError(t, validateNewUserRegistration("github"))
}

func TestGetUserByGoogleMatchesGoogleIDAndVerifiedEmail(t *testing.T) {
	setupGoogleControllerTestDB(t)

	boundUser := createGoogleTestUser(t, "bound", "bound@example.com", "google-sub-1")
	emailUser := createGoogleTestUser(t, "emailuser", "email@example.com", "")

	user, err := getUserByGoogle(&GoogleUser{Sub: "google-sub-1"})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, boundUser.Id, user.Id)

	user, err = getUserByGoogle(&GoogleUser{
		Sub:           "new-google-sub",
		Email:         "email@example.com",
		EmailVerified: true,
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, emailUser.Id, user.Id)

	user, err = getUserByGoogle(&GoogleUser{
		Sub:           "another-google-sub",
		Email:         "email@example.com",
		EmailVerified: false,
	})
	require.NoError(t, err)
	require.Nil(t, user)
}

func TestRegisterAndVerificationBlockedByGoogleOnly(t *testing.T) {
	withGoogleRegisterConfig(t, true, true)
	oldPasswordRegisterEnabled := config.PasswordRegisterEnabled
	config.PasswordRegisterEnabled = true
	t.Cleanup(func() {
		config.PasswordRegisterEnabled = oldPasswordRegisterEnabled
	})

	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{}`))
	Register(c)
	require.Contains(t, w.Body.String(), googleOnlyRegisterMessage)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/verification?email=test@example.com", nil)
	SendEmailVerification(c)
	require.Contains(t, w.Body.String(), googleOnlyRegisterMessage)
}

func TestUpdateOptionRejectsGoogleOnlyRegisterWithoutGoogleOAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	oldGoogleOAuthEnabled := config.GoogleOAuthEnabled
	oldGoogleClientID := config.GoogleClientId
	oldGoogleClientSecret := config.GoogleClientSecret
	config.GoogleOAuthEnabled = false
	config.GoogleClientId = ""
	config.GoogleClientSecret = ""
	t.Cleanup(func() {
		config.GoogleOAuthEnabled = oldGoogleOAuthEnabled
		config.GoogleClientId = oldGoogleClientID
		config.GoogleClientSecret = oldGoogleClientSecret
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(`{"key":"GoogleOnlyRegisterEnabled","value":"true"}`))
	UpdateOption(c)

	require.Contains(t, w.Body.String(), "无法启用仅 Google 注册")
}

func TestUnbindGoogle(t *testing.T) {
	setupGoogleControllerTestDB(t)
	gin.SetMode(gin.TestMode)

	user := createGoogleTestUser(t, "googleuser", "google@example.com", "google-sub-unbind")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", user.Id)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/unbind", strings.NewReader(`{"type":"google"}`))
	Unbind(c)

	require.Contains(t, w.Body.String(), `"success":true`)

	updatedUser, err := model.GetUserById(user.Id, false)
	require.NoError(t, err)
	require.Equal(t, "", updatedUser.GoogleId)
}
