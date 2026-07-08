package infrastructure

import "example/ddd/internal/domain/user"

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) FindByID(id int) (*user.User, error) {
	return &user.User{ID: id, Name: "Alice"}, nil
}
