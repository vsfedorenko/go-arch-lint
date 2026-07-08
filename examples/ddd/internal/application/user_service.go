package application

import "example/ddd/internal/domain/user"

type UserRepository interface {
	FindByID(id int) (*user.User, error)
}

type UserService struct {
	repo UserRepository
}

func NewUserService(repo UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetByID(id int) (*user.User, error) {
	return s.repo.FindByID(id)
}
