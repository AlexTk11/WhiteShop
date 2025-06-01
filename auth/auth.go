package auth

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"tg_bot/internal/bot_commands"
)

type BotAuth struct {
	admins map[int64]bool
}

var (
	instance *BotAuth
	once     sync.Once
)

// Возвращает синглтон BotAuth
func GetAuth() *BotAuth {
	once.Do(func() {
		adminIDsStr := os.Getenv("ADMIN_IDS")
		admins := make(map[int64]bool)

		for _, idStr := range strings.Split(adminIDsStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				continue
			}
			admins[id] = true
		}

		instance = &BotAuth{admins: admins}
	})

	return instance
}

// Проверяет, является ли пользователь админом
func (a *BotAuth) IsAdmin(chatID int64) bool {
	if a == nil || a.admins == nil {
		return false
	}
	return a.admins[chatID]
}

func (a *BotAuth) GetAdminStates() map[int64]int {
	if a == nil || a.admins == nil {
		return make(map[int64]int)
	}

	adminsCopy := make(map[int64]int, len(a.admins))
	for k := range a.admins {
		adminsCopy[k] = bot_commands.None
	}
	return adminsCopy
}
