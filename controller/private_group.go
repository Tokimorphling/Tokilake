package controller

import (
	"net/http"
	"one-api/common"
	"one-api/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type privateGroupCreateRequest struct {
	GroupSlug string `json:"group_slug"`
}

type privateGroupUpdateRequest struct {
	GroupSlug string `json:"group_slug"`
}

type privateGroupInviteCodeCreateRequest struct {
	MaxUses   int   `json:"max_uses"`
	ExpiresAt int64 `json:"expires_at"`
}

type privateGroupInviteCodeUpdateRequest struct {
	Status int `json:"status"`
}

type privateGroupRedeemRequest struct {
	Code string `json:"code"`
}

type adminPrivateGroupGrantRequest struct {
	GroupId   int   `json:"group_id"`
	ExpiresAt int64 `json:"expires_at"`
}

func ListPrivateGroups(c *gin.Context) {
	groups, err := service.ListOwnedPrivateGroups(c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": groups})
}

func ListJoinedPrivateGroups(c *gin.Context) {
	groups, err := service.ListJoinedPrivateGroups(c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": groups})
}

func CreatePrivateGroup(c *gin.Context) {
	var req privateGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	group, err := service.CreatePrivateGroup(c.GetInt("id"), req.GroupSlug)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": group})
}

func UpdatePrivateGroup(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req privateGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	group, err := service.RenamePrivateGroup(c.GetInt("id"), groupId, req.GroupSlug, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": group})
}

func DeletePrivateGroup(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := service.DeletePrivateGroup(c.GetInt("id"), groupId, false); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func ListPrivateGroupInviteCodes(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	codes, err := service.ListPrivateGroupInviteCodes(c.GetInt("id"), groupId, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": codes})
}

func CreatePrivateGroupInviteCode(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req privateGroupInviteCodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	record, code, err := service.CreatePrivateGroupInviteCode(c.GetInt("id"), groupId, req.MaxUses, req.ExpiresAt, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"invite_code": code,
			"record":      record,
		},
	})
}

func UpdatePrivateGroupInviteCode(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	codeId, err := strconv.Atoi(c.Param("code_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req privateGroupInviteCodeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	code, err := service.UpdatePrivateGroupInviteCodeStatus(c.GetInt("id"), groupId, codeId, req.Status, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": code})
}

func ListPrivateGroupMembers(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	members, err := service.ListPrivateGroupMembers(c.GetInt("id"), groupId, false)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": members})
}

func DeletePrivateGroupMember(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := service.RevokePrivateGroupMember(c.GetInt("id"), groupId, userId, false); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func RedeemPrivateGroupInviteCode(c *gin.Context) {
	var req privateGroupRedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	group, grant, created, err := service.RedeemPrivateGroupInviteCode(req.Code, c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"group":   group,
			"grant":   grant,
			"created": created,
		},
	})
}

func AdminListPrivateGroups(c *gin.Context) {
	groups, err := service.ListAllPrivateGroups()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": groups})
}

func AdminUpdatePrivateGroup(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req privateGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	group, err := service.RenamePrivateGroup(c.GetInt("id"), groupId, req.GroupSlug, true)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": group})
}

func AdminDeletePrivateGroup(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := service.DeletePrivateGroup(c.GetInt("id"), groupId, true); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func AdminListPrivateGroupMembers(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	members, err := service.ListPrivateGroupMembers(c.GetInt("id"), groupId, true)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": members})
}

func AdminCreatePrivateGroupInviteCode(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req privateGroupInviteCodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	record, code, err := service.CreatePrivateGroupInviteCode(c.GetInt("id"), groupId, req.MaxUses, req.ExpiresAt, true)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"invite_code": code,
			"record":      record,
		},
	})
}

func AdminUpdatePrivateGroupInviteCode(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	codeId, err := strconv.Atoi(c.Param("code_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req privateGroupInviteCodeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	code, err := service.UpdatePrivateGroupInviteCodeStatus(c.GetInt("id"), groupId, codeId, req.Status, true)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": code})
}

func AdminListPrivateGroupInviteCodes(c *gin.Context) {
	groupId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	codes, err := service.ListPrivateGroupInviteCodes(c.GetInt("id"), groupId, true)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": codes})
}

func AdminListUserPrivateGroups(c *gin.Context) {
	targetUserId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	grants, err := service.ListUserPrivateGroupGrants(targetUserId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": grants})
}

func AdminGrantPrivateGroup(c *gin.Context) {
	targetUserId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req adminPrivateGroupGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	grant, err := service.GrantPrivateGroupMemberByAdmin(c.GetInt("id"), targetUserId, req.GroupId, req.ExpiresAt)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": grant})
}

func AdminRevokePrivateGroup(c *gin.Context) {
	targetUserId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	groupId, err := strconv.Atoi(c.Param("group_id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := service.RevokePrivateGroupMember(c.GetInt("id"), groupId, targetUserId, true); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
