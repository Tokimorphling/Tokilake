package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"one-api/common"
	"one-api/common/utils"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PrivateGroupStatusEnabled = 1

	PrivateGroupGrantRoleOwner  = "owner"
	PrivateGroupGrantRoleMember = "member"

	PrivateGroupGrantSourceCreate = "create"
	PrivateGroupGrantSourceInvite = "invite"
	PrivateGroupGrantSourceAdmin  = "admin"

	PrivateGroupInviteCodeStatusEnabled  = 1
	PrivateGroupInviteCodeStatusDisabled = 2
)

type PrivateGroup struct {
	Id          int            `json:"id"`
	OwnerUserId int            `json:"owner_user_id" gorm:"index"`
	GroupSlug   string         `json:"group_slug" gorm:"type:varchar(64);uniqueIndex"`
	Status      int            `json:"status" gorm:"default:1;index"`
	CreatedAt   int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (g *PrivateGroup) BeforeCreate(tx *gorm.DB) error {
	now := utils.GetTimestamp()
	g.CreatedAt = now
	g.UpdatedAt = now
	if g.Status == 0 {
		g.Status = PrivateGroupStatusEnabled
	}
	return nil
}

func (g *PrivateGroup) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = utils.GetTimestamp()
	return nil
}

type PrivateGroupGrant struct {
	Id        int    `json:"id"`
	GroupId   int    `json:"group_id" gorm:"index;uniqueIndex:idx_private_group_user,priority:1"`
	UserId    int    `json:"user_id" gorm:"index;uniqueIndex:idx_private_group_user,priority:2"`
	Role      string `json:"role" gorm:"type:varchar(16);default:'member'"`
	Source    string `json:"source" gorm:"type:varchar(16);default:'invite'"`
	SourceRef string `json:"source_ref" gorm:"type:varchar(128);default:''"`
	GrantedBy int    `json:"granted_by" gorm:"index"`
	ExpiresAt int64  `json:"expires_at" gorm:"bigint;default:0"`
	RevokedAt int64  `json:"revoked_at" gorm:"bigint;default:0;index"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (g *PrivateGroupGrant) BeforeCreate(tx *gorm.DB) error {
	now := utils.GetTimestamp()
	g.CreatedAt = now
	g.UpdatedAt = now
	return nil
}

func (g *PrivateGroupGrant) BeforeUpdate(tx *gorm.DB) error {
	g.UpdatedAt = utils.GetTimestamp()
	return nil
}

type PrivateGroupInviteCode struct {
	Id        int    `json:"id"`
	GroupId   int    `json:"group_id" gorm:"index"`
	CodeHash  string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	CreatedBy int    `json:"created_by" gorm:"index"`
	Status    int    `json:"status" gorm:"default:1;index"`
	MaxUses   int    `json:"max_uses" gorm:"default:1"`
	UsedCount int    `json:"used_count" gorm:"default:0"`
	ExpiresAt int64  `json:"expires_at" gorm:"bigint;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (c *PrivateGroupInviteCode) BeforeCreate(tx *gorm.DB) error {
	now := utils.GetTimestamp()
	c.CreatedAt = now
	c.UpdatedAt = now
	if c.Status == 0 {
		c.Status = PrivateGroupInviteCodeStatusEnabled
	}
	if c.MaxUses <= 0 {
		c.MaxUses = 1
	}
	return nil
}

func (c *PrivateGroupInviteCode) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = utils.GetTimestamp()
	return nil
}

type PrivateGroupSummary struct {
	Id              int    `json:"id"`
	GroupSlug       string `json:"group_slug"`
	Status          int    `json:"status"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	MemberCount     int64  `json:"member_count"`
	InviteCodeCount int64  `json:"invite_code_count"`
}

type PrivateGroupMember struct {
	GroupId     int    `json:"group_id"`
	UserId      int    `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Source      string `json:"source"`
	ExpiresAt   int64  `json:"expires_at"`
	RevokedAt   int64  `json:"revoked_at"`
	CreatedAt   int64  `json:"created_at"`
}

type UserPrivateGroupGrant struct {
	GroupId   int      `json:"group_id"`
	GroupSlug string   `json:"group_slug"`
	Role      string   `json:"role"`
	Source    string   `json:"source"`
	ExpiresAt int64    `json:"expires_at"`
	RevokedAt int64    `json:"revoked_at"`
	GrantedBy int      `json:"granted_by"`
	CreatedAt int64    `json:"created_at"`
	Models    []string `json:"models" gorm:"-"`
}

func NormalizePrivateGroupInviteCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func HashPrivateGroupInviteCode(code string) string {
	sum := sha256.Sum256([]byte(NormalizePrivateGroupInviteCode(code)))
	return hex.EncodeToString(sum[:])
}

func GetAllActivePrivateGroupSlugs() ([]string, error) {
	var groups []string
	err := DB.Model(&PrivateGroup{}).
		Where("status = ?", PrivateGroupStatusEnabled).
		Order("group_slug asc").
		Pluck("group_slug", &groups).Error
	return groups, err
}

func IsActivePrivateGroupSlug(groupSlug string) (bool, error) {
	if groupSlug == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&PrivateGroup{}).
		Where("group_slug = ?", groupSlug).
		Where("status = ?", PrivateGroupStatusEnabled).
		Count(&count).Error
	return count > 0, err
}

func GetPrivateGroupById(id int) (*PrivateGroup, error) {
	if id <= 0 {
		return nil, errors.New("invalid private group id")
	}
	var group PrivateGroup
	if err := DB.First(&group, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func GetPrivateGroupByIdAndOwner(id int, ownerUserId int) (*PrivateGroup, error) {
	if id <= 0 || ownerUserId <= 0 {
		return nil, errors.New("invalid private group lookup params")
	}
	var group PrivateGroup
	if err := DB.First(&group, "id = ? AND owner_user_id = ?", id, ownerUserId).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func PrivateGroupSlugExists(groupSlug string) (bool, error) {
	var count int64
	err := DB.Model(&PrivateGroup{}).Where("group_slug = ?", groupSlug).Count(&count).Error
	return count > 0, err
}

func GetUserPrivateGroupDescriptions(userId int) (map[string]string, error) {
	now := utils.GetTimestamp()
	rows := make([]struct {
		GroupSlug string `gorm:"column:group_slug"`
	}, 0)
	err := DB.Table("private_group_grants").
		Select("private_groups.group_slug").
		Joins("JOIN private_groups ON private_groups.id = private_group_grants.group_id").
		Where("private_group_grants.user_id = ?", userId).
		Where("private_group_grants.revoked_at = 0").
		Where("(private_group_grants.expires_at = 0 OR private_group_grants.expires_at >= ?)", now).
		Where("private_groups.deleted_at IS NULL").
		Where("private_groups.status = ?", PrivateGroupStatusEnabled).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	groups := make(map[string]string, len(rows))
	for _, row := range rows {
		groups[row.GroupSlug] = row.GroupSlug
	}
	return groups, nil
}

func GetUserPrivateGroupGrantDetails(userId int) ([]*UserPrivateGroupGrant, error) {
	now := utils.GetTimestamp()
	rows := make([]*UserPrivateGroupGrant, 0)
	err := DB.Table("private_group_grants").
		Select("private_group_grants.group_id, private_groups.group_slug, private_group_grants.role, private_group_grants.source, private_group_grants.expires_at, private_group_grants.revoked_at, private_group_grants.granted_by, private_group_grants.created_at").
		Joins("JOIN private_groups ON private_groups.id = private_group_grants.group_id").
		Where("private_group_grants.user_id = ?", userId).
		Where("private_group_grants.revoked_at = 0").
		Where("(private_group_grants.expires_at = 0 OR private_group_grants.expires_at >= ?)", now).
		Where("private_groups.deleted_at IS NULL").
		Where("private_groups.status = ?", PrivateGroupStatusEnabled).
		Order("private_group_grants.created_at DESC").
		Scan(&rows).Error
	return rows, err
}

func GetOwnedPrivateGroups(ownerUserId int) ([]*PrivateGroupSummary, error) {
	var groups []*PrivateGroup
	if err := DB.Where("owner_user_id = ?", ownerUserId).Order("id DESC").Find(&groups).Error; err != nil {
		return nil, err
	}
	summaries := make([]*PrivateGroupSummary, 0, len(groups))
	for _, group := range groups {
		memberCount, err := CountActivePrivateGroupMembers(group.Id)
		if err != nil {
			return nil, err
		}
		inviteCount, err := CountActivePrivateGroupInviteCodes(group.Id)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, &PrivateGroupSummary{
			Id:              group.Id,
			GroupSlug:       group.GroupSlug,
			Status:          group.Status,
			CreatedAt:       group.CreatedAt,
			UpdatedAt:       group.UpdatedAt,
			MemberCount:     memberCount,
			InviteCodeCount: inviteCount,
		})
	}
	return summaries, nil
}

func GetAllPrivateGroups() ([]*PrivateGroupSummary, error) {
	var groups []*PrivateGroup
	if err := DB.Order("id DESC").Find(&groups).Error; err != nil {
		return nil, err
	}
	summaries := make([]*PrivateGroupSummary, 0, len(groups))
	for _, group := range groups {
		memberCount, err := CountActivePrivateGroupMembers(group.Id)
		if err != nil {
			return nil, err
		}
		inviteCount, err := CountActivePrivateGroupInviteCodes(group.Id)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, &PrivateGroupSummary{
			Id:              group.Id,
			GroupSlug:       group.GroupSlug,
			Status:          group.Status,
			CreatedAt:       group.CreatedAt,
			UpdatedAt:       group.UpdatedAt,
			MemberCount:     memberCount,
			InviteCodeCount: inviteCount,
		})
	}
	return summaries, nil
}

func CountActivePrivateGroupMembers(groupId int) (int64, error) {
	var count int64
	now := utils.GetTimestamp()
	err := DB.Model(&PrivateGroupGrant{}).
		Where("group_id = ?", groupId).
		Where("revoked_at = 0").
		Where("(expires_at = 0 OR expires_at >= ?)", now).
		Count(&count).Error
	return count, err
}

func CountActivePrivateGroupInviteCodes(groupId int) (int64, error) {
	var count int64
	now := utils.GetTimestamp()
	err := DB.Model(&PrivateGroupInviteCode{}).
		Where("group_id = ?", groupId).
		Where("status = ?", PrivateGroupInviteCodeStatusEnabled).
		Where("(expires_at = 0 OR expires_at >= ?)", now).
		Count(&count).Error
	return count, err
}

func CreatePrivateGroup(ownerUserId int, groupSlug string) (*PrivateGroup, *PrivateGroupGrant, error) {
	group := &PrivateGroup{
		OwnerUserId: ownerUserId,
		GroupSlug:   groupSlug,
		Status:      PrivateGroupStatusEnabled,
	}
	var ownerGrant *PrivateGroupGrant
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		grant, err := upsertPrivateGroupGrantTx(tx, group.Id, ownerUserId, PrivateGroupGrantRoleOwner, PrivateGroupGrantSourceCreate, fmt.Sprintf("group:%d", group.Id), ownerUserId, 0)
		if err != nil {
			return err
		}
		ownerGrant = grant
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return group, ownerGrant, nil
}

func UpdatePrivateGroupSlug(group *PrivateGroup, newSlug string) error {
	if group == nil {
		return errors.New("private group is nil")
	}
	group.GroupSlug = newSlug
	return DB.Save(group).Error
}

func DeletePrivateGroup(group *PrivateGroup) error {
	if group == nil {
		return errors.New("private group is nil")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		now := utils.GetTimestamp()
		if err := tx.Model(&PrivateGroupGrant{}).
			Where("group_id = ?", group.Id).
			Where("revoked_at = 0").
			Updates(map[string]interface{}{
				"revoked_at": now,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&PrivateGroupInviteCode{}).
			Where("group_id = ?", group.Id).
			Updates(map[string]interface{}{
				"status":     PrivateGroupInviteCodeStatusDisabled,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Delete(group).Error
	})
}

func CreatePrivateGroupInviteCode(groupId int, createdBy int, codeHash string, maxUses int, expiresAt int64) (*PrivateGroupInviteCode, error) {
	inviteCode := &PrivateGroupInviteCode{
		GroupId:   groupId,
		CodeHash:  codeHash,
		CreatedBy: createdBy,
		Status:    PrivateGroupInviteCodeStatusEnabled,
		MaxUses:   maxUses,
		ExpiresAt: expiresAt,
	}
	if err := DB.Create(inviteCode).Error; err != nil {
		return nil, err
	}
	return inviteCode, nil
}

func GetPrivateGroupInviteCodes(groupId int) ([]*PrivateGroupInviteCode, error) {
	codes := make([]*PrivateGroupInviteCode, 0)
	err := DB.Where("group_id = ?", groupId).Order("id DESC").Find(&codes).Error
	return codes, err
}

func GetPrivateGroupInviteCodeById(id int) (*PrivateGroupInviteCode, error) {
	if id <= 0 {
		return nil, errors.New("invalid private group invite code id")
	}
	var code PrivateGroupInviteCode
	if err := DB.First(&code, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &code, nil
}

func UpdatePrivateGroupInviteCodeStatus(code *PrivateGroupInviteCode, status int) error {
	if code == nil {
		return errors.New("private group invite code is nil")
	}
	code.Status = status
	return DB.Save(code).Error
}

func RedeemPrivateGroupInviteCode(code string, userId int) (*PrivateGroup, *PrivateGroupGrant, *PrivateGroupInviteCode, bool, error) {
	if code == "" {
		return nil, nil, nil, false, errors.New("邀请码不能为空")
	}
	if userId <= 0 {
		return nil, nil, nil, false, errors.New("无效的用户")
	}
	codeHash := HashPrivateGroupInviteCode(code)
	group := &PrivateGroup{}
	inviteCode := &PrivateGroupInviteCode{}
	var grant *PrivateGroupGrant
	var created bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("code_hash = ?", codeHash).
			First(inviteCode).Error; err != nil {
			return errors.New("无效的邀请码")
		}
		if inviteCode.Status != PrivateGroupInviteCodeStatusEnabled {
			return errors.New("邀请码不可用")
		}
		now := utils.GetTimestamp()
		if inviteCode.ExpiresAt != 0 && inviteCode.ExpiresAt < now {
			return errors.New("邀请码已过期")
		}
		if inviteCode.MaxUses > 0 && inviteCode.UsedCount >= inviteCode.MaxUses {
			return errors.New("邀请码已达到使用上限")
		}
		if err := tx.First(group, "id = ?", inviteCode.GroupId).Error; err != nil {
			return errors.New("分组不存在")
		}
		if group.Status != PrivateGroupStatusEnabled {
			return errors.New("分组不可用")
		}
		existing, err := getPrivateGroupGrantTx(tx, inviteCode.GroupId, userId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && existing != nil && existing.RevokedAt == 0 && (existing.ExpiresAt == 0 || existing.ExpiresAt >= now) {
			grant = existing
			created = false
			return nil
		}
		grant, err = upsertPrivateGroupGrantTx(tx, inviteCode.GroupId, userId, PrivateGroupGrantRoleMember, PrivateGroupGrantSourceInvite, fmt.Sprintf("invite_code:%d", inviteCode.Id), inviteCode.CreatedBy, 0)
		if err != nil {
			return err
		}
		inviteCode.UsedCount++
		if inviteCode.MaxUses > 0 && inviteCode.UsedCount >= inviteCode.MaxUses {
			inviteCode.Status = PrivateGroupInviteCodeStatusDisabled
		}
		if err := tx.Save(inviteCode).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, nil, nil, false, err
	}
	return group, grant, inviteCode, created, nil
}

func GetPrivateGroupMembers(groupId int) ([]*PrivateGroupMember, error) {
	now := utils.GetTimestamp()
	members := make([]*PrivateGroupMember, 0)
	err := DB.Table("private_group_grants").
		Select("private_group_grants.group_id, private_group_grants.user_id, users.username, users.display_name, private_group_grants.role, private_group_grants.source, private_group_grants.expires_at, private_group_grants.revoked_at, private_group_grants.created_at").
		Joins("JOIN users ON users.id = private_group_grants.user_id").
		Where("private_group_grants.group_id = ?", groupId).
		Where("private_group_grants.revoked_at = 0").
		Where("(private_group_grants.expires_at = 0 OR private_group_grants.expires_at >= ?)", now).
		Order("private_group_grants.id ASC").
		Scan(&members).Error
	return members, err
}

func RevokePrivateGroupGrant(groupId int, userId int) error {
	now := utils.GetTimestamp()
	return DB.Model(&PrivateGroupGrant{}).
		Where("group_id = ? AND user_id = ?", groupId, userId).
		Where("revoked_at = 0").
		Updates(map[string]interface{}{
			"revoked_at": now,
			"updated_at": now,
		}).Error
}

func GrantPrivateGroupAccess(groupId int, userId int, role string, source string, sourceRef string, grantedBy int, expiresAt int64) (*PrivateGroupGrant, error) {
	var grant *PrivateGroupGrant
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		grant, err = upsertPrivateGroupGrantTx(tx, groupId, userId, role, source, sourceRef, grantedBy, expiresAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	return grant, nil
}

func GetPrivateGroupGrant(groupId int, userId int) (*PrivateGroupGrant, error) {
	return getPrivateGroupGrantTx(DB, groupId, userId)
}

func getPrivateGroupGrantTx(tx *gorm.DB, groupId int, userId int) (*PrivateGroupGrant, error) {
	var grant PrivateGroupGrant
	if err := tx.Where("group_id = ? AND user_id = ?", groupId, userId).First(&grant).Error; err != nil {
		return nil, err
	}
	return &grant, nil
}

func upsertPrivateGroupGrantTx(tx *gorm.DB, groupId int, userId int, role string, source string, sourceRef string, grantedBy int, expiresAt int64) (*PrivateGroupGrant, error) {
	if tx == nil {
		return nil, errors.New("transaction is nil")
	}
	now := utils.GetTimestamp()
	grant, err := getPrivateGroupGrantTx(tx, groupId, userId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		grant = &PrivateGroupGrant{
			GroupId:   groupId,
			UserId:    userId,
			Role:      role,
			Source:    source,
			SourceRef: sourceRef,
			GrantedBy: grantedBy,
			ExpiresAt: expiresAt,
			RevokedAt: 0,
		}
		if err := tx.Create(grant).Error; err != nil {
			return nil, err
		}
		return grant, nil
	}
	grant.Role = role
	grant.Source = source
	grant.SourceRef = sourceRef
	grant.GrantedBy = grantedBy
	grant.ExpiresAt = expiresAt
	grant.RevokedAt = 0
	grant.UpdatedAt = now
	if err := tx.Save(grant).Error; err != nil {
		return nil, err
	}
	return grant, nil
}

func HasPrivateGroupTokenReference(groupSlug string) (bool, error) {
	if groupSlug == "" {
		return false, nil
	}
	var count int64
	err := DB.Model(&Token{}).Where(quotePostgresField("group")+" = ? OR backup_group = ?", groupSlug, groupSlug).Count(&count).Error
	return count > 0, err
}

func HasPrivateGroupChannelReference(groupSlug string) (bool, error) {
	if groupSlug == "" {
		return false, nil
	}
	groupCol := quotePostgresField("group")
	condition := "(',' || " + groupCol + " || ',') LIKE ?"
	if !common.UsingPostgreSQL && !common.UsingSQLite {
		condition = "CONCAT(',', " + groupCol + ", ',') LIKE ?"
	}
	var count int64
	err := DB.Model(&Channel{}).Where(condition, "%,"+groupSlug+",%").Count(&count).Error
	return count > 0, err
}
