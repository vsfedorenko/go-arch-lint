package main

import (
	"example/basic/internal/handler"
	"example/basic/internal/repository"
	"example/basic/internal/service"
)

func main() {
	repo := repository.NewUserRepository()
	svc := service.NewUserService(repo)
	h := handler.NewUserHandler(svc)
	h.Handle()
}
