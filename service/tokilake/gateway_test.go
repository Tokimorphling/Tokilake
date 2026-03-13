package tokilake

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

var gatewayTestSeq atomic.Int64

func setupGatewayTestDB(t *testing.T) {
	t.Helper()

	viper.Reset()
	config.InitConf()
	viper.Set("sqlite_path", filepath.Join(t.TempDir(), "tokilake-test.db"))

	common.UsingPostgreSQL = false
	common.UsingSQLite = false
	config.IsMasterNode = true
	logger.SetupLogger()

	err := model.InitDB()
	require.NoError(t, err)

	sqlDB, err := model.DB.DB()
	require.NoError(t, err)

	model.ChannelGroup.Load()
	model.GlobalUserGroupRatio.Load()

	t.Cleanup(func() {
		_ = sqlDB.Close()
		viper.Reset()
		common.UsingPostgreSQL = false
		common.UsingSQLite = false
	})
}

func createGatewayTestUser(t *testing.T, group string) *model.User {
	t.Helper()

	suffix := gatewayTestSeq.Add(1)
	user := &model.User{
		Username:    fmt.Sprintf("toki-user-%d", suffix),
		Password:    "password123",
		DisplayName: fmt.Sprintf("Toki User %d", suffix),
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		AccessToken: fmt.Sprintf("access-%d", suffix),
		Group:       group,
		AffCode:     fmt.Sprintf("aff-%d", suffix),
		CreatedTime: time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func createPrivateGroupGrant(t *testing.T, ownerUserID int, targetUserID int, groupSlug string) {
	t.Helper()

	group, _, err := model.CreatePrivateGroup(ownerUserID, groupSlug)
	require.NoError(t, err)

	_, err = model.GrantPrivateGroupAccess(
		group.Id,
		targetUserID,
		model.PrivateGroupGrantRoleMember,
		model.PrivateGroupGrantSourceAdmin,
		fmt.Sprintf("test:%d", targetUserID),
		ownerUserID,
		0,
	)
	require.NoError(t, err)
}

func TestRegisterWorkerSessionEnforcesNamespaceOwnership(t *testing.T) {
	setupGatewayTestDB(t)

	owner := createGatewayTestUser(t, "owner-primary")
	other := createGatewayTestUser(t, "other-primary")

	manager := NewSessionManager()

	ownerSession := &GatewaySession{Token: &model.Token{UserId: owner.Id}}
	firstResult, err := registerWorkerSession(manager, ownerSession, &RegisterMessage{
		Namespace: "owner-locked-namespace",
		Models:    []string{"model-a"},
	})
	require.NoError(t, err)
	require.Equal(t, "owner-primary", firstResult.Group)

	manager.Release(ownerSession)

	otherSession := &GatewaySession{Token: &model.Token{UserId: other.Id}}
	_, err = registerWorkerSession(manager, otherSession, &RegisterMessage{
		Namespace: "owner-locked-namespace",
		Models:    []string{"model-a"},
	})
	require.Error(t, err)

	var cpErr *controlPlaneError
	require.True(t, errors.As(err, &cpErr))
	require.Equal(t, "namespace_not_owned", cpErr.code)

	reconnectSession := &GatewaySession{Token: &model.Token{UserId: owner.Id}}
	secondResult, err := registerWorkerSession(manager, reconnectSession, &RegisterMessage{
		Namespace: "owner-locked-namespace",
		Models:    []string{"model-b"},
	})
	require.NoError(t, err)
	require.Equal(t, firstResult.WorkerID, secondResult.WorkerID)
	require.Equal(t, firstResult.ChannelID, secondResult.ChannelID)
}

func TestRegisterWorkerSessionAuthorizesPrimaryAndPrivateGroups(t *testing.T) {
	setupGatewayTestDB(t)

	user := createGatewayTestUser(t, "primary-group")
	privateGroupOwner := createGatewayTestUser(t, "owner-group")
	createPrivateGroupGrant(t, privateGroupOwner.Id, user.Id, "joined-private-group")

	manager := NewSessionManager()

	defaultSession := &GatewaySession{Token: &model.Token{UserId: user.Id}}
	defaultResult, err := registerWorkerSession(manager, defaultSession, &RegisterMessage{
		Namespace: "default-group-namespace",
		Models:    []string{"model-a"},
	})
	require.NoError(t, err)
	require.Equal(t, "primary-group", defaultResult.Group)

	manager.Release(defaultSession)

	privateSession := &GatewaySession{Token: &model.Token{UserId: user.Id}}
	privateResult, err := registerWorkerSession(manager, privateSession, &RegisterMessage{
		Namespace: "private-group-namespace",
		Group:     "joined-private-group",
		Models:    []string{"model-a"},
	})
	require.NoError(t, err)
	require.Equal(t, "joined-private-group", privateResult.Group)

	manager.Release(privateSession)

	unauthorizedSession := &GatewaySession{Token: &model.Token{UserId: user.Id}}
	_, err = registerWorkerSession(manager, unauthorizedSession, &RegisterMessage{
		Namespace: "unauthorized-group-namespace",
		Group:     "forbidden-group",
		Models:    []string{"model-a"},
	})
	require.Error(t, err)

	var cpErr *controlPlaneError
	require.True(t, errors.As(err, &cpErr))
	require.Equal(t, "group_not_authorized", cpErr.code)
}

func TestSyncWorkerModelsRejectsUnauthorizedGroups(t *testing.T) {
	setupGatewayTestDB(t)

	user := createGatewayTestUser(t, "primary-group")
	privateGroupOwner := createGatewayTestUser(t, "owner-group")
	createPrivateGroupGrant(t, privateGroupOwner.Id, user.Id, "shared-private-group")

	manager := NewSessionManager()
	session := &GatewaySession{Token: &model.Token{UserId: user.Id}}

	_, err := registerWorkerSession(manager, session, &RegisterMessage{
		Namespace: "sync-models-namespace",
		Models:    []string{"model-a"},
	})
	require.NoError(t, err)

	err = syncWorkerModels(session, &ModelsSyncMessage{
		Group:  "shared-private-group",
		Models: []string{"model-b"},
	})
	require.NoError(t, err)
	require.Equal(t, "shared-private-group", session.Group)

	err = syncWorkerModels(session, &ModelsSyncMessage{
		Group:  "forbidden-group",
		Models: []string{"model-c"},
	})
	require.Error(t, err)

	var cpErr *controlPlaneError
	require.True(t, errors.As(err, &cpErr))
	require.Equal(t, "group_not_authorized", cpErr.code)
}
