package controller

import (
	"net/http"
	"one-api/model"
	"one-api/service"
	"sort"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupSet := make(map[string]struct{})
	for symbol := range model.GlobalUserGroupRatio.GetAll() {
		groupSet[symbol] = struct{}{}
	}
	privateGroups, err := model.GetAllActivePrivateGroupSlugs()
	if err == nil {
		for _, groupSlug := range privateGroups {
			groupSet[groupSlug] = struct{}{}
		}
	}
	groupNames := make([]string, 0, len(groupSet))
	for symbol := range groupSet {
		groupNames = append(groupNames, symbol)
	}
	sort.Strings(groupNames)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetUserGroupRatio(c *gin.Context) {
	userId := c.GetInt("id")
	userSymbol := ""

	if userId > 0 {
		userSymbol, _ = model.CacheGetUserGroup(userId)
	}

	userGroups, err := service.GetUserUsableGroupsForUser(userId, userSymbol)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userGroups,
	})
}
