package order

import "example/ddd/internal/domain/user"

type Order struct {
	ID     int
	UserID int
}

func (o Order) BelongsTo(u user.User) bool {
	return o.UserID == u.ID
}
