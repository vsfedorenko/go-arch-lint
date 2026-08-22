package main

import (
	"example/hexagonal/internal/adapter/db"
	"example/hexagonal/internal/adapter/http"
	"example/hexagonal/internal/core"
)

func main() {
	service := core.NewOrderService()
	repo := db.NewOrderRepository()
	handler := http.NewOrderHandler(service, repo)
	handler.CreateOrder()
}
