package main

import (
	"context"
	"log"
	"net"
	"net/http"

	v1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/config"
	grpcserver "github.com/VitalyMars/delivery_go/services/catalog/internal/grpc"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/handler"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/repository"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/service"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	db, err := config.NewDB(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	log.Println("database connected")

	restaurantRepository := repository.NewRestaurantRepository(db)
	restaurantService := service.NewRestaurantService(restaurantRepository)
	restaurantHandler := handler.NewRestaurantHandler(restaurantService)

	dishRepository := repository.NewDishRepository(db)
	dishService := service.NewDishService(dishRepository)
	dishHandler := handler.NewDishHandler(dishService)

	grpcServer := grpc.NewServer()

	catalogGRPCServer := grpcserver.NewServer(
		restaurantService,
		dishService,
	)

	v1.RegisterCatalogServiceServer(grpcServer, catalogGRPCServer)

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("grpc server started on :9090")

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal(err)
		}
	}()

	router := handler.NewRouter(restaurantHandler, dishHandler)

	log.Println("server started on :8080")

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
	}
}
