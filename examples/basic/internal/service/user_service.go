package service

import (
	"example/basic/internal/models"
	"example/basic/internal/repository"
)

type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(id int) (*models.User, error) {
	return s.repo.FindByID(id)
}
