package db

import (
	"sync"
)

type User struct {
	UserId  int
	IsAdmin bool
}

var userCache = &struct {
	mu   sync.RWMutex
	data map[int]bool
}{
	mu:   sync.RWMutex{},
	data: make(map[int]bool),
}

func (db *DB) UserInsert(user User) error {
	stmt := `INSERT INTO users (user_id, is_admin) VALUES (?, ?)`
	_, err := db.Write.Exec(stmt, user.UserId, user.IsAdmin)
	if err != nil {
		return err
	}
	userCache.mu.Lock()
	if _, ok := userCache.data[user.UserId]; !ok {
		userCache.data[user.UserId] = user.IsAdmin
	}
	userCache.mu.Unlock()
	return nil
}

func (db *DB) UserUpdateAdminStat(userId int, isAdmin bool) error {
	stmt := `UPDATE users SET IsAdmin = ? WHERE user_id = ?`
	_, err := db.Write.Exec(stmt, isAdmin, userId)
	if err != nil {
		return err
	}
	userCache.mu.Lock()
	if _, ok := userCache.data[userId]; ok {
		userCache.data[userId] = isAdmin
	}
	userCache.mu.Unlock()
	return nil
}

func (db *DB) UserAuthenticate(userId int) (*User, error) {
	u := &User{
		UserId: userId,
	}

	userCache.mu.RLock()
	if isAdmin, ok := userCache.data[userId]; ok {
		u.IsAdmin = isAdmin
		userCache.mu.RUnlock()
		return u, nil
	}
	userCache.mu.RUnlock()

	stmt := `SELECT is_admin FROM users WHERE user_id = ?`
	err := db.Read.QueryRow(stmt, userId).Scan(&u.IsAdmin)
	if err != nil {
		return nil, err
	}

	userCache.mu.Lock()
	userCache.data[userId] = u.IsAdmin
	userCache.mu.Unlock()

	return u, nil
}

func (db *DB) UserDelete(userId int) error {
	stmt := `DELETE FROM users WHERE user_id = ?`
	_, err := db.Read.Exec(stmt, userId)
	if err != nil {
		return err
	}
	userCache.mu.Lock()
	delete(userCache.data, userId)
	userCache.mu.Unlock()
	return nil
}
