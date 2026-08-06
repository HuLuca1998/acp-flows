package store

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// translate 把 GORM 的错误翻译成领域错误。
//
// ★ GORM 的错误类型不出本包。app 层用 errors.Is(err, model.ErrNotFound) 判断，
// 不许 import gorm——由 depguard 强制。
//
// op 用于给错误加上下文（"find work work-08"），线上排查靠它。
func translate(op string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return fmt.Errorf("%s: %w", op, model.ErrNotFound)
	case isUniqueViolation(err):
		return fmt.Errorf("%s: %w", op, model.ErrAlreadyExists)
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}

// isUniqueViolation 判断是否唯一约束冲突。
//
// SQLite 不导出结构化的错误码，只能看消息文本。这不优雅，但
// 把它关在这一个函数里，比让调用方各自去 strings.Contains 好得多。
func isUniqueViolation(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}
