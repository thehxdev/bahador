package db

type User struct {
	UserId    int
	FirstName string
	Username  string
	IsAdmin   bool
}

func (db *DB) UserInsert(user User) error {
	stmt := `INSERT INTO users (user_id, first_name, username, is_admin) VALUES (?, ?, ?, ?)`
	_, err := db.Write.Exec(stmt, user.UserId, user.FirstName, user.Username, user.IsAdmin)
	return err
}

func (db *DB) UserSetAdmin(userId int) error {
	stmt := `UPDATE users SET IsAdmin = 1 WHERE user_id = ?`
	_, err := db.Write.Exec(stmt, userId)
	return err
}

func (db *DB) UserAuthenticate(userId int) (*User, error) {
	u := &User{
		UserId: userId,
	}
	stmt := `SELECT is_admin FROM users WHERE user_id = ?`
	err := db.Read.QueryRow(stmt, userId).Scan(&u.IsAdmin)
	if err != nil {
		u = nil
	}
	return u, err
}
