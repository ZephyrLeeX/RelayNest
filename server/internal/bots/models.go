// Package bots 提供 Bot 元数据仓储及与管理员会话独立的凭据授权。
package bots

import (
	"errors"
	"time"
)

var (
	ErrInvalid      = errors.New("invalid bot input")
	ErrNotFound     = errors.New("bot not found")
	ErrConflict     = errors.New("bot version conflict")
	ErrUnauthorized = errors.New("invalid bot credential")
	ErrForbidden    = errors.New("bot operation forbidden")
)

type Capabilities struct {
	Send    bool `json:"send"`
	Receive bool `json:"receive"`
}
type CredentialMetadata struct {
	CredentialID string `json:"credential_id"`
	Generation   int64  `json:"generation"`
}
type Credentials struct {
	APIToken   CredentialMetadata `json:"api_token"`
	WebhookKey CredentialMetadata `json:"webhook_key"`
}
type Connection struct {
	State      string     `json:"state"`
	LastPollAt *time.Time `json:"last_poll_at"`
}
type Bot struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Enabled      bool         `json:"enabled"`
	Capabilities Capabilities `json:"capabilities"`
	Version      int64        `json:"version"`
	Credentials  Credentials  `json:"credentials"`
	Connection   Connection   `json:"connection"`
}
type CreateRequest struct {
	Name         string
	Enabled      bool
	Capabilities Capabilities
}
type PatchRequest struct {
	ExpectedVersion int64
	Name            *string
	Enabled         *bool
	Capabilities    *Capabilities
}

// DeleteRequest 仅在调用方完成二次确认后提交；取消确认不调用仓储。
type DeleteRequest struct {
	ConfirmName     string
	ExpectedVersion int64
}
type IssuedAPIToken struct {
	Credential   CredentialMetadata `json:"credential"`
	Token        string             `json:"token"`
	Capabilities Capabilities       `json:"capabilities"`
}
type IssuedWebhookKey struct {
	Credential CredentialMetadata `json:"credential"`
	Key        string             `json:"key"`
}

// Created 仅由成功创建返回。普通 Bot、查询和编辑模型不包含明文或摘要。
type Created struct {
	Bot        Bot              `json:"bot"`
	APIToken   IssuedAPIToken   `json:"api_token"`
	WebhookKey IssuedWebhookKey `json:"webhook_key"`
}
