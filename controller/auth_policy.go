package controller

import (
	"errors"

	"one-api/common/config"
)

const googleOnlyRegisterMessage = "管理员仅允许通过 Google 注册新用户"

func validateNewUserRegistration(provider string) error {
	if !config.RegisterEnabled {
		return errors.New("管理员关闭了新用户注册")
	}
	if config.GoogleOnlyRegisterEnabled && provider != "google" {
		return errors.New(googleOnlyRegisterMessage)
	}
	return nil
}
