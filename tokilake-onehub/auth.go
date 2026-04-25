package tokilake_onehub

import (
	"context"
	"errors"
	"strings"

	"one-api/model"
	tokilake "tokilake-core"
)

type HubAuthenticator struct{}

func NewHubAuthenticator() *HubAuthenticator {
	return &HubAuthenticator{}
}

func (a *HubAuthenticator) AuthenticateTokenKey(ctx context.Context, tokenKey string) (string, *tokilake.Token, error) {
	tokenKey = strings.TrimSpace(tokenKey)
	if strings.HasPrefix(strings.ToLower(tokenKey), "bearer ") {
		tokenKey = strings.TrimSpace(tokenKey[7:])
	}
	tokenKey = strings.TrimSpace(strings.TrimPrefix(tokenKey, "sk-"))
	if tokenKey == "" {
		return "", nil, errors.New("missing authorization token")
	}
	token, err := model.ValidateUserToken(tokenKey)
	if err != nil {
		return "", nil, err
	}
	return tokenKey, &tokilake.Token{UserId: token.UserId}, nil
}
