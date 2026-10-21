// Package migrations 提供随二进制发布的有序 SQL 迁移。
package migrations

import "embed"

// Files 中的已发布迁移不可修改；后续升级必须新增连续编号文件。
//
//go:embed *.sql
var Files embed.FS
