package service

import (
	"errors"
	"fmt"
	"one-api/common/utils"
	"one-api/model"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
)

var privateGroupSlugPattern = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)

func BuildPrivateGroupUserGroup(groupSlug string) *model.UserGroup {
	enabled := true
	return &model.UserGroup{
		Symbol:  groupSlug,
		Name:    groupSlug,
		Ratio:   1,
		APIRate: 600,
		Public:  false,
		Enable:  &enabled,
	}
}

func ValidatePrivateGroupSlug(groupSlug string, excludeGroupId int) (string, error) {
	groupSlug = strings.TrimSpace(strings.ToLower(groupSlug))
	if !privateGroupSlugPattern.MatchString(groupSlug) {
		return "", errors.New("分组名只能包含小写字母、数字和连字符，长度需在 3-64 之间")
	}
	if groupSlug == "auto" || groupSlug == "default" {
		return "", errors.New("该分组名为保留分组")
	}
	exists, err := model.UserGroupSymbolExists(groupSlug)
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.New("该分组名已被系统分组占用")
	}
	privateExists, err := model.PrivateGroupSlugExists(groupSlug)
	if err != nil {
		return "", err
	}
	if privateExists {
		if excludeGroupId > 0 {
			group, err := model.GetPrivateGroupById(excludeGroupId)
			if err == nil && group != nil && group.GroupSlug == groupSlug {
				return groupSlug, nil
			}
		}
		return "", errors.New("该分组名已存在")
	}
	return groupSlug, nil
}

func GeneratePrivateGroupInviteCode() string {
	parts := []string{
		strings.ToUpper(utils.GetRandomString(4)),
		strings.ToUpper(utils.GetRandomString(4)),
		strings.ToUpper(utils.GetRandomString(4)),
	}
	return strings.Join(parts, "-")
}

func CreatePrivateGroup(ownerUserId int, groupSlug string) (*model.PrivateGroup, error) {
	validatedSlug, err := ValidatePrivateGroupSlug(groupSlug, 0)
	if err != nil {
		return nil, err
	}
	group, _, err := model.CreatePrivateGroup(ownerUserId, validatedSlug)
	if err != nil {
		return nil, err
	}
	model.RecordLog(ownerUserId, model.LogTypeSystem, fmt.Sprintf("创建私有分组 %s", group.GroupSlug))
	return group, nil
}

func RenamePrivateGroup(ownerUserId int, groupId int, newSlug string, isAdmin bool) (*model.PrivateGroup, error) {
	var (
		group *model.PrivateGroup
		err   error
	)
	if isAdmin {
		group, err = model.GetPrivateGroupById(groupId)
	} else {
		group, err = model.GetPrivateGroupByIdAndOwner(groupId, ownerUserId)
	}
	if err != nil {
		return nil, err
	}
	validatedSlug, err := ValidatePrivateGroupSlug(newSlug, group.Id)
	if err != nil {
		return nil, err
	}
	hasTokenRef, err := model.HasPrivateGroupTokenReference(group.GroupSlug)
	if err != nil {
		return nil, err
	}
	if hasTokenRef {
		return nil, errors.New("该分组仍被令牌引用，请先解绑相关令牌")
	}
	hasChannelRef, err := model.HasPrivateGroupChannelReference(group.GroupSlug)
	if err != nil {
		return nil, err
	}
	if hasChannelRef {
		return nil, errors.New("该分组仍被渠道引用，请先调整 TOKIAME_GROUP 或渠道分组")
	}
	oldSlug := group.GroupSlug
	if err := model.UpdatePrivateGroupSlug(group, validatedSlug); err != nil {
		return nil, err
	}
	model.RecordLog(ownerUserId, model.LogTypeSystem, fmt.Sprintf("将私有分组 %s 重命名为 %s", oldSlug, validatedSlug))
	return group, nil
}

func DeletePrivateGroup(ownerUserId int, groupId int, isAdmin bool) error {
	var (
		group *model.PrivateGroup
		err   error
	)
	if isAdmin {
		group, err = model.GetPrivateGroupById(groupId)
	} else {
		group, err = model.GetPrivateGroupByIdAndOwner(groupId, ownerUserId)
	}
	if err != nil {
		return err
	}
	hasTokenRef, err := model.HasPrivateGroupTokenReference(group.GroupSlug)
	if err != nil {
		return err
	}
	if hasTokenRef {
		return errors.New("该分组仍被令牌引用，请先解绑相关令牌")
	}
	hasChannelRef, err := model.HasPrivateGroupChannelReference(group.GroupSlug)
	if err != nil {
		return err
	}
	if hasChannelRef {
		return errors.New("该分组仍被渠道引用，请先调整 TOKIAME_GROUP 或渠道分组")
	}
	if err := model.DeletePrivateGroup(group); err != nil {
		return err
	}
	model.RecordLog(ownerUserId, model.LogTypeSystem, fmt.Sprintf("删除私有分组 %s", group.GroupSlug))
	return nil
}

func ListOwnedPrivateGroups(ownerUserId int) ([]*model.PrivateGroupSummary, error) {
	return model.GetOwnedPrivateGroups(ownerUserId)
}

func ListAllPrivateGroups() ([]*model.PrivateGroupSummary, error) {
	return model.GetAllPrivateGroups()
}

func CreatePrivateGroupInviteCode(ownerUserId int, groupId int, maxUses int, expiresAt int64, isAdmin bool) (*model.PrivateGroupInviteCode, string, error) {
	var (
		group *model.PrivateGroup
		err   error
	)
	if maxUses <= 0 {
		return nil, "", errors.New("邀请码使用次数必须大于 0")
	}
	if expiresAt != 0 && expiresAt < utils.GetTimestamp() {
		return nil, "", errors.New("邀请码过期时间无效")
	}
	if isAdmin {
		group, err = model.GetPrivateGroupById(groupId)
	} else {
		group, err = model.GetPrivateGroupByIdAndOwner(groupId, ownerUserId)
	}
	if err != nil {
		return nil, "", err
	}
	code := GeneratePrivateGroupInviteCode()
	inviteCode, err := model.CreatePrivateGroupInviteCode(group.Id, ownerUserId, model.HashPrivateGroupInviteCode(code), maxUses, expiresAt)
	if err != nil {
		return nil, "", err
	}
	return inviteCode, code, nil
}

func UpdatePrivateGroupInviteCodeStatus(ownerUserId int, groupId int, codeId int, status int, isAdmin bool) (*model.PrivateGroupInviteCode, error) {
	var (
		group *model.PrivateGroup
		err   error
	)
	if status != model.PrivateGroupInviteCodeStatusEnabled && status != model.PrivateGroupInviteCodeStatusDisabled {
		return nil, errors.New("无效的邀请码状态")
	}
	if isAdmin {
		group, err = model.GetPrivateGroupById(groupId)
	} else {
		group, err = model.GetPrivateGroupByIdAndOwner(groupId, ownerUserId)
	}
	if err != nil {
		return nil, err
	}
	code, err := model.GetPrivateGroupInviteCodeById(codeId)
	if err != nil {
		return nil, err
	}
	if code.GroupId != group.Id {
		return nil, errors.New("邀请码不属于该分组")
	}
	if err := model.UpdatePrivateGroupInviteCodeStatus(code, status); err != nil {
		return nil, err
	}
	return code, nil
}

func ListPrivateGroupInviteCodes(ownerUserId int, groupId int, isAdmin bool) ([]*model.PrivateGroupInviteCode, error) {
	if isAdmin {
		if _, err := model.GetPrivateGroupById(groupId); err != nil {
			return nil, err
		}
	} else {
		if _, err := model.GetPrivateGroupByIdAndOwner(groupId, ownerUserId); err != nil {
			return nil, err
		}
	}
	return model.GetPrivateGroupInviteCodes(groupId)
}

func RedeemPrivateGroupInviteCode(code string, userId int) (*model.PrivateGroup, *model.PrivateGroupGrant, bool, error) {
	group, grant, _, created, err := model.RedeemPrivateGroupInviteCode(code, userId)
	if err != nil {
		return nil, nil, false, err
	}
	if created {
		model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("通过邀请码加入私有分组 %s", group.GroupSlug))
	}
	return group, grant, created, nil
}

func ListPrivateGroupMembers(ownerUserId int, groupId int, isAdmin bool) ([]*model.PrivateGroupMember, error) {
	if isAdmin {
		if _, err := model.GetPrivateGroupById(groupId); err != nil {
			return nil, err
		}
	} else {
		if _, err := model.GetPrivateGroupByIdAndOwner(groupId, ownerUserId); err != nil {
			return nil, err
		}
	}
	return model.GetPrivateGroupMembers(groupId)
}

func RevokePrivateGroupMember(actorUserId int, groupId int, targetUserId int, isAdmin bool) error {
	var (
		group *model.PrivateGroup
		err   error
	)
	if isAdmin {
		group, err = model.GetPrivateGroupById(groupId)
	} else {
		group, err = model.GetPrivateGroupByIdAndOwner(groupId, actorUserId)
	}
	if err != nil {
		return err
	}
	grant, err := model.GetPrivateGroupGrant(groupId, targetUserId)
	if err != nil {
		return err
	}
	if grant.Role == model.PrivateGroupGrantRoleOwner {
		return errors.New("不能撤销组主权限")
	}
	if grant.RevokedAt != 0 {
		return errors.New("成员权限已被撤销")
	}
	if err := model.RevokePrivateGroupGrant(groupId, targetUserId); err != nil {
		return err
	}
	model.RecordLog(targetUserId, model.LogTypeSystem, fmt.Sprintf("私有分组 %s 权限已被撤销", group.GroupSlug))
	return nil
}

func GrantPrivateGroupMemberByAdmin(actorUserId int, targetUserId int, groupId int, expiresAt int64) (*model.PrivateGroupGrant, error) {
	if expiresAt != 0 && expiresAt < utils.GetTimestamp() {
		return nil, errors.New("成员过期时间无效")
	}
	group, err := model.GetPrivateGroupById(groupId)
	if err != nil {
		return nil, err
	}
	if group.Status != model.PrivateGroupStatusEnabled {
		return nil, errors.New("分组不可用")
	}
	return model.GrantPrivateGroupAccess(groupId, targetUserId, model.PrivateGroupGrantRoleMember, model.PrivateGroupGrantSourceAdmin, fmt.Sprintf("admin:%d", actorUserId), actorUserId, expiresAt)
}

func ListUserPrivateGroupGrants(userId int) ([]*model.UserPrivateGroupGrant, error) {
	grants, err := model.GetUserPrivateGroupGrantDetails(userId)
	if err != nil {
		return nil, err
	}
	for _, grant := range grants {
		models, err := model.ChannelGroup.GetGroupModels(grant.GroupSlug)
		if err != nil {
			grant.Models = []string{}
			continue
		}
		sort.Strings(models)
		grant.Models = models
	}
	return grants, nil
}

func ListJoinedPrivateGroups(userId int) ([]*model.UserPrivateGroupGrant, error) {
	grants, err := ListUserPrivateGroupGrants(userId)
	if err != nil {
		return nil, err
	}
	joined := make([]*model.UserPrivateGroupGrant, 0, len(grants))
	for _, grant := range grants {
		if grant.Role == model.PrivateGroupGrantRoleOwner {
			continue
		}
		joined = append(joined, grant)
	}
	return joined, nil
}

func GetUserUsableGroupsForUser(userId int, userGroup string) (map[string]*model.UserGroup, error) {
	groups := make(map[string]*model.UserGroup)
	for symbol, group := range model.GlobalUserGroupRatio.GetAll() {
		if group.Public || symbol == userGroup {
			groupCopy := *group
			groups[symbol] = &groupCopy
		}
	}
	if userId <= 0 {
		return groups, nil
	}
	privateGroups, err := model.GetUserPrivateGroupDescriptions(userId)
	if err != nil {
		return nil, err
	}
	for groupSlug := range privateGroups {
		groups[groupSlug] = BuildPrivateGroupUserGroup(groupSlug)
	}
	return groups, nil
}

func GroupInUserUsableGroupsForUser(userId int, userGroup string, groupName string) (bool, error) {
	groups, err := GetUserUsableGroupsForUser(userId, userGroup)
	if err != nil {
		return false, err
	}
	_, ok := groups[groupName]
	return ok, nil
}

func LoadPrivateGroupForAccess(groupId int) (*model.PrivateGroup, error) {
	group, err := model.GetPrivateGroupById(groupId)
	if err != nil {
		return nil, err
	}
	if group.Status != model.PrivateGroupStatusEnabled {
		return nil, gorm.ErrRecordNotFound
	}
	return group, nil
}
