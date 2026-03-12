package model

import (
	"fmt"
	"time"

	"one-api/common"
)

const (
	TokilakeWorkerNodeStatusOnline  = 1
	TokilakeWorkerNodeStatusBusy    = 2
	TokilakeWorkerNodeStatusOffline = 3
)

type TokilakeWorkerNode struct {
	Id            int    `json:"id"`
	ProviderId    int    `json:"provider_id" gorm:"index"`
	Namespace     string `json:"namespace" gorm:"type:varchar(255);uniqueIndex"`
	NodeName      string `json:"node_name" gorm:"type:varchar(255)"`
	Status        int    `json:"status" gorm:"default:3;index"`
	Models        string `json:"models" gorm:"type:text"`
	HardwareInfo  string `json:"hardware_info" gorm:"type:text"`
	LastHeartbeat int64  `json:"last_heartbeat" gorm:"bigint;index"`
	ChannelId     int    `json:"channel_id" gorm:"uniqueIndex"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`
}

func (node *TokilakeWorkerNode) SetModels(models []string) {
	data, err := common.Marshal(models)
	if err != nil {
		return
	}
	node.Models = string(data)
}

func (node *TokilakeWorkerNode) GetModels() []string {
	if node.Models == "" {
		return nil
	}
	var models []string
	if err := common.UnmarshalJsonStr(node.Models, &models); err != nil {
		return nil
	}
	return models
}

func (node *TokilakeWorkerNode) SetHardwareInfo(info map[string]any) {
	data, err := common.Marshal(info)
	if err != nil {
		return
	}
	node.HardwareInfo = string(data)
}

func (node *TokilakeWorkerNode) GetHardwareInfo() map[string]any {
	if node.HardwareInfo == "" {
		return nil
	}
	var info map[string]any
	if err := common.UnmarshalJsonStr(node.HardwareInfo, &info); err != nil {
		return nil
	}
	return info
}

func (node *TokilakeWorkerNode) beforeCreate() {
	now := time.Now().Unix()
	node.CreatedAt = now
	node.UpdatedAt = now
	if node.LastHeartbeat == 0 {
		node.LastHeartbeat = now
	}
}

func (node *TokilakeWorkerNode) Insert() error {
	node.beforeCreate()
	return DB.Create(node).Error
}

func (node *TokilakeWorkerNode) Update() error {
	node.UpdatedAt = time.Now().Unix()
	return DB.Model(node).Updates(node).Error
}

func (node *TokilakeWorkerNode) TouchHeartbeat(status int) error {
	updates := map[string]any{
		"last_heartbeat": time.Now().Unix(),
		"updated_at":     time.Now().Unix(),
	}
	if status > 0 {
		updates["status"] = status
	}
	return DB.Model(node).Updates(updates).Error
}

func GetTokilakeWorkerNodeByNamespace(namespace string) (*TokilakeWorkerNode, error) {
	node := &TokilakeWorkerNode{}
	if err := DB.Where("namespace = ?", namespace).First(node).Error; err != nil {
		return nil, err
	}
	return node, nil
}

func GetTokilakeWorkerNodeByChannelId(channelId int) (*TokilakeWorkerNode, error) {
	node := &TokilakeWorkerNode{}
	if err := DB.Where("channel_id = ?", channelId).First(node).Error; err != nil {
		return nil, err
	}
	return node, nil
}

func GetOnlineTokilakeWorkerNodes() ([]*TokilakeWorkerNode, error) {
	var nodes []*TokilakeWorkerNode
	err := DB.Where("status = ?", TokilakeWorkerNodeStatusOnline).Find(&nodes).Error
	return nodes, err
}

func MarkOfflineTokilakeWorkerNodesBefore(cutoff int64) error {
	return DB.Model(&TokilakeWorkerNode{}).
		Where("last_heartbeat < ? AND status != ?", cutoff, TokilakeWorkerNodeStatusOffline).
		Updates(map[string]any{
			"status":     TokilakeWorkerNodeStatusOffline,
			"updated_at": time.Now().Unix(),
		}).Error
}

func (node *TokilakeWorkerNode) String() string {
	return fmt.Sprintf("TokilakeWorkerNode(namespace=%s, channel_id=%d)", node.Namespace, node.ChannelId)
}
