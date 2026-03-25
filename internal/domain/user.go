package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role - роль пользователя в системе
type Role string

const (
	RoleUser  Role = "user"  // Обычный пользователь
	RoleAdmin Role = "admin" // Администратор
)

// User представляет пользователя системы
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         Role      `json:"role" db:"role"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
