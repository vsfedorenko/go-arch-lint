package repository

import "example/basic/internal/models"

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) FindByID(id int) (*models.User, error) {
	return &models.User{ID: id, Name: "Alice", Email: "alice@example.com"}, nil
}
