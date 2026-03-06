package db

import (
	"crypto/sha256"
	"fmt"
	"time"
)

func (d *DB) GenerateID(prefix, title string) (string, error) {
	for i := 0; i < 5; i++ {
		salt := ""
		if i > 0 {
			salt = fmt.Sprintf("-%d", i)
		}
		raw := fmt.Sprintf("%s%s%s", title, time.Now().Format(time.RFC3339Nano), salt)
		hash := sha256.Sum256([]byte(raw))
		hex := fmt.Sprintf("%x", hash[:])
		id := fmt.Sprintf("%s%s", prefix, hex[:6])

		var exists int
		for _, table := range []string{"tasks", "decisions", "sessions"} {
			err := d.conn.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE id = ?", id).Scan(&exists)
			if err != nil {
				return "", err
			}
			if exists > 0 {
				break
			}
		}
		if exists == 0 {
			return id, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique ID after 5 attempts")
}
